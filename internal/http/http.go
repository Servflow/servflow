package http

import (
	"net/http"

	"github.com/Servflow/servflow/pkg/engine/requestctx"
)

type SfResponse struct {
	Body    []byte
	Code    int
	Headers http.Header
	File    *requestctx.FileValue
}

func (s *SfResponse) SetHeader(key, value string) {
	if s.Headers == nil {
		s.Headers = make(http.Header)
	}
	s.Headers.Set(key, value)
}

// Kind implements responses.Result. SfResponse is the HTTP result type.
func (s *SfResponse) Kind() string {
	return "http"
}
