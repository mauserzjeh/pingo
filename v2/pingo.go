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
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
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

	responseHeader struct {
		status     string
		statusCode int
		headers    http.Header
	}

	responseStream struct {
		responseHeader
		cancel context.CancelFunc
		body   io.ReadCloser
	}

	response struct {
		responseHeader
		body []byte
	}

	Logger interface {
		Debug(msg string, args ...any)
		Info(msg string, args ...any)
	}

	ResponseUnmarshaler func(*response) error
	StreamReceiver      func(r io.Reader) error

	multipartFormFile struct {
		reader    io.Reader
		filePath  string
		fieldName string
		fileName  string
	}
)

var (
	defaultClient = newDefaultClient()

	headerContentType  = textproto.CanonicalMIMEHeaderKey("Content-Type")
	headerAccept       = textproto.CanonicalMIMEHeaderKey("Accept")
	headerCacheControl = textproto.CanonicalMIMEHeaderKey("Cache-Control")
	headerConnection   = textproto.CanonicalMIMEHeaderKey("Connection")
	headerUserAgent    = textproto.CanonicalMIMEHeaderKey("User-Agent")

	ErrRequestTimedOut = errors.New("request timed out")

	headerUserAgentDefaultValue = "pingo " + version + " (github.com/mauserzjeh/pingo)"
)

const (
	ContentTypeJson            = "application/json"
	ContentTypeXml             = "application/xml"
	ContentTypeFormUrlEncoded  = "application/x-www-form-urlencoded"
	ContentTypeTextEventStream = "text/event-stream"

	version = "v2.0.0"
)

// ---------------------------------------------- //
// Client                                         //
// ---------------------------------------------- //

func newDefaultClient() *client {
	c := &client{
		client: &http.Client{},
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
		headers:     make(http.Header),
		queryParams: make(url.Values),
	}

	c.headers.Set(headerUserAgent, headerUserAgentDefaultValue)

	return c
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
	setValues(headers, c.headers)
	return c
}

func (c *client) SetHeader(key, value string) *client {
	c.headers.Set(key, value)
	return c
}

func (c *client) AddHeaders(headers http.Header) *client {
	addValues(headers, c.headers)
	return c
}

func (c *client) AddHeader(key, value string) *client {
	c.headers.Add(key, value)
	return c
}

func (c *client) SetQueryParams(queryParams url.Values) *client {
	setValues(queryParams, c.queryParams)
	return c
}

func (c *client) SetQueryParam(key, value string) *client {
	c.queryParams.Set(key, value)
	return c
}

func (c *client) AddQueryParams(queryParams url.Values) *client {
	addValues(queryParams, c.queryParams)
	return c
}

func (c *client) AddQueryParam(key, value string) *client {
	c.queryParams.Add(key, value)
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
	setValues(headers, r.headers)
	return r
}

func (r *request) SetHeader(key, value string) *request {
	r.headers.Set(key, value)
	return r
}

func (r *request) AddHeaders(headers http.Header) *request {
	addValues(headers, r.headers)
	return r
}

func (r *request) AddHeader(key, value string) *request {
	r.headers.Add(key, value)
	return r
}

func (r *request) SetQueryParams(queryParams url.Values) *request {
	setValues(queryParams, r.queryParams)
	return r
}

func (r *request) SetQueryParam(key, value string) *request {
	r.queryParams.Set(key, value)
	return r
}

func (r *request) AddQueryParams(queryParams url.Values) *request {
	addValues(queryParams, r.queryParams)
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

func (r *request) BodyMultipartForm(data map[string]any, files ...multipartFormFile) *request {
	r.resetBody()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	// handle data
	for fieldName, value := range data {
		err := w.WriteField(fieldName, fmt.Sprint(value))
		if err != nil {
			r.bodyErr = err
			return r
		}
	}

	// handle files
	for _, file := range files {
		err := file.Write(w)
		if err != nil {
			r.bodyErr = err
			return r
		}
	}

	err := w.Close()
	if err != nil {
		r.bodyErr = err
		return r
	}

	r.body = body
	r.SetHeader(headerContentType, w.FormDataContentType())
	return r
}

func (r *request) do(ctx context.Context) (*http.Response, error) {
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

	return resp, nil
}

func (r *request) DoCtx(ctx context.Context) (*response, error) {
	resp, err := r.do(ctx)
	if err != nil {
		return nil, err
	}
	if r.cancel != nil {
		defer r.cancel()
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &response{
		responseHeader: responseHeader{
			status:     resp.Status,
			statusCode: resp.StatusCode,
			headers:    resp.Header,
		},
		body: responseBody,
	}, nil
}

func (r *request) Do() (*response, error) {
	return r.DoCtx(context.Background())
}

func (r *request) DoStream(ctx context.Context) (*responseStream, error) {
	r.headers.Set(headerAccept, ContentTypeTextEventStream)
	r.headers.Set(headerCacheControl, "no-cache")
	r.headers.Set(headerConnection, "keep-alive")

	resp, err := r.do(ctx)
	if err != nil {
		return nil, err
	}

	return &responseStream{
		responseHeader: responseHeader{
			status:     resp.Status,
			statusCode: resp.StatusCode,
			headers:    resp.Header,
		},
		body: resp.Body,
	}, nil
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
		req  *http.Request
		err  error
		rctx context.Context
	)

	if r.timeout > 0 {
		tctx, cancel := context.WithTimeoutCause(ctx, r.timeout, ErrRequestTimedOut)
		r.cancel = cancel
		rctx = tctx
	} else {
		rctx = ctx
	}

	req, err = http.NewRequestWithContext(rctx, r.method, url, body)
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
// ResponseHeader                                 //
// ---------------------------------------------- //

func (r *responseHeader) Status() string {
	return r.status
}

func (r *responseHeader) StatusCode() int {
	return r.statusCode
}

func (r *responseHeader) Headers() http.Header {
	return r.headers
}

func (r *responseHeader) GetHeader(key string) string {
	return r.headers.Get(key)
}

// ---------------------------------------------- //
// Response                                       //
// ---------------------------------------------- //

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
	return u(r)
}

// ---------------------------------------------- //
// ResponseStream                                 //
// ---------------------------------------------- //

func (r *responseStream) RecvReceiver(sr StreamReceiver) error {
	return sr(r.body)
}

func (r *responseStream) Recv(n uint) ([]byte, error) {
	b := make([]byte, n)
	nn, err := r.body.Read(b)
	if err != nil {
		return nil, err
	}
	return b[:nn], nil
}

func (r *responseStream) Close() {
	r.body.Close()
	if r.cancel != nil {
		r.cancel()
	}
}

// ---------------------------------------------- //
// MultipartFormFile                              //
// ---------------------------------------------- //

func NewMultipartFormFile(name string, filePath string) multipartFormFile {
	return multipartFormFile{
		filePath:  filePath,
		fieldName: name,
	}
}

func NewMultipartFormFileReader(name, fileName string, r io.Reader) multipartFormFile {
	return multipartFormFile{
		reader:    r,
		fieldName: name,
		fileName:  fileName,
	}
}

func (f *multipartFormFile) Write(w *multipart.Writer) error {
	if f.reader == nil {
		ff, err := os.Open(f.filePath)
		if err != nil {
			return err
		}
		defer ff.Close()
		f.reader = ff
		f.fileName = path.Base(ff.Name())
	}

	pw, err := w.CreateFormFile(f.fieldName, f.fileName)
	if err != nil {
		return err
	}

	_, err = io.Copy(pw, f.reader)
	if err != nil {
		return err
	}

	return nil
}

// ---------------------------------------------- //
// Helpers                                        //
// ---------------------------------------------- //

func setValues[T http.Header | url.Values](src, dst T) {
	switch src := any(src).(type) {
	case http.Header:
		if dst, ok := any(dst).(http.Header); ok {
			for k, vs := range src {
				if len(vs) == 0 || vs[0] == "" {
					dst.Del(k)
					continue
				}

				dst.Set(k, vs[0])
			}
		}
	case url.Values:
		if dst, ok := any(dst).(url.Values); ok {
			for k, vs := range src {
				if len(vs) == 0 || vs[0] == "" {
					dst.Del(k)
					continue
				}

				dst.Set(k, vs[0])
			}
		}
	}
}

func addValues[T http.Header | url.Values](src, dst T) {
	switch src := any(src).(type) {
	case http.Header:
		if dst, ok := any(dst).(http.Header); ok {
			for k, vs := range src {
				for _, v := range vs {
					dst.Add(k, v)
				}
			}
		}
	case url.Values:
		if dst, ok := any(dst).(url.Values); ok {
			for k, vs := range src {
				for _, v := range vs {
					dst.Add(k, v)
				}
			}
		}
	}
}
