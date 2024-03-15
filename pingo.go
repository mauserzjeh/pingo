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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

type (

	// logger is the internal logger used by the package
	logger struct {
		l          *log.Logger            // underlying [log.Logger]
		flag       atomic.Int32           // logging flags
		timeFormat atomic.Pointer[string] // format of the time part when [Ftime] flag is provided
	}

	// client is the client used by the package
	client struct {
		client       *http.Client  // underlying [net/http.Client]
		baseUrl      string        // base URL for the client
		debug        bool          // debug mode
		debugBody    bool          // debug mode to include body
		headers      http.Header   // headers for the client
		queryParams  url.Values    // query parameters for the client
		timeout      time.Duration // timeout for the client
		logger       *logger       // logger used by the client
		isLogEnabled bool          // whether logging is enabled or disabled in this client
	}

	// request is the request created by calling [NewRequest]
	request struct {
		client       *client            // the client the request was created on
		method       string             // method of the request e.g: "GET", "POST", "PUT"
		baseUrl      string             // base URL for the request
		path         string             // path of the request
		headers      http.Header        // headers for the request
		queryParams  url.Values         // query parameters for the request
		timeout      time.Duration      // timeout for the request
		body         *bytes.Buffer      // request body
		bodyErr      error              // error signaling if there was an error creating the request body
		cancel       context.CancelFunc // cancel is used to cancel any resources associated with the [context.Context] of the request
		ctx          context.Context    // [context.Context] of the request
		debug        bool               // debug mode
		debugBody    bool               // debug mode to include body
		isLogEnabled bool               // whether loggin is enabled or disabled for the request
	}

	// responseHeader contains information about response headers
	responseHeader struct {
		status     string      // status of the response
		statusCode int         // status code of the response
		headers    http.Header // headers of the response
	}

	// responseStream is a streamed response
	responseStream struct {
		responseHeader                    // response header info
		cancel         context.CancelFunc // [context.CancelFunc] to cancel any resources associated with the request/response
		reader         *bufio.Reader      // [bufio.Reader] to read the response from
		response       *http.Response     // the original [net/http.Response]
	}

	// response is the default response
	response struct {
		responseHeader        // response header info
		body           []byte // response body
	}

	// ResponseUnmarshaler is a function that can be used to unmarshal a response
	ResponseUnmarshaler func(r *response) error

	// StreamReceiver is a function that can be used to read from a streamed response
	StreamReceiver func(r *bufio.Reader) error

	// multipartFormFile contains information about a multipartform file
	multipartFormFile struct {
		reader    io.Reader // [io.Reader] to read the file data
		filePath  string    // the full filepath
		fieldName string    // name to use when performing the request
		fileName  string    // name of the file
	}
)

var (
	headerUserAgentDefaultValue = pingoWithVersion + " (github.com/mauserzjeh/pingo)"
	pingoWithVersion            = pingo + " " + version

	// default client created by the package
	defaultClient = newDefaultClient()

	// header constants

	headerContentType  = textproto.CanonicalMIMEHeaderKey("Content-Type")
	headerAccept       = textproto.CanonicalMIMEHeaderKey("Accept")
	headerCacheControl = textproto.CanonicalMIMEHeaderKey("Cache-Control")
	headerConnection   = textproto.CanonicalMIMEHeaderKey("Connection")
	headerUserAgent    = textproto.CanonicalMIMEHeaderKey("User-Agent")

	// errors

	ErrRequestTimedOut = errors.New("request timed out")
)

const (
	version           = "v2.0.0"
	pingo             = "pingo"
	defaultTimeFormat = "2006-01-02 15:04:05"

	// logger flags

	Fshortfile = 1 << iota // short file name and line number: file.go:123
	Flongfile              // full file name and line number: a/b/c/file.go:123
	Ftime                  // whether to include date-time in the log message
	FtimeUTC               // if [Ftime] is set then use UTC

	// content type headers

	ContentTypeJson            = "application/json"
	ContentTypeXml             = "application/xml"
	ContentTypeFormUrlEncoded  = "application/x-www-form-urlencoded"
	ContentTypeTextEventStream = "text/event-stream"
)

// ---------------------------------------------- //
// Logger                                         //
// ---------------------------------------------- //

// newDefaultLogger creates a new default logger
func newDefaultLogger() *logger {
	l := &logger{
		l: log.New(os.Stdout, "", 0),
	}

	l.setFlags(Ftime)
	l.setTimeFormat(defaultTimeFormat)

	return l
}

// setFlags sets the flag value
func (l *logger) setFlags(flag int) {
	l.flag.Store(int32(flag))
}

// flags returns the flag value
func (l *logger) flags() int {
	return int(l.flag.Load())
}

// setTimeFormat sets the time format
func (l *logger) setTimeFormat(format string) {
	l.timeFormat.Store(&format)
}

// timeFmt returns the time format
func (l *logger) timeFmt() string {
	return *(l.timeFormat.Load())
}

// setOutput sets the output
func (l *logger) setOutput(w io.Writer) {
	l.l.SetOutput(w)
}

// log writes the log message
func (l *logger) log(format string, args ...any) {
	t := time.Now()
	flag := l.flags()
	sb := strings.Builder{}

	// pingo label
	sb.WriteRune('[')
	sb.WriteString(pingoWithVersion)
	sb.WriteRune(']')
	sb.WriteRune(' ')

	// time
	if flag&Ftime != 0 {
		if flag&FtimeUTC != 0 {
			t = t.UTC()
		}

		timeFmt := l.timeFmt()
		sb.WriteString(t.Format(timeFmt))
		sb.WriteString(" | ")
	}

	// file + line
	if flag&(Fshortfile|Flongfile) != 0 {
		_, file, line, _ := runtime.Caller(5)
		if flag&Fshortfile != 0 {
			file = path.Base(file)
		}

		sb.WriteString(file)
		sb.WriteRune(':')
		fmt.Fprintf(&sb, "%d", line)
		sb.WriteString(" | ")
	}

	fmt.Fprintf(&sb, format, args...)
	l.l.Println(sb.String())
}

// ---------------------------------------------- //
// Client                                         //
// ---------------------------------------------- //

// newDefaultClient creates a new default client
func newDefaultClient() *client {
	c := &client{
		client:       &http.Client{},
		logger:       newDefaultLogger(),
		headers:      make(http.Header),
		queryParams:  make(url.Values),
		isLogEnabled: true,
	}

	c.headers.Set(headerUserAgent, headerUserAgentDefaultValue)

	return c
}

// NewClient creates a new client with the default settings
func NewClient() *client {
	c := newDefaultClient()

	return c
}

// SetClient sets the underlying [net/http.Client]
func (c *client) SetClient(client *http.Client) *client {
	c.client = client
	return c
}

// SetBaseUrl sets the base URL
func (c *client) SetBaseUrl(baseUrl string) *client {
	c.baseUrl = baseUrl
	return c
}

// SetHeaders sets the header values
func (c *client) SetHeaders(headers http.Header) *client {
	setValues(headers, c.headers)
	return c
}

// SetHeader sets a single header value
func (c *client) SetHeader(key, value string) *client {
	c.headers.Set(key, value)
	return c
}

// AddHeaders adds the header values
func (c *client) AddHeaders(headers http.Header) *client {
	addValues(headers, c.headers)
	return c
}

// AddHeader adds a single header value
func (c *client) AddHeader(key, value string) *client {
	c.headers.Add(key, value)
	return c
}

// SetQueryParams sets the query parameters
func (c *client) SetQueryParams(queryParams url.Values) *client {
	setValues(queryParams, c.queryParams)
	return c
}

// SetQueryParam sets a single query parameter
func (c *client) SetQueryParam(key, value string) *client {
	c.queryParams.Set(key, value)
	return c
}

// AddQueryParams adds the query parameters
func (c *client) AddQueryParams(queryParams url.Values) *client {
	addValues(queryParams, c.queryParams)
	return c
}

// AddQueryParam adds a single query parameter
func (c *client) AddQueryParam(key, value string) *client {
	c.queryParams.Add(key, value)
	return c
}

// SetTimeout sets the timeout
func (c *client) SetTimeout(timeout time.Duration) *client {
	c.timeout = timeout
	return c
}

// SetDebug sets the debug mode
func (c *client) SetDebug(debug, includeBody bool) *client {
	c.debug = debug
	c.debugBody = includeBody
	return c
}

// SetLogEnabled sets the log mode
func (c *client) SetLogEnabled(enable bool) *client {
	c.isLogEnabled = enable
	return c
}

// SetLogTimeFormat sets the log time format if [Ftime] flag is given
func (c *client) SetLogTimeFormat(layout string) *client {
	c.logger.setTimeFormat(layout)
	return c
}

// SetLogOutput sets the log output to the given [io.Writer]
func (c *client) SetLogOutput(w io.Writer) *client {
	c.logger.setOutput(w)
	return c
}

// SetLogFlags sets the log flags
func (c *client) SetLogFlags(flag int) *client {
	c.logger.setFlags(flag)
	return c
}

// NewRequest creates a new request
func (c *client) NewRequest() *request {
	return &request{
		client:       c,
		method:       http.MethodGet,
		baseUrl:      c.baseUrl,
		path:         "",
		headers:      c.headers,
		queryParams:  c.queryParams,
		timeout:      c.timeout,
		body:         nil,
		bodyErr:      nil,
		cancel:       nil,
		ctx:          nil,
		debug:        c.debug,
		debugBody:    c.debugBody,
		isLogEnabled: c.isLogEnabled,
	}
}

// ---------------------------------------------- //
// Request                                        //
// ---------------------------------------------- //

// NewRequest creates a new request
func NewRequest() *request {
	return defaultClient.NewRequest()
}

// SetDebug sets the debug mode
func (r *request) SetDebug(debug, includeBody bool) *request {
	r.debug = debug
	r.debugBody = includeBody
	return r
}

// SetLogEnabled sets the log mode
func (r *request) SetLogEnabled(enabled bool) *request {
	r.isLogEnabled = enabled
	return r
}

// SetMethod sets the request method
// e.g.: "GET", "POST", "PUT"
func (r *request) SetMethod(method string) *request {
	if method != "" {
		r.method = method
	}
	return r
}

// SetBaseUrl sets the base URL
func (r *request) SetBaseUrl(baseUrl string) *request {
	r.baseUrl = baseUrl
	return r
}

// SetPath sets the request path
func (r *request) SetPath(path string) *request {
	r.path = path
	return r
}

// SetHeaders sets the header values
func (r *request) SetHeaders(headers http.Header) *request {
	setValues(headers, r.headers)
	return r
}

// SetHeader sets a single header value
func (r *request) SetHeader(key, value string) *request {
	r.headers.Set(key, value)
	return r
}

// AddHeaders adds the header values
func (r *request) AddHeaders(headers http.Header) *request {
	addValues(headers, r.headers)
	return r
}

// AddHeader adds a single header value
func (r *request) AddHeader(key, value string) *request {
	r.headers.Add(key, value)
	return r
}

// SetQueryParams sets the query parameters
func (r *request) SetQueryParams(queryParams url.Values) *request {
	setValues(queryParams, r.queryParams)
	return r
}

// SetQueryParam sets a single query parameter
func (r *request) SetQueryParam(key, value string) *request {
	r.queryParams.Set(key, value)
	return r
}

// AddQueryParams adds the query parameters
func (r *request) AddQueryParams(queryParams url.Values) *request {
	addValues(queryParams, r.queryParams)
	return r
}

// AddQueryParam adds a single query parameter
func (r *request) AddQueryParam(key, value string) *request {
	r.queryParams.Add(key, value)
	return r
}

// SetTimeout sets the timeout
func (r *request) SetTimeout(timeout time.Duration) *request {
	r.timeout = timeout
	return r
}

// BodyJson prepares the body as a JSON request with the given data.
// Content-Type header is automatically set to "application/json"
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

// BodyXml prepares the body as an XML request with the given data.
// Content-Type header is automatically set to "application/xml"
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

// BodyFormUrlEncoded prepares the body as a form URL encoded request with the given data.
// Content-Type header is automatically set to "application/x-www-form-urlencoded"
func (r *request) BodyFormUrlEncoded(data url.Values) *request {
	r.resetBody()
	r.SetHeader(headerContentType, ContentTypeFormUrlEncoded)

	r.body = bytes.NewBufferString(data.Encode())
	return r
}

// BodyCustom prepares the body with the given callback function
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

// BodyRaw prepares the body with the given raw data bytes
func (r *request) BodyRaw(data []byte) *request {
	r.resetBody()
	r.body = bytes.NewBuffer(data)
	return r
}

// BodyMultipartForm prepares the body as a multipartform request with the given data and files.
// Content-Type header is automatically set to "multipart/form-data" with the proper boundary.
// Use [NewMultipartFormFile] or [NewMultipartFormFileReader] to pass files for file upload
func (r *request) BodyMultipartForm(data map[string]any, files ...multipartFormFile) *request {
	r.resetBody()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	// handle data
	for fieldName, value := range data {
		err := w.WriteField(fieldName, fmt.Sprint(value))
		if err != nil {
			r.bodyErr = err
			w.Close()
			return r
		}
	}

	// handle files
	for _, file := range files {
		err := file.write(w)
		if err != nil {
			r.bodyErr = err
			w.Close()
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

// do performs the request with the given [context.Context]
func (r *request) do(ctx context.Context) (*http.Response, error) {
	var (
		reqDump, resDump []byte
		now              = time.Now()
		statusCode       int
		err              error
	)

	requestUrl := r.requestUrl()

	defer func() {
		if err == nil && r.isLogEnabled {
			r.client.logger.log("%s", createLog(r.method, statusCode, requestUrl, time.Since(now), reqDump, resDump, r.debug))
		}
	}()

	requestBody, err := r.requestBody()
	if err != nil {
		return nil, err
	}

	req, err := r.createRequest(ctx, requestUrl, requestBody)
	if err != nil {
		return nil, err
	}

	if r.isLogEnabled && r.debug {
		reqDump, _ = httputil.DumpRequestOut(req, r.debugBody)
	}

	resp, err := r.client.client.Do(req)
	if err != nil {
		select {
		case <-r.ctx.Done():
			err = fmt.Errorf("%v \"%v\": %w", strings.ToUpper(r.method), requestUrl, context.Cause(r.ctx))
		default:
		}

		return nil, err
	}

	statusCode = resp.StatusCode

	if r.isLogEnabled && r.debug {
		resDump, _ = httputil.DumpResponse(resp, r.debugBody)
	}

	return resp, nil
}

// DoCtx performs the request with the given [context.Context] and returns a response
func (r *request) DoCtx(ctx context.Context) (*response, error) {
	resp, err := r.do(ctx)
	if err != nil {
		return nil, err
	}
	if r.cancel != nil {
		r.cancel()
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

// Do performs the request using [context.Background]
func (r *request) Do() (*response, error) {
	return r.DoCtx(context.Background())
}

// DoStream performs a request using the given [context.Context] and returns a streaming response
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
		reader:   bufio.NewReader(resp.Body),
		response: resp,
		cancel:   r.cancel,
	}, nil
}

// requestUrl creates the request url
func (r *request) requestUrl() string {
	b := strings.Builder{}

	baseUrl := strings.TrimRight(r.baseUrl, "/")
	if baseUrl != "" {
		b.WriteString(baseUrl)
	}

	path := strings.TrimLeft(r.path, "/")
	if path != "" {

		if b.Len() > 0 {
			b.WriteString("/")
		}

		b.WriteString(path)
	}

	return b.String()
}

// requestBody creates the request body
func (r *request) requestBody() (io.Reader, error) {
	if r.bodyErr != nil {
		return nil, r.bodyErr
	}

	if r.body == nil {
		return http.NoBody, nil
	}

	return r.body, nil
}

// createRequest creates a [net/http.Request]
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

	r.ctx = rctx
	req, err = http.NewRequestWithContext(rctx, r.method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header = r.headers

	query := req.URL.Query()
	for k, vs := range r.queryParams {
		for _, v := range vs {
			query.Set(k, v)
		}
	}

	req.URL.RawQuery = query.Encode()

	return req, nil
}

// resetBody resets the request body and bodyErr if subsequent SetBody* functions are called on the request
func (r *request) resetBody() {
	r.body = nil
	r.bodyErr = nil
}

// ---------------------------------------------- //
// ResponseHeader                                 //
// ---------------------------------------------- //

// Status returns the status of a response
func (r *responseHeader) Status() string {
	return r.status
}

// StatusCode returns the status code of a response
func (r *responseHeader) StatusCode() int {
	return r.statusCode
}

// Headers returns the response headers
func (r *responseHeader) Headers() http.Header {
	return r.headers
}

// GetHeader is a convenience method to retrieve a single response header value
func (r *responseHeader) GetHeader(key string) string {
	return r.headers.Get(key)
}

// ---------------------------------------------- //
// Response                                       //
// ---------------------------------------------- //

// BodyRaw returns the response body as a byte slice
func (r *response) BodyRaw() []byte {
	return r.body
}

// BodyString returns the response body as string
func (r *response) BodyString() string {
	return string(r.body)
}

// IsError returns a non nil error if the response is considered as an error based on the status code.
// The error's text will be the response body
func (r *response) IsError() error {
	if r.statusCode < 200 || r.statusCode >= 400 {
		return fmt.Errorf("%s", r.body)
	}

	return nil
}

// Unmarshal is a convenience method that can receive a [ResponseUnmarshaler] callback
// function that performs the unmarshalling of the response body
func (r *response) Unmarshal(u ResponseUnmarshaler) error {
	return u(r)
}

// ---------------------------------------------- //
// ResponseStream                                 //
// ---------------------------------------------- //

// RecvFunc can receive a [StreamReceiver] callback function that performs
// the stream reading of the streamed response body
func (r *responseStream) RecvFunc(sr StreamReceiver) error {
	return sr(r.reader)
}

// Recv reads up to n bytes from a streamed response body
func (r *responseStream) Recv(n uint) ([]byte, error) {
	b := make([]byte, n)
	nn, err := r.reader.Read(b)
	if err != nil {
		return nil, err
	}
	return b[:nn], nil
}

// Close closes the streamed response body and additionally frees up any
// resources associated with the [context.Context] used to perform the streamed request
func (r *responseStream) Close() {
	r.response.Body.Close()
	if r.cancel != nil {
		r.cancel()
	}
}

// ---------------------------------------------- //
// MultipartFormFile                              //
// ---------------------------------------------- //

// NewMultipartFormFile creates a new multipartform file by reading the file from the given filepath
func NewMultipartFormFile(name string, filePath string) multipartFormFile {
	return multipartFormFile{
		filePath:  filePath,
		fieldName: name,
	}
}

// NewMultipartFormFileReader creates a new multipartform file by using the given [io.Reader]
func NewMultipartFormFileReader(name, fileName string, r io.Reader) multipartFormFile {
	return multipartFormFile{
		reader:    r,
		fieldName: name,
		fileName:  fileName,
	}
}

// write writes the contents of the file to the given [mime/multipart.Writer]
func (f *multipartFormFile) write(w *multipart.Writer) error {
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

// setValues is a helper function that sets [net/http.Header] or [net/url.Values]
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

// setValues is a helper function that adds [net/http.Header] or [net/url.Values]
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

// formatDump formats the given dump
func formatDump(label string, dump []byte) string {
	sb := strings.Builder{}

	format := "|  %s  | %s\n"

	sb.WriteString(strings.Repeat("-", len(format)-5))
	sb.WriteRune('\n')

	ls := bytes.Split(dump, []byte("\n"))
	for i, line := range ls {
		c := " "
		if i <= len(label) && i > 0 {
			c = string(label[i-1])
		}

		fmt.Fprintf(&sb, format, c, line)
	}

	if len(ls)-1 <= len(label) {
		remainder := label[len(ls)-1:]
		for _, r := range remainder {
			fmt.Fprintf(&sb, format, string(r), "")
		}
	}
	fmt.Fprintf(&sb, format, " ", "")

	sb.WriteString(strings.Repeat("-", len(format)-5))
	sb.WriteRune('\n')

	return sb.String()
}

// debugLog creates a debug log for the request
func debugLog(reqDump, resDump []byte) string {
	sb := strings.Builder{}

	sb.WriteRune('\n')

	label := "REQUEST"
	d := formatDump(label, reqDump)
	sb.WriteString(d)

	sb.WriteRune('\n')

	label = "RESPONSE"
	d = formatDump(label, resDump)
	sb.WriteString(d)

	return sb.String()
}

// createLog creates a log message for the request
func createLog(method string, statusCode int, url string, duration time.Duration, reqDump, resDump []byte, debug bool) string {
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "%v | %v | %v | %v", method, statusCode, url, duration)

	if debug {
		fmt.Fprintf(&sb, "\n%s", debugLog(reqDump, resDump))
	}

	return sb.String()
}
