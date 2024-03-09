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
	logger struct {
		l          *log.Logger
		flag       atomic.Int32
		timeFormat atomic.Pointer[string]
	}

	client struct {
		client           *http.Client
		baseUrl          string
		debug            bool
		debugIncludeBody bool
		headers          http.Header
		queryParams      url.Values
		timeout          time.Duration
		logger           *logger
		isLogEnabled     bool
	}

	request struct {
		client           *client
		method           string
		baseUrl          string
		path             string
		headers          http.Header
		queryParams      url.Values
		timeout          time.Duration
		body             *bytes.Buffer
		bodyErr          error
		cancel           context.CancelFunc
		ctx              context.Context
		debug            bool
		debugIncludeBody bool
		isLogEnabled     bool
	}

	responseHeader struct {
		status     string
		statusCode int
		headers    http.Header
	}

	responseStream struct {
		responseHeader
		cancel   context.CancelFunc
		reader   *bufio.Reader
		response *http.Response
	}

	response struct {
		responseHeader
		body []byte
	}

	ResponseUnmarshaler func(*response) error
	StreamReceiver      func(r *bufio.Reader) error

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

	headerUserAgentDefaultValue = pingoWithVersion + " (github.com/mauserzjeh/pingo)"

	pingoWithVersion = pingo + " " + version
)

const (
	Fshortfile = 1 << iota
	Flongfile
	Ftime
	FUTC

	ContentTypeJson            = "application/json"
	ContentTypeXml             = "application/xml"
	ContentTypeFormUrlEncoded  = "application/x-www-form-urlencoded"
	ContentTypeTextEventStream = "text/event-stream"

	version           = "v2.0.0"
	pingo             = "pingo"
	defaultTimeFormat = "2006-01-02 15:04:05"
)

// ---------------------------------------------- //
// Logger                                         //
// ---------------------------------------------- //

func newDefaultLogger() *logger {
	l := &logger{
		l: log.New(os.Stdout, "", 0),
	}

	l.setFlags(Ftime)
	l.setTimeFormat(defaultTimeFormat)

	return l
}

func (l *logger) setFlags(flag int) {
	l.flag.Store(int32(flag))
}

func (l *logger) flags() int {
	return int(l.flag.Load())
}

func (l *logger) setTimeFormat(format string) {
	l.timeFormat.Store(&format)
}

func (l *logger) timeFmt() string {
	return *(l.timeFormat.Load())
}

func (l *logger) setOutput(w io.Writer) {
	l.l.SetOutput(w)
}

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
		if flag&FUTC != 0 {
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

func (c *client) SetDebug(debug, includeBody bool) *client {
	c.debug = debug
	c.debugIncludeBody = includeBody
	return c
}

func (c *client) SetLogEnabled(enable bool) *client {
	c.isLogEnabled = enable
	return c
}

func (c *client) SetLogTimeFormat(layout string) *client {
	c.logger.setTimeFormat(layout)
	return c
}

func (c *client) SetLogOutput(w io.Writer) *client {
	c.logger.setOutput(w)
	return c
}

func (c *client) SetLogFlags(flag int) *client {
	c.logger.setFlags(flag)
	return c
}

func (c *client) NewRequest() *request {
	return &request{
		client:           c,
		method:           http.MethodGet,
		baseUrl:          c.baseUrl,
		path:             "",
		headers:          c.headers,
		queryParams:      c.queryParams,
		timeout:          c.timeout,
		body:             nil,
		bodyErr:          nil,
		cancel:           nil,
		ctx:              nil,
		debug:            c.debug,
		debugIncludeBody: c.debugIncludeBody,
		isLogEnabled:     c.isLogEnabled,
	}
}

// ---------------------------------------------- //
// Request                                        //
// ---------------------------------------------- //

func NewRequest() *request {
	return defaultClient.NewRequest()
}

func (r *request) SetDebug(debug, includeBody bool) *request {
	r.debug = debug
	r.debugIncludeBody = includeBody
	return r
}

func (r *request) SetLogEnabled(enabled bool) *request {
	r.isLogEnabled = enabled
	return r
}

func (r *request) SetMethod(method string) *request {
	if method != "" {
		r.method = method
	}
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
			w.Close()
			return r
		}
	}

	// handle files
	for _, file := range files {
		err := file.Write(w)
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
		reqDump, _ = httputil.DumpRequestOut(req, r.debugIncludeBody)
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
		resDump, _ = httputil.DumpResponse(resp, r.debugIncludeBody)
	}

	return resp, nil
}

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
		reader:   bufio.NewReader(resp.Body),
		response: resp,
		cancel:   r.cancel,
	}, nil
}

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

func (r *request) requestBody() (io.Reader, error) {
	if r.bodyErr != nil {
		return nil, r.bodyErr
	}

	if r.body == nil {
		return http.NoBody, nil
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

func (r *responseStream) RecvFunc(sr StreamReceiver) error {
	return sr(r.reader)
}

func (r *responseStream) Recv(n uint) ([]byte, error) {
	b := make([]byte, n)
	nn, err := r.reader.Read(b)
	if err != nil {
		return nil, err
	}
	return b[:nn], nil
}

func (r *responseStream) Close() {
	r.response.Body.Close()
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

func formatDump(label string, dump []byte) string {
	sb := strings.Builder{}

	format := "|  %s  | %s\n"

	sb.WriteString(strings.Repeat("-", len(format)-5))
	sb.WriteRune('\n')

	// fmt.Fprintf(&sb, format, " ", "")

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

func createLog(method string, statusCode int, url string, duration time.Duration, reqDump, resDump []byte, debug bool) string {
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "%v | %v | %v | %v", method, statusCode, url, duration)

	if debug {
		fmt.Fprintf(&sb, "\n%s", debugLog(reqDump, resDump))
	}

	return sb.String()
}
