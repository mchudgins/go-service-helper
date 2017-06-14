package handlers

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/google/uuid"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	//zlog "github.com/opentracing/opentracing-go/log"
	"github.com/mchudgins/go-service-helper/httpWriter"
)

const (
	CORRID string = "X-Correlation-ID"
)

var (
	debugMode          = true
	serviceHostPort    = "localhost:8080"
	zipkinHTTPEndpoint = "http://localhost:9411/api/v1/spans"
)

type traceLogger struct{}

func (logger traceLogger) Log(keyval ...interface{}) error {
	log.Warn("Log was called!")
	return nil
}

func NewTracer(serviceName string) opentracing.Tracer {
	collector, err := zipkin.NewHTTPCollector(zipkinHTTPEndpoint,
		zipkin.HTTPLogger(traceLogger{}),
		zipkin.HTTPBatchSize(1))
	if err != nil {
		log.WithError(err).Fatal("zipkin.NewHTTPCollector failed")
	}

	tracer, err := zipkin.NewTracer(
		zipkin.NewRecorder(collector, debugMode, serviceHostPort, serviceName),
		zipkin.WithLogger(traceLogger{}),
		//		zipkin.DebugMode(true),
		//		zipkin.ClientServerSameSpan(true),
	)

	if err != nil {
		log.WithError(err).Warn("unable to construct zipkin.Tracer")
	}

	opentracing.InitGlobalTracer(tracer)

	return tracer
}

// HandlerFunc is a middleware function for incoming HTTP requests.
type HandlerFunc func(next http.Handler) http.Handler

// FromHTTPRequest returns a Middleware HandlerFunc that tries to join with an
// OpenTracing trace found in the HTTP request headers and starts a new Span
// called `operationName`. If no trace could be found in the HTTP request
// headers, the Span will be a trace root. The Span is incorporated in the
// HTTP Context object and can be retrieved with
// opentracing.SpanFromContext(ctx).
func TracerFromHTTPRequest(tracer opentracing.Tracer, operationName string,
) HandlerFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			span, ctx := opentracing.StartSpanFromContext(req.Context(), operationName)
			defer span.Finish()

			// tag this request with a correlation ID, so we can troubleshoot it later, if necessary
			corrID := req.Header.Get(CORRID)
			if len(corrID) == 0 {
				corrID = uuid.New().String()
			}

			w.Header().Add(CORRID, corrID)
			span.SetTag(CORRID, corrID)
			ext.HTTPUrl.Set(span, req.URL.Path)
			//			span.SetTag(ext.HTTPUrl,req.URL.Path)

			// store span in context
			ctx = opentracing.ContextWithSpan(req.Context(), span)

			// update request context to include our new span
			req = req.WithContext(ctx)

			// we want the status code from the handler chain,
			// so inject an HTTPWriter, if one doesn't exist

			if _, ok := w.(*httpWriter.HTTPWriter); !ok {
				w = httpWriter.NewHTTPWriter(w)
			}

			// next middleware or actual request handler
			next.ServeHTTP(w, req)

			if hw, ok := w.(*httpWriter.HTTPWriter); ok {
				span.SetTag(string(ext.HTTPStatusCode), hw.StatusCode())
			}
		})
	}
}

func TracerFromInternalHTTPRequest(tracer opentracing.Tracer, operationName string,
) HandlerFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			var serverSpan opentracing.Span
			appSpecificOperationName := operationName
			wireContext, err := opentracing.GlobalTracer().Extract(
				opentracing.HTTPHeaders,
				opentracing.HTTPHeadersCarrier(req.Header))
			if err != nil {
				log.WithError(err).Error("unable to extract wire context")
			}

			// Create the span referring to the RPC client if available.
			// If wireContext == nil, a root span will be created.
			serverSpan = opentracing.StartSpan(
				appSpecificOperationName,
				ext.RPCServerOption(wireContext))

			defer serverSpan.Finish()

			ext.HTTPUrl.Set(serverSpan, req.URL.Path)

			/*
				serverSpan.LogFields(
					zlog.String(string(ext.HTTPUrl), req.URL.Path),
				)
			*/

			ctx := opentracing.ContextWithSpan(req.Context(), serverSpan)

			// update request context to include our new span
			req = req.WithContext(ctx)

			// we want the status code from the handler chain,
			// so inject an HTTPWriter, if one doesn't exist

			if _, ok := w.(*httpWriter.HTTPWriter); !ok {
				w = httpWriter.NewHTTPWriter(w)
			}
			defer func() {
				if hw, ok := w.(*httpWriter.HTTPWriter); ok {
					serverSpan.SetTag(string(ext.HTTPStatusCode), hw.StatusCode())
				}
			}()

			// next middleware or actual request handler
			next.ServeHTTP(w, req)
		})
	}
}
