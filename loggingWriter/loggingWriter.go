package loggingWriter

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
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
func HttpApacheLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := NewLoggingWriter(w)
		defer func() {
			end := time.Now()
			duration := end.Sub(start)
			log.Printf("host: %s; uri: %s; CorrelationID: %s; remoteAddress: %s; method:  %s; proto: %s; status: %d, contentLength: %d, duration: %.3f; ua: %s",
				r.Host,
				r.RequestURI,
				lw.Header().Get("X-Correlation-ID"),
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

func getRequestURIFromRaw(rawURI string) string {
	if ! strings.Contains(rawURI, "?") {
		return rawURI
	}

	i := strings.Index(rawURI, "?")

	return rawURI[:i]
}

func HTTPLogrusLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := NewLoggingWriter(w)

		// save some values, in case the handler changes 'em
		host := r.Host
		url := getRequestURIFromRaw(r.RequestURI)
		remoteAddr := r.RemoteAddr
		method := r.Method
		proto := r.Proto

		defer func() {
			end := time.Now()
			duration := end.Sub(start)

			fields := logrus.Fields{}
			for key := range r.Header {
				fields[key] = r.Header.Get(key)
			}
			fields["Host"] = host
			fields["URL"] = url
			fields["remoteIP"] = remoteAddr
			fields["method"] = method
			fields["proto"] = proto
			fields["status"] = lw.StatusCode()
			fields["length"] = lw.Length()
			fields["duration"] = duration.Seconds() * 1000

			logrus.WithFields(fields).Info("")
		}()

		h.ServeHTTP(lw, r)

	})
}
