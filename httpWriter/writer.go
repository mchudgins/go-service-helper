package httpWriter

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

type HTTPWriter struct {
	w             http.ResponseWriter
	statusCode    int
	contentLength int
	logger        *log.Entry
}

type Option func(w *HTTPWriter)

func Logger(logger *log.Entry) Option {
	return func(w *HTTPWriter) { w.logger = logger }
}

func NewHTTPWriter(w http.ResponseWriter, options ...Option) *HTTPWriter {
	writer := &HTTPWriter{w: w}

	for _, option := range options {
		option(writer)
	}

	return writer
}

func (l *HTTPWriter) Header() http.Header {
	return l.w.Header()
}

func (l *HTTPWriter) Write(data []byte) (int, error) {

	if l.logger != nil {
		l.logger.
			WithField("data", string(data)).
			WithField("len", len(data)).
			Info("HTTPWriter.Write")
	}

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
