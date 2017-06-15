package httpWriter

import (
	"io"
	"net/http"
	"net/url"

	"github.com/afex/hystrix-go/hystrix"
)

type Client struct {
	http.Client
	HystrixCommandName string
}

func NewClient(commandName string) *Client {
	return &Client{
		HystrixCommandName: commandName,
	}
}

func (c *Client) Do(r *http.Request) (*http.Response, error) {
	hystrix.Go(c.HystrixCommandName, func() error {
		c.Client.Do(r)
		return nil
	}, nil)
}

func (c *Client) Get(url string) (*http.Response, error) {
	return c.Client.Get(url)
}

func (c *Client) Head(url string) (*http.Response, error) {
	return c.Client.Head(url)
}

func (c *Client) Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	return c.Client.Post(url, contentType, body)
}

func (c *Client) PostForm(url string, data url.Values) (*http.Response, error) {
	return c.Client.PostForm(url, data)
}
