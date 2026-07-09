package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	sfhttp "github.com/Servflow/servflow/internal/http"
	apiconfig "github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"github.com/Servflow/servflow/pkg/tracing"
	"github.com/gorilla/mux"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TODO optimize this

// NewAPIHandlerForConfig takes an apiconfig and a logger and returns an APIHandler with the appropriate
// actions and datasource managers
func (e *Engine) createBasicHandler(config *apiconfig.APIConfig) (http.Handler, error) {
	logger := logging.FromContext(e.ctx)
	logger.Debug("Loading API configuration", zap.String("api", config.ID), zap.String("path", config.HttpConfig.ListenPath), zap.String("method", config.HttpConfig.Method))

	ws, err := e.resolveWorkspace(config)
	if err != nil {
		return nil, err
	}

	planner := plan.NewPlannerV2(plan.PlannerConfig{
		Actions:      config.Actions,
		Conditions:   config.Conditionals,
		Responses:    config.Responses,
		Integrations: config.Integrations,
		Workspace:    ws,
	}, logger)
	p, err := planner.Plan()
	if err != nil {
		return nil, err
	}

	logger.Debug("Starting plan generation from", zap.String("start", config.HttpConfig.Next))

	a := &APIHandler{
		apiPath:   config.HttpConfig.ListenPath,
		apiName:   config.ID,
		planStart: config.HttpConfig.Next,
		p:         p,
	}

	return a.CreateChain(config, e.getCorsConfig()), nil
}

type APIHandler struct {
	apiPath   string
	apiName   string
	p         *plan.Plan
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
			if len(key) == 0 {
				quoted := strconv.Quote(string(b))
				return quoted[1 : len(quoted)-1]
			}
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

// initTracing initializes tracing for the request and returns the updated context and span
func (h *APIHandler) initTracing(req *http.Request) (context.Context, trace.Span) {
	if !tracing.OTELEnabled() {
		return req.Context(), nil
	}

	ctx, span := tracing.StartHTTPEntry(req.Context())

	span.SetAttributes(
		attribute.String("sf.http.method", req.Method),
		attribute.String("sf.http.path", req.URL.Path),
	)

	// Add query parameters to trace
	queryParams := req.URL.Query()
	for key, values := range queryParams {
		span.SetAttributes(attribute.StringSlice("sf.http.query."+key, values))
	}

	// Add form values to trace
	if err := req.ParseForm(); err == nil {
		for key, values := range req.Form {
			span.SetAttributes(attribute.StringSlice("sf.http.form."+key, values))
		}
	}

	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err == nil {
			span.SetAttributes(attribute.String("sf.body", string(bodyBytes)))
			req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		}
	}

	return ctx, span
}

// ServeHttp extracts the context parameters and begins excuting the plan (step)
func (h *APIHandler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	start := time.Now()
	ctx := req.Context()
	logger := logging.FromContext(ctx)
	logger.Debug("Handling request")

	ctx, span := h.initTracing(req)
	if span != nil {
		defer span.End()
		// Registered after span.End so it runs first (LIFO): attach the
		// request-level token total before the root span closes.
		defer tracing.SetRequestTokens(ctx, span)
	}

	rectx, ok := requestctx.FromContext(ctx)
	if !ok {
		logger.Error("Could not get request context")
		tracing.SetHTTPStatus(span, http.StatusInternalServerError, errors.New("could not get request context"))
		http.Error(wr, "Error processing request", http.StatusInternalServerError)
		return
	}

	if span != nil {
		span.SetAttributes(attribute.String("sf.request_id", rectx.ID()))
	}
	rectx.AddRequestTemplateFunctions(requestTemplateFunctions(req), false)

	err := rectx.LoadRequestFiles(req)
	if err != nil {
		logger.Error("Error storing HTTP request", zap.Error(err))
		tracing.SetHTTPStatus(span, http.StatusInternalServerError, err)
		http.Error(wr, "Error processing request", http.StatusInternalServerError)
		return
	}

	ctx = plan.WithRequest(ctx, req)

	result, err := h.p.Execute(ctx, h.planStart)
	resp, ok := result.(*sfhttp.SfResponse)
	if err != nil || !ok || resp == nil {
		tracing.SetHTTPStatus(span, http.StatusInternalServerError, err)
		switch {
		case err != nil:
			h.logAndWriteInternalServerError(wr, err, logger)
		case result != nil && !ok:
			// A non-nil result that isn't an HTTP response means a non-http
			// response kind was mounted on an HTTP endpoint. Surface the type
			// rather than the misleading "response missing".
			h.logAndWriteInternalServerError(wr, fmt.Errorf("unexpected result type %T for HTTP endpoint", result), logger)
		default:
			h.logAndWriteInternalServerError(wr, errors.New("error executing api, response missing"), logger)
		}
		return
	}

	tracing.SetHTTPStatus(span, resp.Code, nil)
	for key := range resp.Headers {
		wr.Header().Set(key, resp.Headers.Get(key))
	}
	wr.WriteHeader(resp.Code)
	wr.Write(resp.Body)
	timeTaken := time.Since(start)
	logger.Debug("Finished handling request", zap.String("timeTaken", timeTaken.String()))
}

func (h *APIHandler) logAndWriteInternalServerError(w http.ResponseWriter, err error, logger *zap.Logger) {
	logger.Error("error handling request", zap.Error(err))
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("error completing request, please reach out to admin"))
}
