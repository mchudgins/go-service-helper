package handlers

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/mchudgins/go-service-helper/correlationID"
	"github.com/mchudgins/go-service-helper/httpWriter"
	"github.com/mchudgins/go-service-helper/hystrix"
	"github.com/mchudgins/go-service-helper/zipkin"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	openzipkin "github.com/openzipkin-contrib/zipkin-go-opentracing"
	//zlog "github.com/opentracing/opentracing-go/log"
)

var (
	debugMode          = true
	serviceHostPort    = "localhost:8080"
	zipkinHTTPEndpoint = "http://localhost:9411/api/v1/spans"
)

type traceLogger struct{}

func (logger traceLogger) Log(keyval ...interface{}) error {
	fields := make(log.Fields)
	len := len(keyval)

	for i := 0; i < len; i = +2 {
		if key, ok := keyval[i].(string); ok {
			fields[key] = keyval[i+1]
		} else {
			log.WithField("field", keyval[i]).
				Error("key name is not of type 'string'")
		}
	}

	log.WithFields(fields).Info("opentracer.go")

	return nil
}

func NewTracer(serviceName string) opentracing.Tracer {
	collector, err := zipkin.NewHTTPCollector(zipkinHTTPEndpoint,
		zipkin.HTTPLogger(traceLogger{}),
		zipkin.HTTPClient(hystrix.NewClient("zipkin")),
		zipkin.HTTPBatchSize(100))
	if err != nil {
		log.WithError(err).Fatal("zipkin.NewHTTPCollector failed")
	}

	tracer, err := openzipkin.NewTracer(
		openzipkin.NewRecorder(collector, debugMode, serviceHostPort, serviceName),
		openzipkin.WithLogger(traceLogger{}),
		//		zipkin.DebugMode(true),
		//		zipkin.ClientServerSameSpan(true),
	)

	if err != nil {
		log.WithError(err).Warn("unable to construct zipkin.Tracer")
	}

	opentracing.SetGlobalTracer(tracer)

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
/*
func TracerFromHTTPRequest(tracer opentracing.Tracer, operationName string,
) HandlerFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			span, ctx := opentracing.StartSpanFromContext(req.Context(), operationName)
			defer span.Finish()

			// tag this request with a correlation ID, so we can troubleshoot it later, if necessary
			req, corrID := correlationID.FromRequest(req)
			w.Header().Set(correlationID.CORRID, corrID)
			span.SetTag(correlationID.CORRID, corrID)
			ext.HTTPUrl.Set(span, req.URL.Path)

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
*/

func TracerFromHTTPRequest(tracer opentracing.Tracer, operationName string,
) HandlerFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			// tag this request with a correlation ID, so we can troubleshoot it later, if necessary
			req, corrID, fExisted := correlationID.FromRequest(req)

			// if we're at the edge of the system, send the correlation ID back in the response
			if !fExisted {
				w.Header().Set(correlationID.CORRID, corrID)
			}

			var serverSpan opentracing.Span
			//			appSpecificOperationName := operationName
			appSpecificOperationName := req.Method + ":" + req.URL.Path
			wireContext, err := opentracing.GlobalTracer().Extract(
				opentracing.HTTPHeaders,
				opentracing.HTTPHeadersCarrier(req.Header))
			if err != nil {
				// no need to handle, we'll just create a parent span later
				//log.WithError(err).Error("unable to extract wire context")
			}

			// Create the span referring to the RPC client if available.
			// If wireContext == nil, a root span will be created.
			serverSpan = opentracing.StartSpan(
				appSpecificOperationName,
				ext.RPCServerOption(wireContext))

			defer serverSpan.Finish()

			ext.HTTPUrl.Set(serverSpan, req.URL.Path)
			serverSpan.SetTag(correlationID.CORRID, corrID)

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
