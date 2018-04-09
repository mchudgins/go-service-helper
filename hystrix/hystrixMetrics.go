package hystrix

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/afex/hystrix-go/hystrix/metric_collector"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	hystrixAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_attempts_total",
			Help: "Number of attempts to cross the circuit breaker.",
		},
		[]string{"circuit"},
	)
	hystrixErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_errors_total",
			Help: "Number of errors when crossing the circuit breaker.",
		},
		[]string{"circuit"},
	)
	hystrixSuccesses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_successes_total",
			Help: "Number of successful requests.",
		},
		[]string{"circuit"},
	)
	hystrixFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_failures_total",
			Help: "Number of failed requests.",
		},
		[]string{"circuit"},
	)
	hystrixRejects = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_rejects_total",
			Help: "Number of rejected requests.",
		},
		[]string{"circuit"},
	)
	hystrixShortCircuits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_short_circuits_total",
			Help: "Number of short circuited requests (the circuit breaker was open at time of request).",
		},
		[]string{"circuit"},
	)
	hystrixTimeouts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_timeouts_total",
			Help: "Number of requests which timeout.",
		},
		[]string{"circuit"},
	)
	hystrixFallbackSuccesses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_fallback_successes_total",
			Help: "Number of successes that occurred during the execution of the fallback function.",
		},
		[]string{"circuit"},
	)
	hystrixFallbackFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_fallback_failures_total",
			Help: "Number of failures that occurred during the execution of the fallback function.",
		},
		[]string{"circuit"},
	)
	hystrixTotalDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_duration_total",
			Help: "Duration in circuit breaker.",
		},
		[]string{"circuit"},
	)
	hystrixRunDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hystrix_run_duration",
			Help: "runtime duration.",
		},
		[]string{"circuit"},
	)
)

func init() {
	prometheus.MustRegister(hystrixAttempts)
	prometheus.MustRegister(hystrixErrors)
	prometheus.MustRegister(hystrixSuccesses)
	prometheus.MustRegister(hystrixFailures)
	prometheus.MustRegister(hystrixRejects)
	prometheus.MustRegister(hystrixShortCircuits)
	prometheus.MustRegister(hystrixTimeouts)
	prometheus.MustRegister(hystrixFallbackSuccesses)
	prometheus.MustRegister(hystrixFallbackFailures)
	prometheus.MustRegister(hystrixTotalDuration)
	prometheus.MustRegister(hystrixRunDuration)
}

func (h *hystrixHelper) IncrementAttempts() {
	hystrixAttempts.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) IncrementErrors() {
	hystrixErrors.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) IncrementSuccesses() {
	hystrixSuccesses.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) IncrementFailures() {
	hystrixFailures.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) IncrementRejects() {
	hystrixRejects.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) IncrementShortCircuits() {
	hystrixShortCircuits.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) IncrementTimeouts() {
	hystrixTimeouts.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) IncrementFallbackSuccesses() {
	hystrixFallbackSuccesses.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) IncrementFallbackFailures() {
	hystrixFallbackFailures.With(prometheus.Labels{"circuit": h.commandName}).Inc()
}

func (h *hystrixHelper) UpdateTotalDuration(timeSinceStart time.Duration) {
	hystrixTotalDuration.With(prometheus.Labels{"circuit": h.commandName}).Add(float64(timeSinceStart))
}

func (h *hystrixHelper) UpdateRunDuration(runDuration time.Duration) {
	hystrixRunDuration.With(prometheus.Labels{"circuit": h.commandName}).Add(float64(runDuration))
}

func (h *hystrixHelper) Reset() {
	log.Debug("HystrixHelper reset called")
}

func (h *hystrixHelper) Update(mr metricCollector.MetricResult) {
	log.Debug("HystrixHelper.Update called")
}

func (h *hystrixHelper) NewPrometheusCollector(name string) metricCollector.MetricCollector {
	return h
}
