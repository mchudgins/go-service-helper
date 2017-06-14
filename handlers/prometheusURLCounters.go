package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/mchudgins/go-service-helper/httpWriter"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "httpRequestsReceived_total",
			Help: "Number of HTTP requests received.",
		},
		[]string{"url"},
	)
	httpRequestsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "httpRequestsProcessed_total",
			Help: "Number of HTTP requests processed.",
		},
		[]string{"url", "status"},
	)
	httpRequestDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_response_duration",
			Help: "Duration of HTTP responses.",
		},
		[]string{"url", "status"},
	)
	httpResponseSize = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_response_size",
			Help: "Size of http responses",
		},
		[]string{"url"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsReceived)
	prometheus.MustRegister(httpRequestsProcessed)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(httpResponseSize)
}

func HTTPMetricsCollector(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		u := r.URL.Path
		httpRequestsReceived.With(prometheus.Labels{"url": u}).Inc()

		// we want the status code from the handler chain,
		// so inject an HTTPWriter, if one doesn't exist

		if _, ok := w.(*httpWriter.HTTPWriter); !ok {
			w = httpWriter.NewHTTPWriter(w)
		}

		// after ServeHTTP runs, collect metrics!

		defer func() {
			if hw, ok := w.(*httpWriter.HTTPWriter); ok {
				status := strconv.Itoa(hw.StatusCode())
				httpRequestsProcessed.With(prometheus.Labels{"url": u, "status": status}).Inc()
				end := time.Now()
				duration := end.Sub(start)
				httpRequestDuration.With(prometheus.Labels{"url": u, "status": status}).Observe(float64(duration.Nanoseconds()))
				httpResponseSize.With(prometheus.Labels{"url": u}).Observe(float64(hw.Length()))
			}
		}()

		fn.ServeHTTP(w, r)
	})
}
