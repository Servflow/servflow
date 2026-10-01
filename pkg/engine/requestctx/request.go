package requestctx

import (
	"bytes"
	"io"
	"net/http"
)

// ReadAndRestoreBody reads the request body and restores it so it can be read again.
// Returns the body as a string. Returns empty string if request is nil, body is nil, or on error.
func ReadAndRestoreBody(req *http.Request) string {
	if req == nil || req.Body == nil {
		return ""
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return ""
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return string(bodyBytes)
}

// SetRequest records the HTTP request that opened this request context. The
// host sets it once, before the context is shared; a request that did not
// arrive over HTTP has none.
func (rc *RequestContext) SetRequest(req *http.Request) {
	rc.Lock()
	defer rc.Unlock()
	rc.request = req
}

// Request returns the HTTP request that opened this request context, or nil
// when it did not arrive over HTTP.
func (rc *RequestContext) Request() *http.Request {
	rc.Lock()
	defer rc.Unlock()
	return rc.request
}
