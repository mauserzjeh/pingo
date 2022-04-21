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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// Helpers --------------------------------------------------------------------

const (
	BASE_URL = "test_base_url"
	FOO      = "foo"
	BAR      = "bar"
)

// assertEqual fails if the two values are not equal
func assertEqual[T comparable](t testing.TB, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got: %v != want: %v", got, want)
	}
}

// assertNotEqual fails if the two values are equal
func assertNotEqual[T comparable](t testing.TB, got, want T) {
	t.Helper()
	if got == want {
		t.Errorf("didn't want %v", got)
	}
}

// testServer creates a test server
func testServer() (mux *http.ServeMux, server *httptest.Server, shutdown func()) {
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)
	shutdown = func() {
		server.Close()
	}
	return mux, server, shutdown
}

// jsonData returns a byte slice of JSON data
func jsonData(data any) []byte {
	var rData []byte
	rData, _ = json.Marshal(data)
	return rData
}

// Tests ----------------------------------------------------------------------
func TestNewDefaultClient(t *testing.T) {
	c := NewClient()
	assertEqual(t, c.Client() != nil, true)
	assertEqual(t, c.BaseUrl(), "")
	assertEqual(t, c.Timeout(), 0)
	assertEqual(t, c.logf != nil, true)
	assertEqual(t, c.Debug(), false)
	assertEqual(t, len(c.Headers()), 0)
	assertEqual(t, len(c.QueryParams()), 0)
}

func TestNewClientWithOptions(t *testing.T) {
	h := http.Header{}
	h.Add(FOO, BAR)

	q := url.Values{}
	q.Add(BAR, FOO)

	o := Options{
		BaseUrl:     BASE_URL,
		Timeout:     10 * time.Second,
		Debug:       true,
		Headers:     h,
		QueryParams: q,
	}
	c1 := NewClient(SetOptionsStruct(o))
	assertEqual(t, c1.Client() != nil, true)
	assertEqual(t, c1.BaseUrl(), BASE_URL)
	assertEqual(t, c1.Timeout(), 10*time.Second)
	assertEqual(t, c1.logf != nil, true)
	assertEqual(t, c1.Debug(), true)
	assertEqual(t, len(c1.Headers()), 1)
	assertEqual(t, len(c1.QueryParams()), 1)
	assertEqual(t, c1.Headers().Get(FOO), BAR)
	assertEqual(t, c1.QueryParams().Get(BAR), FOO)

	c2 := NewClient(
		BaseUrl(BASE_URL),
		Debug(true),
		Client(&http.Client{
			Timeout: 5 * time.Second,
		}),
		Header(h),
		SetQueryParams(q),
	)

	assertEqual(t, c2.Client() != nil, true)
	assertEqual(t, c2.BaseUrl(), BASE_URL)
	assertEqual(t, c2.Timeout(), 5*time.Second)
	assertEqual(t, c2.logf != nil, true)
	assertEqual(t, c2.Debug(), true)
	assertEqual(t, len(c2.Headers()), 1)
	assertEqual(t, len(c2.QueryParams()), 1)
	assertEqual(t, c2.Headers().Get(FOO), BAR)
	assertEqual(t, c2.QueryParams().Get(BAR), FOO)
}

func TestSetOptions(t *testing.T) {
	c := NewClient()
	SetOptions(c,
		BaseUrl(BASE_URL),
		Logf(nil),
		Debug(false),
		Timeout(10*time.Second),
	)

	assertEqual(t, c.Client() != nil, true)
	assertEqual(t, c.BaseUrl(), BASE_URL)
	assertEqual(t, c.Timeout(), 10*time.Second)
	assertEqual(t, c.logf != nil, true)
	assertEqual(t, c.Debug(), false)
	assertEqual(t, len(c.Headers()), 0)
	assertEqual(t, len(c.QueryParams()), 0)
}

func TestEmptyRequest(t *testing.T) {
	// initial test setup
	mux, server, shutdown := testServer()
	defer shutdown()

	type data struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}

	d := data{
		Foo: BAR,
		Bar: FOO,
	}

	path := "/test-empty-request"
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.Header().Set(FOO, BAR)
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData(d))
	})

	// create client
	c := NewClient(
		BaseUrl(server.URL),
	)

	// create request
	req := NewEmptyRequest()
	req.Method = GET
	req.Path = path

	// json response
	res := NewJsonResponse(&data{})

	err := c.Request(req, res)
	assertEqual(t, err == nil, true)
	assertEqual(t, *(res.Data().(*data)) == d, true)
	assertEqual(t, res.StatusCode(), http.StatusOK)
	assertEqual(t, len(res.Headers()) > 0, true)
	assertEqual(t, res.Headers().Get("content-type"), "application/json")
	assertEqual(t, res.Headers().Get(FOO), BAR)

	// raw response
	resRaw := NewResponse()
	err = c.Request(req, resRaw)
	assertEqual(t, string(resRaw.Data().([]byte)) == `{"foo":"bar","bar":"foo"}`, true)
}

func TestRequest(t *testing.T) {
	// initial test setup
	mux, server, shutdown := testServer()
	defer shutdown()

	path := "/test-raw-request"
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	// create client
	c := NewClient(
		BaseUrl(server.URL),
	)

	data := []byte{1}

	// create request
	req := NewRequest(data)
	req.Method = POST
	req.Path = path

	// response
	res := NewResponse()

	err := c.Request(req, res)
	assertEqual(t, err == nil, true)
	assertEqual(t, len(res.Data().([]byte)) == len(data), true)
	assertEqual(t, res.Data().([]byte)[0] == data[0], true)
}

func TestJsonRequest(t *testing.T) {
	// initial test setup
	mux, server, shutdown := testServer()
	defer shutdown()

	type data struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}

	path := "/test-json-request"
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		var rd data
		err := json.NewDecoder(r.Body).Decode(&rd)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		rd.Foo = FOO
		rd.Bar = BAR

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData(rd))
	})

	// create client
	c := NewClient(
		BaseUrl(server.URL),
	)

	// create request
	req, err := NewJsonRequest(data{
		Foo: BAR,
		Bar: FOO,
	})
	assertEqual(t, err == nil, true)
	assertEqual(t, req.Headers.Get("content-type"), "application/json")

	req.Method = POST
	req.Path = path

	// response
	res := NewJsonResponse(&data{})

	err = c.Request(req, res)
	assertEqual(t, err == nil, true)

	resData := res.Data().(*data)
	assertEqual(t, resData.Foo, FOO)
	assertEqual(t, resData.Bar, BAR)
}

func TestFormRequest(t *testing.T) {
	// initial test setup
	mux, server, shutdown := testServer()
	defer shutdown()

	type data struct {
		Foo string `json:"foo" form:"foo"`
		Bar string `json:"bar" form:"bar"`
	}

	path := "/test-form-request"
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		foo := r.FormValue("foo")
		bar := r.FormValue("bar")

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData(data{
			Foo: bar,
			Bar: foo,
		}))
	})

	// create client
	c := NewClient(
		BaseUrl(server.URL),
		Debug(true),
		Logf(func(s string, a ...any) {
		}),
	)

	t.Run("FormRequestStruct", func(t *testing.T) {
		// create request
		req, err := NewFormRequest(data{
			Foo: BAR,
			Bar: FOO,
		})

		assertEqual(t, err == nil, true)
		assertEqual(t, req.Headers.Get("content-type"), "application/x-www-form-urlencoded")

		req.Method = POST
		req.Path = path

		// response
		res := NewJsonResponse(&data{})

		err = c.Request(req, res)
		assertEqual(t, err == nil, true)

		resData := res.Data().(*data)
		assertEqual(t, resData.Foo, FOO)
		assertEqual(t, resData.Bar, BAR)
	})

	t.Run("FormRequestMap", func(t *testing.T) {
		// create request
		req, err := NewFormRequest(map[string]any{
			"foo": BAR,
			"bar": FOO,
		})

		assertEqual(t, err == nil, true)
		assertEqual(t, req.Headers.Get("content-type"), "application/x-www-form-urlencoded")

		req.Method = POST
		req.Path = path

		// response
		res := NewJsonResponse(&data{})

		err = c.Request(req, res)
		assertEqual(t, err == nil, true)

		resData := res.Data().(*data)
		assertEqual(t, resData.Foo, FOO)
		assertEqual(t, resData.Bar, BAR)
	})

	t.Run("FormRequestError", func(t *testing.T) {
		// create request
		req, err := NewFormRequest(1)
		assertEqual(t, req, nil)
		assertEqual(t, err != nil, true)
	})

}

func TestErrorResponse(t *testing.T) {
	// initial test setup
	mux, server, shutdown := testServer()
	defer shutdown()

	// error string
	es := `oops an error happened!`

	path := "/test-error-response"
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("error", "error")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(es))
	})

	// create client
	c := NewClient(
		BaseUrl(server.URL),
	)

	// create request
	req := NewEmptyRequest()
	req.Method = GET
	req.Path = path

	res := NewResponse()
	err := c.Request(req, res)
	assertEqual(t, err != nil, true)

	ce := err.(ErrorResponse)
	assertEqual(t, ce.Error(), "status code: 400, response body: oops an error happened!")
	assertEqual(t, ce.Headers.Get("error"), "error")
	assertEqual(t, ce.StatusCode, http.StatusBadRequest)
	assertEqual(t, string(ce.Response), es)
}

func TestRequestOptions(t *testing.T) {
	// initial test setup
	mux, server, shutdown := testServer()
	defer shutdown()

	gs := "this is a gzipped content"

	path := "/test-request-options"
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, r.Header.Get(FOO), FOO)
		assertEqual(t, r.FormValue(BAR), BAR)

		var b bytes.Buffer
		gw := gzip.NewWriter(&b)
		gw.Write([]byte(gs))
		gw.Close()

		w.WriteHeader(http.StatusOK)
		w.Write(b.Bytes())
	})

	c := NewClient(
		BaseUrl(server.URL),
		Header(http.Header{
			FOO: {"this value will get overwritten by request options"},
		}),
		SetQueryParams(url.Values{
			BAR: {"this value will get overwritten by request options"},
		}),
	)

	// request
	req := NewEmptyRequest()
	req.Method = GET
	req.Path = path
	req.Headers.Set(FOO, FOO)
	req.QueryParams.Set(BAR, BAR)

	// response
	res := NewResponse()

	err := c.Request(req, res,
		Gzip(),
		OverWriteHeaders(),
		OverWriteQueryParams(),
	)
	assertEqual(t, err == nil, true)
	assertEqual(t, res.StatusCode(), http.StatusOK)
	assertEqual(t, string(res.Data().([]byte)), gs)
}

func TestComplex(t *testing.T) {

}
