package handlers

import (
	"context"
	"log"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/mchudgins/go-service-helper/correlationID"
	"github.com/mchudgins/go-service-helper/httpWriter"
	"github.com/mchudgins/go-service-helper/user"
)

type key int

var loggerKey key = 0

func FromContext(ctx context.Context) (*logrus.Entry, bool) {
	logger, ok := ctx.Value(loggerKey).(*logrus.Entry)
	return logger, ok
}

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
				lw.Header().Get(correlationID.CORRID),
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

		ctx := r.Context()
		l := logrus.New().WithField(correlationID.CORRID, correlationID.FromContext(ctx))
		r = r.WithContext(context.WithValue(ctx, loggerKey, l))

		lw := httpWriter.NewHTTPWriter(w)

		// save some values, in case the handler changes 'em
		host := r.Host
		url := getRequestURIFromRaw(r.RequestURI)
		remoteAddr := r.RemoteAddr
		method := r.Method
		proto := r.Proto

		fields := logrus.Fields{}
		fields["Host"] = host
		for key := range r.Header {
			fields[textproto.CanonicalMIMEHeaderKey(key)] = r.Header.Get(key)
		}
		fields["URL"] = url
		fields["remoteIP"] = remoteAddr
		fields["method"] = method
		fields["proto"] = proto

		defer func() {
			fields["status"] = lw.StatusCode()
			fields["length"] = lw.Length()

			// maybe the X-Request-ID was set on the way back?
			id, ok := fields[correlationID.CORRID].(string)
			if !ok || len(id) == 0 {
				fields[correlationID.CORRID] = lw.Header().Get(correlationID.CORRID)
			}

			// get some info about the response
			rct := lw.Header().Get("Content-Type")
			if len(rct) > 0 {
				fields["response-Content-Type"] = rct
			}
			cc := lw.Header().Get("Cache-Control")
			if len(cc) > 0 {
				fields["response-Cache-Control"] = cc
			}

			end := time.Now()
			duration := end.Sub(start)

			fields["duration"] = duration.Seconds() * 1000
			fields["time"] = start.Format("20060102030405.000000")

			// who dat?
			_, uid := user.FromRequest(r)
			if len(uid) == 0 {
				uid = user.FromContext(r.Context())
				if len(uid) > 0 {
					fields["userID"] = uid
				}
			}

			logrus.WithFields(fields).Info("")
		}()

		h.ServeHTTP(lw, r)

	})
}
