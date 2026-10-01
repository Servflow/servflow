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
