package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Servflow/servflow/pkg/engine/actions"
	"github.com/Servflow/servflow/pkg/engine/plan"
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

func (h *Http) Config() string {
	configBytes, err := json.Marshal(h.cfg)
	if err != nil {
		return ""
	}
	return string(configBytes)
}

func (h *Http) Execute(ctx context.Context, filledInConfig string) (interface{}, error) {
	logger := logging.FromContext(ctx).With(zap.String("execution_type", h.Type()))
	ctx = logging.WithLogger(ctx, logger)

	var cfg Config

	if err := json.Unmarshal([]byte(filledInConfig), &cfg); err != nil {
		return nil, err
	}

	var body io.Reader
	if cfg.Body != nil {
		body = bytes.NewBuffer(cfg.Body)
	}

	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, body)
	if err != nil {
		return nil, err
	}

	if len(cfg.Headers) > 0 {
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if cfg.ExpectedResponseCode != "" && cfg.ExpectedResponseCode != "0" {
		expectedCode := cfg.ExpectedResponseCode
		if fmt.Sprintf("%d", resp.StatusCode) != expectedCode {
			return nil, fmt.Errorf("%w: unexpected response code %d, expected %s", plan.ErrFailure, resp.StatusCode, expectedCode)
		}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	logger.Debug("finished request", zap.String("url", req.URL.String()), zap.Int("status", resp.StatusCode), zap.ByteString("body", bodyBytes))

	if len(bodyBytes) == 0 && cfg.FailIfResponseEmpty {
		return nil, fmt.Errorf("%w: response body is empty", plan.ErrFailure)
	}

	if cfg.ResponsePath == "" {
		var result interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	if !gjson.ValidBytes(bodyBytes) {
		return nil, fmt.Errorf("%w: invalid JSON response", plan.ErrFailure)
	}

	value := gjson.GetBytes(bodyBytes, cfg.ResponsePath)
	if !value.Exists() {
		return nil, fmt.Errorf("%w: path '%s' not found in response", plan.ErrFailure, cfg.ResponsePath)
	}

	return value.Value(), nil
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
			Placeholder: "GET, POST, PUT, DELETE",
			Required:    true,
			Default:     "GET",
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
		Constructor: func(config json.RawMessage) (actions.ActionExecutable, error) {
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
