package httpWriter

import (
	"net/http"
)

type HTTPWriter struct {
	w             http.ResponseWriter
	statusCode    int
	contentLength int
}

func NewHTTPWriter(w http.ResponseWriter) *HTTPWriter {
	return &HTTPWriter{w: w}
}

func (l *HTTPWriter) Header() http.Header {
	return l.w.Header()
}

func (l *HTTPWriter) Write(data []byte) (int, error) {
	l.contentLength += len(data)
	return l.w.Write(data)
}

func (l *HTTPWriter) WriteHeader(status int) {
	l.statusCode = status
	l.w.WriteHeader(status)
}

func (l *HTTPWriter) Length() int {
	return l.contentLength
}

func (l *HTTPWriter) StatusCode() int {

	// if nobody set the status, but data has been written
	// then all must be well.
	if l.statusCode == 0 && l.contentLength > 0 {
		return http.StatusOK
	}

	return l.statusCode
}
