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
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strings"
	"time"
)

// Constants
const (

	// Method constants
	GET    method = http.MethodGet
	HEAD   method = http.MethodHead
	POST   method = http.MethodPost
	PUT    method = http.MethodPut
	PATCH  method = http.MethodPatch
	DELETE method = http.MethodDelete
)

// Types
type (

	// Method type
	method string

	// Option function that sets options on the client
	option func(c *client)

	// Client options
	Options struct {
		BaseUrl     string               // Base URL for the client
		Timeout     time.Duration        // Client timeout
		Logf        func(string, ...any) // Logger function
		Debug       bool                 // Debug mode
		Client      *http.Client         // http.Client that the client will use
		Headers     http.Header          // Client headers
		QueryParams url.Values           // Client query parameters
	}

	// Request option function that sets options on the request
	requestOption func(r *requestOptions)

	// Request options
	requestOptions struct {
		gzip                 bool // Whether the response is gzipped or not
		overwriteHeaders     bool // Whether to overwrite existing headers
		overwriteQueryParams bool // Whether to overwrite existing query parameters
	}

	// request struct holds necessary data for a request
	request struct {
		Method      method      // Method of the request
		Path        string      // Path of the request
		Headers     http.Header // Headers of the request
		QueryParams url.Values  // Query parameters of the request
		data        []byte      // Request data
	}

	// response struct holds the necessary data for a response
	response struct {
		processResp func([]byte, int, http.Header) error // A function that processes the request and sets the field values
		data        any                                  // Response data
		statusCode  int                                  // Response status code
		headers     http.Header                          // Response headers
	}

	// client struct holds the necessary data for the client
	client struct {
		client      *http.Client         // http.Client
		baseUrl     string               // Base URL
		logf        func(string, ...any) // Logger function
		debug       bool                 // Debug mode
		headers     http.Header          // Headers
		queryParams url.Values           // Query parameters
	}

	// ErrorResponse struct holds the necessary data when an error response is received.
	// A response is considered to be an error when the status code is not between 200 and 299 (inclusive).
	ErrorResponse struct {
		Response   []byte      // Response content
		StatusCode int         // Status code
		Headers    http.Header // Response headers
	}
)

// Client  --------------------------------------------------------------------

// NewClient creates a new client with the given options
func NewClient(opts ...option) *client {
	c := client{}
	c.defaults()
	SetOptions(&c, opts...)
	return &c
}

// defaults sets the default values for the client
func (c *client) defaults() {
	c.client = &http.Client{}
	c.logf = log.Printf
}

// BaseUrl returns the base URL set in the client
func (c *client) BaseUrl() string {
	return c.baseUrl
}

// Timeout returns the timeout set in the client
func (c *client) Timeout() time.Duration {
	return c.client.Timeout
}

// Debug returns if the client is in debug mode
func (c *client) Debug() bool {
	return c.debug
}

// Headers returns the headers set in the client
func (c *client) Headers() http.Header {
	return c.headers
}

// QueryParams returns the query parameters set in the client
func (c *client) QueryParams() url.Values {
	return c.queryParams
}

// Client returns the http.Client used by the client
func (c *client) Client() *http.Client {
	return c.client
}

// Request performs a request with the given request and options and sets the response with the result. Upon an error a non nil error is returned.
// A response with a status code that is not between 200 and 299 (inclusive) also considered as an error.
func (c *client) Request(req *request, res *response, opts ...requestOption) error {
	start := time.Now()

	// Set options
	options := initRequestOptions(opts...)

	// Create request URL
	requestUrl := fmt.Sprintf("%s/%s", strings.TrimRight(c.baseUrl, "/"), strings.TrimLeft(req.Path, "/"))
	if c.debug {
		c.logf("[REQUEST] %s: %s\n", req.Method, requestUrl)
	}

	defer func() {
		c.logf("%s %s %s", req.Method, requestUrl, time.Since(start))
	}()

	// Create body
	var reqBody io.Reader
	if req.data != nil && len(req.data) > 0 {
		reqBody = bytes.NewBuffer(req.data)
		if c.debug {
			c.logf("[REQUEST BODY] %s\n", reqBody)
		}
	}

	// Create request
	request, err := http.NewRequest(string(req.Method), requestUrl, reqBody)
	if err != nil {
		return err
	}

	// Set headers
	for k, v := range c.headers {
		for _, v2 := range v {
			request.Header.Set(k, v2)
		}
	}

	for k, v := range req.Headers {
		for _, v2 := range v {
			if options.overwriteHeaders {
				request.Header.Set(k, v2)
			} else {
				request.Header.Add(k, v2)
			}
		}
	}

	// Set query parameters
	query := request.URL.Query()
	for k, v := range c.queryParams {
		for _, v2 := range v {
			query.Add(k, v2)
		}
	}

	for k, v := range req.QueryParams {
		for _, v2 := range v {
			if options.overwriteQueryParams {
				query.Set(k, v2)
			} else {
				query.Add(k, v2)
			}
		}
	}

	request.URL.RawQuery = query.Encode()

	if c.debug {
		c.logf("[REQUEST PARAMS] %q\n", request.URL.Query())
		dump, _ := httputil.DumpRequestOut(request, true)
		c.logf("[REQUEST] %s\n", dump)
	}

	// Do the request
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if c.debug {
		dump, _ := httputil.DumpResponse(response, true)
		c.logf("[RESPONSE] %s\n", dump)
	}

	// Read response body
	resBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Process gzipped response if it was set in the options
	if options.gzip {
		gr, err := gzip.NewReader(bytes.NewBuffer(resBody))
		if err != nil {
			return err
		}
		defer gr.Close()

		resBody, err = ioutil.ReadAll(gr)
		if err != nil {
			return err
		}
	}

	// Return error if a bad status code is returned
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return ErrorResponse{
			Response:   resBody,
			StatusCode: response.StatusCode,
			Headers:    response.Header,
		}
	}

	// Process and set response
	return res.processResp(resBody, response.StatusCode, response.Header)
}

// Request --------------------------------------------------------------------

// NewEmptyRequest creates a new empty request.
func NewEmptyRequest() *request {
	return &request{}
}

// NewRequest creates a new raw request
func NewRequest(data []byte) *request {
	return &request{
		data: data,
	}
}

// NewJsonRequest creates a new json request
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

// NewFormRequest creates a new form request
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

// Response -------------------------------------------------------------------

// Data returns the parsed response data
func (r *response) Data() any {
	return r.data
}

// Headers returns the response headers
func (r *response) Headers() http.Header {
	return r.headers
}

// StatusCode returns the status code of the response
func (r *response) StatusCode() int {
	return r.statusCode
}

// NewResponse creates a new raw response
func NewResponse() *response {
	r := response{}
	r.data = make([]byte, 0)
	r.processResp = func(res []byte, statusCode int, headers http.Header) error {
		r.data = res
		r.statusCode = statusCode
		r.headers = headers
		return nil
	}

	return &r
}

// NewJsonResponse creates a new json response
func NewJsonResponse(data any) *response {
	r := response{}
	r.data = data
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

// ErrorResponse --------------------------------------------------------------

// Error implements the error interface
func (e ErrorResponse) Error() string {
	return fmt.Sprintf("status code: %v\nresponse body: %s\n", e.StatusCode, e.Response)
}

// Options --------------------------------------------------------------------

// SetOptions sets options on the client
func SetOptions(c *client, opts ...option) {
	for _, optionf := range opts {
		optionf(c)
	}
}

// SetOptionsStruct sets options by the given Options struct
func SetOptionsStruct(o Options) option {
	return func(c *client) {
		f := o.Logf
		if f == nil {
			f = log.Printf
		}
		c.logf = f

		cl := o.Client
		if cl == nil {
			cl = &http.Client{}
		}
		c.client = cl

		if o.Timeout > 0 {
			c.client.Timeout = o.Timeout
		}

		c.debug = o.Debug
		c.baseUrl = o.BaseUrl
		c.headers = o.Headers
		c.queryParams = o.QueryParams
	}
}

// BaseUrl sets the base URL to the given url
func BaseUrl(url string) option {
	return func(c *client) {
		c.baseUrl = url
	}
}

// Timeout sets the timeout to the given duration
func Timeout(d time.Duration) option {
	return func(c *client) {
		c.client.Timeout = d
	}
}

// Logf sets the logging function to the given function
func Logf(f func(string, ...any)) option {
	return func(c *client) {

		// Ensure the function is set
		if f == nil {
			f = log.Printf
		}
		c.logf = f
	}
}

// Debug sets the debug mode
func Debug(on bool) option {
	return func(c *client) {
		c.debug = on
	}
}

// Client sets the client
func Client(cl *http.Client) option {
	return func(c *client) {
		c.client = cl
	}
}

// Header sets the headers
func Header(headers http.Header) option {
	return func(c *client) {
		c.headers = headers
	}
}

// SetQueryParams sets the query parameters
func SetQueryParams(queryParams url.Values) option {
	return func(c *client) {
		c.queryParams = queryParams
	}
}

// Request Options ------------------------------------------------------------

// initRequestOptions initializes the request options
func initRequestOptions(opts ...requestOption) *requestOptions {
	r := requestOptions{}
	for _, optionf := range opts {
		optionf(&r)
	}

	return &r
}

// Gzip turns on the gzip processing for the request
func Gzip() requestOption {
	return func(r *requestOptions) {
		r.gzip = true
	}
}

// OverWriteHeaders will use http.Header.Set overwriting all existing values
func OverWriteHeaders() requestOption {
	return func(r *requestOptions) {
		r.overwriteHeaders = true
	}
}

// OverWriteQueryParams will use url.Values.Set overwriting all existing values
func OverWriteQueryParams() requestOption {
	return func(r *requestOptions) {
		r.overwriteQueryParams = true
	}
}

// Helpers --------------------------------------------------------------------

// createUrlValues will create url.Values from any map or struct.
// If any other type is given, then an error is returned.
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
			values.Add(fmt.Sprint(iter.Key()), fmt.Sprint(iter.Value()))
		}
		return values, nil
	default:
		return nil, errors.New("data must be type of struct or map")
	}
}

// structFieldsToMap creates a map from any given struct.
// The struct fields must be exported and tagged with a `form:"fieldName"` tag.
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
