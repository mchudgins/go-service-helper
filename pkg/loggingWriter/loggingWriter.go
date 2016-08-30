package loggingWriter

import (
	"log"
	"net/http"
	"time"
)

type loggingWriter struct {
	w             http.ResponseWriter
	statusCode    int
	contentLength int
}

func NewLoggingWriter(w http.ResponseWriter) *loggingWriter {
	return &loggingWriter{w: w}
}

func (l *loggingWriter) Header() http.Header {
	return l.w.Header()
}

func (l *loggingWriter) Write(data []byte) (int, error) {
	l.contentLength += len(data)
	return l.w.Write(data)
}

func (l *loggingWriter) WriteHeader(status int) {
	l.statusCode = status
	l.w.WriteHeader(status)
}

func (l *loggingWriter) Length() int {
	return l.contentLength
}

func (l *loggingWriter) StatusCode() int {

	// if nobody set the status, but data has been written
	// then all must be well.
	if l.statusCode == 0 && l.contentLength > 0 {
		return http.StatusOK
	}

	return l.statusCode
}

// httpLogger provides per request log statements (ala Apache httpd)
func HttpLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := NewLoggingWriter(w)
		defer func() {
			end := time.Now()
			duration := end.Sub(start)
			log.Printf("host: %s; uri: %s; remoteAddress: %s; method:  %s; proto: %s; status: %d, contentLength: %d, duration: %.3f; ua: %s",
				r.Host,
				r.RequestURI,
				r.RemoteAddr,
				r.Method,
				r.Proto,
				lw.StatusCode(),
				lw.Length(),
				duration.Seconds()*1000,
				r.UserAgent())
		}()

		h.ServeHTTP(lw, r)

	})
}
