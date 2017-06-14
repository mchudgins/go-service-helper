package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mchudgins/go-service-helper/httpWriter"
)

// httpLogger provides per request log statements (ala Apache httpd)
func HttpApacheLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := httpWriter.NewHTTPWriter(w)
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
	if !strings.Contains(rawURI, "?") {
		return rawURI
	}

	i := strings.Index(rawURI, "?")

	return rawURI[:i]
}

func HTTPLogrusLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := httpWriter.NewHTTPWriter(w)

		// save some values, in case the handler changes 'em
		host := r.Host
		url := getRequestURIFromRaw(r.RequestURI)
		remoteAddr := r.RemoteAddr
		method := r.Method
		proto := r.Proto

		defer func() {
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
			fields["X-Correlation-ID"] = lw.Header().Get("X-Correlation-ID")

			end := time.Now()
			duration := end.Sub(start)

			fields["duration"] = duration.Seconds() * 1000
			fields["time"] = start.Format("20060102030405.000000")

			logrus.WithFields(fields).Info("")
		}()

		h.ServeHTTP(lw, r)

	})
}
