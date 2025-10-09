package middleware

import (
	"errors"
	"net/http"
)

var ErrMiddlewareFailed = errors.New("middleware failed")

type Middleware interface {
	Handle(wr http.ResponseWriter, req *http.Request) error
	Name() string
}
