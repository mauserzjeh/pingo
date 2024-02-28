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
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"reflect"
	"strings"
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
		client      *client
		method      string
		baseUrl     string
		path        string
		headers     http.Header
		queryParams url.Values
		timeout     time.Duration
		body        *bytes.Buffer
		bodyErr     error
		cancel      context.CancelFunc
	}

	response struct {
		status     string
		statusCode int
		headers    http.Header
		body       []byte
	}

	Logger interface {
		Debug(msg string, args ...any)
		Info(msg string, args ...any)
	}

	ResponseUnmarshaler interface {
		UnmarshalResponse(*response) error
	}
)

var (
	defaultClient = newDefaultClient()

	headerContentType = textproto.CanonicalMIMEHeaderKey("Content-Type")
)

const (
	ContentTypeJson           = "application/json"
	ContentTypeXml            = "application/xml"
	ContentTypeFormUrlEncoded = "application/x-www-form-urlencoded"
)

// ---------------------------------------------- //
// Client                                         //
// ---------------------------------------------- //

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

func Client() *client {
	return defaultClient
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
		client:      c,
		method:      "",
		baseUrl:     c.baseUrl,
		path:        "",
		headers:     c.headers,
		queryParams: c.queryParams,
		timeout:     c.timeout,
		body:        nil,
	}
}

// ---------------------------------------------- //
// Request                                        //
// ---------------------------------------------- //

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
	for k, vs := range headers {
		if len(vs) == 0 {
			continue
		}

		r.headers.Set(k, vs[0])
	}
	return r
}

func (r *request) SetHeader(key, value string) *request {
	r.headers.Set(key, value)
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

func (r *request) AddHeader(key, value string) *request {
	r.headers.Add(key, value)
	return r
}

func (r *request) BodyJson(data any) *request {
	r.resetBody()
	r.SetHeader(headerContentType, ContentTypeJson)

	b, err := json.Marshal(data)
	if err != nil {
		r.bodyErr = err
		return r
	}

	r.body = bytes.NewBuffer(b)
	return r
}

func (r *request) BodyXml(data any) *request {
	r.resetBody()
	r.SetHeader(headerContentType, ContentTypeXml)

	b, err := xml.Marshal(data)
	if err != nil {
		r.bodyErr = err
		return r
	}

	r.body = bytes.NewBuffer(b)
	return r
}

func (r *request) BodyFormUrlEncoded(data url.Values) *request {
	r.resetBody()
	r.SetHeader(headerContentType, ContentTypeFormUrlEncoded)

	r.body = bytes.NewBufferString(data.Encode())
	return r
}

func (r *request) BodyCustom(f func() (*bytes.Buffer, error)) *request {
	r.resetBody()

	body, err := f()
	if err != nil {
		r.bodyErr = err
		return r
	}

	r.body = body
	return r
}

func (r *request) BodyRaw(data []byte) *request {
	r.resetBody()
	r.body = bytes.NewBuffer(data)
	return r
}

func (r *request) SetQueryParams(queryParams url.Values) *request {
	for k, vs := range queryParams {
		if len(vs) == 0 {
			continue
		}

		r.queryParams.Set(k, vs[0])
	}
	return r
}

func (r *request) SetQueryParam(key, value string) *request {
	r.queryParams.Set(key, value)
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

func (r *request) AddQueryParam(key, value string) *request {
	r.queryParams.Add(key, value)
	return r
}

func (r *request) SetTimeout(timeout time.Duration) *request {
	r.timeout = timeout
	return r
}

func (r *request) DoCtx(ctx context.Context) (*response, error) {
	requestUrl := r.requestUrl()

	requestBody, err := r.requestBody()
	if err != nil {
		return nil, err
	}

	req, err := r.createRequest(ctx, requestUrl, requestBody)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &response{
		status:     resp.Status,
		statusCode: resp.StatusCode,
		headers:    resp.Header,
		body:       responseBody,
	}, nil
}

func (r *request) Do() (*response, error) {
	return r.DoCtx(context.Background())
}

func (r *request) requestUrl() string {
	return fmt.Sprintf("%s/%s", strings.TrimRight(r.baseUrl, "/"), strings.TrimLeft(r.path, "/"))
}

func (r *request) requestBody() (io.Reader, error) {
	if r.body == nil {
		return http.NoBody, nil
	}

	if r.bodyErr != nil {
		return nil, r.bodyErr
	}

	return r.body, nil
}

func (r *request) createRequest(ctx context.Context, url string, body io.Reader) (*http.Request, error) {
	var (
		req *http.Request
		err error
	)

	if r.timeout > 0 {
		req, err = http.NewRequestWithContext(ctx, r.method, url, body)
	} else {
		tctx, cancel := context.WithTimeoutCause(ctx, r.timeout, errors.New("request timed out"))
		r.cancel = cancel
		req, err = http.NewRequestWithContext(tctx, r.method, url, body)
	}

	if err != nil {
		return nil, err
	}

	req.Header = r.headers

	query := req.URL.Query()
	for k, vs := range r.queryParams {
		for _, v := range vs {
			query.Add(k, v)
		}
	}

	req.URL.RawQuery = query.Encode()

	return req, nil
}

func (r *request) resetBody() {
	r.body = nil
	r.bodyErr = nil
}

// ---------------------------------------------- //
// Response                                       //
// ---------------------------------------------- //

func (r *response) Status() string {
	return r.status
}

func (r *response) StatusCode() int {
	return r.statusCode
}

func (r *response) Headers() http.Header {
	return r.headers
}

func (r *response) BodyRaw() []byte {
	return r.body
}

func (r *response) BodyString() string {
	return string(r.body)
}

func (r *response) IsError() error {
	if r.statusCode < 200 || r.statusCode >= 400 {
		return fmt.Errorf("%s", r.body)
	}

	return nil
}

func (r *response) Unmarshal(u ResponseUnmarshaler) error {
	return u.UnmarshalResponse(r)
}

// ---------------------------------------------- //
// Helpers                                        //
// ---------------------------------------------- //

func kind(v any) reflect.Kind {
	return reflect.Indirect(reflect.ValueOf(v)).Type().Kind()
}
