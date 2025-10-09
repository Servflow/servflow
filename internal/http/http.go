package http

import "net/http"

type SfResponse struct {
	Body    []byte
	Code    int
	Headers http.Header
}

func (s *SfResponse) SetHeader(key, value string) {
	if s.Headers == nil {
		s.Headers = make(http.Header)
	}
	s.Headers.Set(key, value)
}
