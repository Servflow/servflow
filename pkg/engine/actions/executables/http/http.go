package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Servflow/servflow/pkg/engine/actions"
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
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	Body         json.RawMessage   `json:"body"`
	ResponsePath string            `json:"response_path"`
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

// TODO sanitize response if string or query
func (h *Http) Execute(ctx context.Context, filledInConfig string) (interface{}, error) {
	logger := logging.WithContext(ctx).With(zap.String("action", h.Type()))
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	logger.Debug("finished request", zap.String("url", req.URL.String()), zap.Int("status", resp.StatusCode), zap.ByteString("body", bodyBytes))

	if cfg.ResponsePath == "" {
		var result interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	if !gjson.ValidBytes(bodyBytes) {
		return nil, fmt.Errorf("invalid JSON response: %s", string(bodyBytes))
	}

	value := gjson.GetBytes(bodyBytes, cfg.ResponsePath)
	if !value.Exists() {
		return nil, fmt.Errorf("path '%s' not found in response: %s", cfg.ResponsePath, string(bodyBytes))
	}

	return value.Value(), nil
}

func init() {
	fields := map[string]actions.FieldInfo{
		"url": {
			Type:        "string",
			Label:       "URL",
			Placeholder: "https://api.example.com/endpoint",
			Required:    true,
		},
		"method": {
			Type:        "string",
			Label:       "HTTP Method",
			Placeholder: "GET, POST, PUT, DELETE",
			Required:    true,
			Default:     "GET",
		},
		"headers": {
			Type:        "object",
			Label:       "Headers",
			Placeholder: "HTTP headers as key-value pairs",
			Required:    false,
		},
		"body": {
			Type:        "object",
			Label:       "Request Body",
			Placeholder: "Request body data",
			Required:    false,
		},
		"response_path": {
			Type:        "string",
			Label:       "Response Path",
			Placeholder: "JSONPath to extract from response (optional)",
			Required:    false,
		},
	}

	if err := actions.RegisterAction("http", actions.ActionRegistration{
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
