package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Servflow/servflow/internal/logging"
	"github.com/Servflow/servflow/internal/tracing"
	"github.com/Servflow/servflow/pkg/definitions"
	plan2 "github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/gorilla/mux"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	"go.opentelemetry.io/otel/trace"
)

// TODO optimize this

// NewAPIHandlerForConfig takes an apiconfig and a logger and returns an APIHandler with the appropriate
// actions and datasource managers
func (e *Engine) createBasicHandler(config *apiconfig.APIConfig) (http.Handler, error) {
	logger := logging.GetLogger()
	logger.Info("Loading API configuration", zap.String("api", config.ID), zap.String("path", config.HttpConfig.ListenPath), zap.String("method", config.HttpConfig.Method))

	planner := plan2.NewPlannerV2(plan2.PlannerConfig{
		Actions:    config.Actions,
		Conditions: config.Conditionals,
		Responses:  config.Responses,
	})
	p, err := planner.Plan()
	if err != nil {
		return nil, err
	}

	logging.GetLogger().Debug("Starting plan generation from", zap.String("start", config.HttpConfig.Next))

	a := &APIHandler{
		apiPath:   config.HttpConfig.ListenPath,
		apiName:   config.ID,
		planStart: config.HttpConfig.Next,
		p:         p,
	}

	return a.CreateChain(config), nil
}

type APIHandler struct {
	apiPath   string
	apiName   string
	p         *plan2.Plan
	planStart string
}

const mcpServerVersion = "0.1.0"

func requestTemplateFunctions(req *http.Request) template.FuncMap {
	var (
		once sync.Once
		b    []byte
	)
	return map[string]any{
		"header": req.Header.Get,
		"param":  req.FormValue,
		"body": func(key string) string {
			once.Do(func() {
				if req.Header.Get("Content-Type") != "application/json" || req.Body == nil {
					return
				}
				body, err := io.ReadAll(io.LimitReader(req.Body, 1048576))
				if err != nil {
					return
				}
				b = body
			})
			if len(b) > 0 {
				v := gjson.GetBytes(b, key).String()
				v = strconv.Quote(v)
				return strings.Trim(v, `"`)
			}
			return ""
		},
		"urlparam": func(key string) string {
			vars := mux.Vars(req)
			r := vars[key]
			return r
		},
	}
}

// ServeHttp extracts the context parameters and begins excuting the plan (step)
func (h *APIHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()

	logger := logging.GetLogger().With(zap.String("path", req.URL.Path), zap.String("method", req.Method))
	logger.Debug("Handling request")
	if tracing.OTELEnabled() {
		var span trace.Span
		ctx, span = tracing.SpanCtxFromContext(req.Context(), h.apiPath+" Handler")
		defer span.End()
	}

	rectx, ok := requestctx.FromContext(ctx)
	if !ok {
		logger.Error("Could not get request context")
		http.Error(wr, "Error processing request", http.StatusInternalServerError)
		return
	}
	rectx.AddRequestTemplateFunctions(requestTemplateFunctions(req))

	resp, err := h.p.Execute(ctx, h.planStart, "")
	if err != nil || resp == nil {
		if err != nil {
			h.logAndWriteInternalServerError(wr, err)
		} else {
			h.logAndWriteInternalServerError(wr, errors.New("error executing api, response missing"))
		}
		return
	}

	for key := range resp.Headers {
		wr.Header().Set(key, resp.Headers.Get(key))
	}
	wr.WriteHeader(resp.Code)
	wr.Write(resp.Body)
	timeTaken := time.Since(start)
	logger.Debug("Finished handling request", zap.String("timeTaken", timeTaken.String()))
}

func (h *APIHandler) logAndWriteInternalServerError(w http.ResponseWriter, err error) {
	logging.Error(context.Background(), "error handling request", err)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("error completing request, please reach out to admin"))
}
