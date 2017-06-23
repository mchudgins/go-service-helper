package zipkin

import (
	"io"
	"net/http"
	"net/url"

	"github.com/mchudgins/go-service-helper/correlationID"
	"github.com/mchudgins/go-service-helper/hystrix"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

type TraceClient struct {
	*hystrix.HTTPClient
}

func NewClient(commandName string) *TraceClient {
	return &TraceClient{
		HTTPClient: hystrix.NewClient(commandName),
	}
}

func (c *TraceClient) Do(r *http.Request) (*http.Response, error) {

	ctx := r.Context()

	// send any correlation ID on to the servers we contact
	corrID := correlationID.FromContext(ctx)
	if len(corrID) > 0 {
		r.Header.Set(correlationID.CORRID, corrID)
	}

	// enable tracing
	var childSpan opentracing.Span
	span := opentracing.SpanFromContext(ctx)

	if span == nil {
		childSpan = opentracing.StartSpan(c.HystrixCommandName)
	} else {
		childSpan = opentracing.StartSpan(c.HystrixCommandName,
			opentracing.ChildOf(span.Context()))

	}
	defer childSpan.Finish()

	ext.SpanKindRPCClient.Set(childSpan)

	//r = r.WithContext(ctx)

	// Transmit the span's TraceContext as HTTP headers on our
	// outbound request.
	opentracing.GlobalTracer().Inject(
		childSpan.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header))

	return c.Client.Do(r)
}

func (c *TraceClient) Get(url string) (*http.Response, error) {
	return c.Client.Get(url)
}

func (c *TraceClient) Head(url string) (*http.Response, error) {
	return c.Client.Head(url)
}

func (c *TraceClient) Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	return c.Client.Post(url, contentType, body)
}

func (c *TraceClient) PostForm(url string, data url.Values) (*http.Response, error) {
	return c.Client.PostForm(url, data)
}
