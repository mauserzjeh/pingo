// MIT License
//
// Copyright (c) 2022 Soma Rádóczi
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"
)

const (
	GET    method = http.MethodGet
	HEAD   method = http.MethodHead
	POST   method = http.MethodPost
	PUT    method = http.MethodPut
	PATCH  method = http.MethodPatch
	DELETE method = http.MethodDelete
)

type (
	option func(c *client)
	method string

	Options struct {
		BaseUrl     string
		Timeout     time.Duration
		Logf        func(string, ...any)
		Debug       bool
		Client      http.Client
		Headers     http.Header
		QueryParams map[string]any
	}

	request struct {
		Method      method
		Path        string
		Headers     http.Header
		QueryParams map[string]any
		data        []byte
	}

	response struct {
		processResp func([]byte, int, http.Header) error
		data        any
		statusCode  int
		headers     http.Header
	}

	client struct {
		client      http.Client
		baseUrl     string
		timeout     time.Duration
		logf        func(string, ...any)
		debug       bool
		headers     http.Header
		queryParams map[string]any
	}

	ClientError struct {
		Response   []byte
		StatusCode int
		Headers    http.Header
	}
)

// Client  -------------------------------------------------

func NewClient(opts ...option) *client {
	c := client{}
	SetOptions(&c, opts...)
	return &c
}

func (c *client) Request(req *request, res *response) error {
	start := time.Now()

	requestUrl := path.Join(c.baseUrl, req.Path)
	if c.debug {
		c.logf("[REQUEST] %s: %s\n", req.Method, requestUrl)
	}

	defer func() {
		c.logf("%s %s %s", req.Method, requestUrl, time.Since(start))
	}()

	var reqBody io.Reader
	if req.data != nil && len(req.data) > 0 {
		reqBody = bytes.NewBuffer(req.data)
		if c.debug {
			c.logf("[REQUEST BODY] %s\n", reqBody)
		}
	}

	request, err := http.NewRequest(string(req.Method), requestUrl, reqBody)
	if err != nil {
		return err
	}

	request.Header = c.headers
	for k, v := range req.Headers {
		if len(v) > 0 {
			request.Header.Set(k, v[0])
		}
	}

	query := request.URL.Query()
	for k, v := range c.queryParams {
		query.Set(k, fmt.Sprint(v))
	}

	if req.QueryParams != nil {
		for k, v := range req.QueryParams {
			query.Set(k, fmt.Sprint(v))
		}
	}

	request.URL.RawQuery = query.Encode()

	if c.debug {
		c.logf("[REQUEST PARAMS] %q\n", request.URL.Query())
		dump, _ := httputil.DumpRequestOut(request, true)
		c.logf("[REQUEST] %s\n", dump)
	}

	cl := c.client

	response, err := cl.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if c.debug {
		dump, _ := httputil.DumpResponse(response, true)
		c.logf("[RESPONSE] %s\n", dump)
	}

	resBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return ClientError{
			Response:   resBody,
			StatusCode: response.StatusCode,
			Headers:    response.Header,
		}
	}

	return res.processResp(resBody, response.StatusCode, response.Header)
}

// Request -------------------------------------------------
func NewRawRequest(data []byte) *request {
	return &request{
		data: data,
	}
}

func NewJsonRequest(data any) (*request, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	h := http.Header{}
	h.Add("Content-Type", "application/json")

	return &request{
		data:    jsonData,
		Headers: h,
	}, nil
}

func NewFormRequest(data any) (*request, error) {
	values, err := createUrlValues(data)
	if err != nil {
		return nil, err
	}

	h := http.Header{}
	h.Add("Content-Type", "application/x-www-form-urlencoded")

	return &request{
		Headers: h,
		data:    []byte(values.Encode()),
	}, nil
}

// Response ------------------------------------------------
func (r *response) Data() any {
	return r.data
}

func (r *response) Headers() http.Header {
	return r.headers
}

func (r *response) StatusCode() int {
	return r.statusCode
}

func NewRawResponse() *response {
	r := response{}
	r.processResp = func(res []byte, statusCode int, headers http.Header) error {
		r.data = res
		r.statusCode = statusCode
		r.headers = headers
		return nil
	}

	return &r
}

func NewJsonResponse(data any) *response {
	r := response{}
	r.processResp = func(res []byte, statusCode int, headers http.Header) error {
		err := json.Unmarshal(res, r.data)
		if err != nil {
			return err
		}

		r.statusCode = statusCode
		r.headers = headers
		return nil
	}

	return &r
}

// ClientError ---------------------------------------------
func (e ClientError) String() string {
	return fmt.Sprintf("status code: %v\nresponse body: %s\n", e.StatusCode, e.Response)
}

func (e ClientError) Error() string {
	return e.String()
}

// Options -------------------------------------------------
func SetOptions(c *client, opts ...option) {
	for _, optionf := range opts {
		optionf(c)
	}
}

func SetOption(c *client, o option) {
	o(c)
}

func SetOptionsStruct(o Options) option {
	return func(c *client) {
		c.baseUrl = o.BaseUrl
		c.timeout = o.Timeout
		c.logf = o.Logf

		if c.logf == nil {
			c.logf = log.Printf
		}

		c.debug = o.Debug
		c.client = o.Client
		c.headers = o.Headers
		c.queryParams = o.QueryParams
	}
}

func BaseUrl(url string) option {
	return func(c *client) {
		c.baseUrl = url
	}
}

func Timeout(d time.Duration) option {
	return func(c *client) {
		c.timeout = d
	}
}

func Logf(f func(string, ...any)) option {
	return func(c *client) {
		c.logf = f
	}
}

func Debug(d bool) option {
	return func(c *client) {
		c.debug = d
	}
}

func Client(cl http.Client) option {
	return func(c *client) {
		c.client = cl
	}
}

func Header(headers http.Header) option {
	return func(c *client) {
		c.headers = headers
	}
}

func SetQueryParams(queryParams map[string]any) option {
	return func(c *client) {
		c.queryParams = queryParams
	}
}

// Helpers -------------------------------------------------
func createUrlValues(data any) (url.Values, error) {
	t := reflect.ValueOf(data).Type().Kind()
	values := url.Values{}

	switch t {
	case reflect.Struct:
		m := structFieldsToMap(data)
		for k, v := range m {
			values.Add(k, fmt.Sprint(v))
		}
		return values, nil
	case reflect.Map:
		iter := reflect.ValueOf(data).MapRange()
		for iter.Next() {
			values.Add(iter.Key().String(), iter.Value().String())
		}
		return values, nil
	default:
		return nil, errors.New("data must be type of struct or map")
	}
}

func structFieldsToMap(s any) map[string]any {
	m := make(map[string]any)

	val := reflect.ValueOf(s)
	for i := 0; i < val.NumField(); i++ {
		fieldType := val.Type().Field(i)

		// skip unexported fields
		if !val.Field(i).CanInterface() {
			continue
		}

		fieldValue := val.Field(i).Interface()

		// if we have a promoted field then recursively run the function on it
		if fieldType.Anonymous {
			subM := structFieldsToMap(fieldValue)
			for k, v := range subM {
				m[k] = v
			}
			continue
		}

		if tag, ok := fieldType.Tag.Lookup("form"); ok {
			tagName := strings.Split(tag, ",")[0]

			if tagName != "-" {
				m[tagName] = fieldValue
			}
		}
	}

	return m
}
