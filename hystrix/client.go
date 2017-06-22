package hystrix

import (
	"io"
	"net/http"
	"net/url"

	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/afex/hystrix-go/hystrix"
	"github.com/mchudgins/go-service-helper/correlationID"
)

type HTTPClient struct {
	http.Client
	HystrixCommandName string
}

func NewClient(commandName string) *HTTPClient {
	return &HTTPClient{
		HystrixCommandName: commandName,
	}
}

func circuitBreaker(u, commandName string, fn func() (*http.Response, error)) (*http.Response, error) {

	output := make(chan *http.Response, 1)
	errors := make(chan error, 1)

	hystrix.Go(commandName, func() error {
		response, err := fn()
		if err != nil {
			errors <- err
		} else {
			output <- response
			if response.StatusCode == http.StatusInternalServerError {
				return fmt.Errorf("error %d", response.StatusCode)
			}
		}

		return err
	}, func(err error) error {
		log.WithError(err).WithFields(log.Fields{"commandName": commandName, "URL": u}).
			Info("breaker closed")
		return err
	})

	select {
	case r := <-output:
		return r, nil

	case err := <-errors:
		return nil, err
	}
}

func (c *HTTPClient) Do(r *http.Request) (*http.Response, error) {
	return circuitBreaker(r.URL.Path, c.HystrixCommandName, func() (*http.Response, error) {

		// send any correlation ID on to the servers we contact
		corrID := correlationID.FromContext(r.Context())
		r.Header.Set(correlationID.CORRID, corrID)

		return c.Client.Do(r)
	})
}

func (c *HTTPClient) Get(url string) (*http.Response, error) {
	return circuitBreaker(url, c.HystrixCommandName, func() (*http.Response, error) {
		return c.Client.Get(url)
	})
}

func (c *HTTPClient) Head(url string) (*http.Response, error) {
	return circuitBreaker(url, c.HystrixCommandName, func() (*http.Response, error) {
		return c.Client.Head(url)
	})

}

func (c *HTTPClient) Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	return circuitBreaker(url, c.HystrixCommandName, func() (*http.Response, error) {
		return c.Client.Post(url, contentType, body)
	})
}

func (c *HTTPClient) PostForm(url string, data url.Values) (*http.Response, error) {
	return circuitBreaker(url, c.HystrixCommandName, func() (*http.Response, error) {
		return c.Client.PostForm(url, data)
	})
}
