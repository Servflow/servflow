package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/logging"
	"go.uber.org/zap"

	"github.com/tidwall/gjson"
)

type Http struct {
	client *http.Client
	cfg    *Config
}

func (h *Http) Type() string {
	return "http"
}

func (h *Http) SupportsReplica() bool {
	return true
}

// unwrapBody returns the body template to resolve. The raw-string body form
// stores the body as a JSON string wrapping the real text, so it is unwrapped
// once; every other shape (object/array, or a non-JSON body) is used as-is. The
// author owns escaping of any dynamic value they place inside JSON — a single
// {{ escape }} is correct because the body is resolved on its own, with no outer
// config-serialization layer.
func unwrapBody(raw json.RawMessage) string {
	var unwrapped string
	if err := json.Unmarshal(raw, &unwrapped); err == nil {
		return unwrapped
	}
	return string(raw)
}

type Config struct {
	URL                  string            `json:"url" yaml:"url"`
	Method               string            `json:"method" yaml:"method"`
	Headers              map[string]string `json:"headers" yaml:"headers"`
	Body                 json.RawMessage   `json:"body" yaml:"body"`
	ResponsePath         string            `json:"responsePath" yaml:"responsePath"`
	ExpectedResponseCode string            `json:"expectedResponseCode" yaml:"expectedResponseCode"`
	FailIfResponseEmpty  bool              `json:"failIfResponseEmpty" yaml:"failIfResponseEmpty"`
}

func New(cfg Config) *Http {
	return &Http{
		client: &http.Client{},
		cfg:    &cfg,
	}
}

func (h *Http) Execute(ctx context.Context) (interface{}, map[string]string, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", h.Type()))
	ctx = logging.WithLogger(ctx, logger)

	rc, err := requestctx.FromContextOrError(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get request context: %w", err)
	}

	// V2: every templated field — the scalars, each header key/value, and the
	// body — resolves against the request context in a SINGLE batched pass.
	// ResolveBatch returns results in the same order as the inputs, so we just
	// append in a known order and read back in that same order.
	cfg := *h.cfg

	var batch []string
	batch = append(batch, cfg.URL, cfg.Method, cfg.ResponsePath, cfg.ExpectedResponseCode)

	for k, v := range cfg.Headers {
		batch = append(batch, k, v)
	}

	hasBody := len(cfg.Body) > 0
	if hasBody {
		batch = append(batch, unwrapBody(cfg.Body))
	}

	resolved, err := rc.ResolveBatch(ctx, batch...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve http config: %w", err)
	}

	i := 0
	next := func() string { v := resolved[i]; i++; return v }

	cfg.URL = next()
	cfg.Method = next()
	cfg.ResponsePath = next()
	cfg.ExpectedResponseCode = next()

	if len(cfg.Headers) > 0 {
		headers := make(map[string]string, len(cfg.Headers))
		for range cfg.Headers {
			key := next()
			headers[key] = next()
		}
		cfg.Headers = headers
	}

	var body io.Reader
	if hasBody {
		body = bytes.NewBufferString(next())
	}

	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, body)
	if err != nil {
		return nil, nil, err
	}

	if len(cfg.Headers) > 0 {
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	fields := map[string]string{}
	fields["status_code"] = strconv.Itoa(resp.StatusCode)
	fields["response_body"] = string(bodyBytes)

	// Scrub the URL explicitly (a secret can ride in a query param); the
	// context logger's scrub core also covers the body if it echoes a secret.
	logger.Debug("finished request", zap.String("url", rc.Scrub(cfg.URL)), zap.Int("status", resp.StatusCode), zap.ByteString("body", bodyBytes))

	if cfg.ExpectedResponseCode != "" && cfg.ExpectedResponseCode != "0" {
		expectedCode := cfg.ExpectedResponseCode
		if fmt.Sprintf("%d", resp.StatusCode) != expectedCode {
			return nil, fields, fmt.Errorf("%w: unexpected response code %d, expected %s", plan.ErrFailure, resp.StatusCode, expectedCode)
		}
	}

	if len(bodyBytes) == 0 && cfg.FailIfResponseEmpty {
		return nil, fields, fmt.Errorf("%w: response body is empty", plan.ErrFailure)
	}

	if cfg.ResponsePath == "" {
		var result interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return string(bodyBytes), fields, nil
		}
		return result, nil, nil
	}

	if !gjson.ValidBytes(bodyBytes) {
		return nil, nil, fmt.Errorf("%w: invalid JSON response", plan.ErrFailure)
	}

	value := gjson.GetBytes(bodyBytes, cfg.ResponsePath)
	if !value.Exists() {
		return nil, nil, fmt.Errorf("%w: path '%s' not found in response", plan.ErrFailure, cfg.ResponsePath)
	}

	return value.Value(), fields, nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"url": {
			Type:        actions.FieldTypeString,
			Label:       "URL",
			Placeholder: "https://api.example.com/endpoint",
			Required:    true,
		},
		"method": {
			Type:        actions.FieldTypeString,
			Label:       "HTTP Method",
			Placeholder: "GET, POST, PUT, DELETE, PATCH",
			Required:    true,
			Default:     "GET",
			Values:      []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		},
		"headers": {
			Type:        actions.FieldTypeMap,
			Label:       "Headers",
			Placeholder: "HTTP headers as key-value pairs",
			Required:    false,
		},
		"body": {
			Type:        actions.FieldTypeMap,
			Label:       "Request Body",
			Placeholder: "Request body data",
			Required:    false,
			Metadata: map[string]string{
				"type": "httpbody",
			},
		},
		"responsePath": {
			Type:        actions.FieldTypeString,
			Label:       "Response Path",
			Placeholder: "JSONPath to extract from response (optional)",
			Required:    false,
		},
		"expectedResponseCode": {
			Type:        actions.FieldTypeString,
			Label:       "Expected Response Code",
			Placeholder: "200",
			Required:    false,
			Default:     "",
		},
		"failIfResponseEmpty": {
			Type:        actions.FieldTypeBoolean,
			Label:       "Fail if Response Empty",
			Placeholder: "Treat empty response as failure",
			Required:    false,
			Default:     true,
		},
	}

	if err := actions.RegisterAction("http", actions.ActionRegistrationInfo{
		Name:        "HTTP Request",
		Description: "Makes HTTP requests to external APIs and returns the response",
		Fields:      fields,
		UseV2:       true,
		ConstructorV2: func(config json.RawMessage) (actions.ActionExecutableV2, error) {
			var cfg Config
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("error creating http action: %v", err)
			}
			return New(cfg), nil
		},
	}); err != nil {
		panic(err)
	}
}
