// MIT License
//
// Copyright (c) 2024 Soma Rádóczi
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package pingo

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"
)

type (
	client struct {
		client      *http.Client
		baseUrl     string
		debug       bool
		headers     http.Header
		queryParams url.Values
		timeout     time.Duration
		logger      Logger
	}

	request struct {
		method      string
		baseUrl     string
		path        string
		headers     http.Header
		queryParams url.Values
		timeout     time.Duration
		body        any
	}

	// TODO isError ?
	response[T any] struct {
		status     string
		statusCode int
		headers    http.Header
		body       T
	}

	Logger interface {
		Debug(msg string, args ...any)
		Info(msg string, args ...any)
	}
)

var (
	defaultClient = newDefaultClient()
)

func newDefaultClient() *client {
	return &client{
		client: &http.Client{},
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
	}
}

func NewClient() *client {
	c := newDefaultClient()

	return c
}

func (c *client) SetClient(client *http.Client) *client {
	c.client = client
	return c
}

func (c *client) SetBaseUrl(baseUrl string) *client {
	c.baseUrl = baseUrl
	return c
}

func (c *client) SetHeaders(headers http.Header) *client {
	c.headers = headers
	return c
}

func (c *client) SetQueryParams(queryParams url.Values) *client {
	c.queryParams = queryParams
	return c
}

func (c *client) SetTimeout(timeout time.Duration) *client {
	c.timeout = timeout
	return c
}

func (c *client) SetDebug(debug bool) *client {
	c.debug = debug
	return c
}

func (c *client) SetLogger(logger Logger) *client {
	c.logger = logger
	return c
}

func (c *client) NewRequest() *request {
	return &request{
		method:      "",
		baseUrl:     c.baseUrl,
		path:        "",
		headers:     c.headers,
		queryParams: c.queryParams,
		timeout:     c.timeout,
		body:        nil,
	}
}

func (r *request) SetMethod(method string) *request {
	r.method = method
	return r
}

func (r *request) SetBaseUrl(baseUrl string) *request {
	r.baseUrl = baseUrl
	return r
}

func (r *request) SetPath(path string) *request {
	r.path = path
	return r
}

func (r *request) SetHeaders(headers http.Header) *request {
	r.headers = headers
	return r
}

func (r *request) AddHeaders(headers http.Header) *request {
	for k, vs := range headers {
		for _, v := range vs {
			r.headers.Add(k, v)
		}
	}

	return r
}

func (r *request) SetQueryParams(queryParams url.Values) *request {
	r.queryParams = queryParams
	return r
}

func (r *request) AddQueryParams(queryParams url.Values) *request {
	for k, vs := range queryParams {
		for _, v := range vs {
			r.queryParams.Add(k, v)
		}
	}

	return r
}

func (r *request) SetTimeout(timeout time.Duration) *request {
	r.timeout = timeout
	return r
}

func Do[T any](r *request) (*response[T], error) {
	return nil, errors.New("TODO")
}

func DoAsync[T any](r *request) (<-chan response[T], error) {
	return nil, errors.New("TODO")
}
