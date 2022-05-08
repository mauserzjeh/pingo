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
	"fmt"
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
	API_KEY  = "your_api_key"
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
		Headers(h),
		QueryParams(q),
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

	path1 := "/test-json-request"
	mux.HandleFunc(path1, func(w http.ResponseWriter, r *http.Request) {
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

	path2 := "/test-json-request-wrong"
	mux.HandleFunc(path2, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`this is clearly not json`))
	})

	// create client
	c := NewClient(
		BaseUrl(server.URL),
	)

	t.Run("JsonRequestOkJsonResponse", func(t *testing.T) {
		// create request
		req, err := NewJsonRequest(data{
			Foo: BAR,
			Bar: FOO,
		})
		assertEqual(t, err == nil, true)
		assertEqual(t, req.Headers.Get("content-type"), "application/json")

		req.Method = POST
		req.Path = path1

		// response
		res := NewJsonResponse(&data{})

		err = c.Request(req, res)
		assertEqual(t, err == nil, true)

		resData := res.Data().(*data)
		assertEqual(t, resData.Foo, FOO)
		assertEqual(t, resData.Bar, BAR)
	})

	t.Run("JsonRequestWrongJsonResponse", func(t *testing.T) {
		// create request
		req := NewEmptyRequest()
		req.Path = path2

		// response
		res := NewJsonResponse(&data{})

		err := c.Request(req, res)
		assertEqual(t, err != nil, true)
	})

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
	e1 := `oops an error happened!`

	path1 := "/test-error-response"
	mux.HandleFunc(path1, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("error", "error")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(e1))
	})

	type errResp struct {
		This  string `json:"this"`
		Is    string `json:"is"`
		Error string `json:"error"`
		Num   int    `json:"num"`
	}

	// error object
	e2 := errResp{
		This:  "this",
		Is:    "is",
		Error: "error",
		Num:   12345,
	}

	path2 := "/test-error-response-custom-error-ok"
	mux.HandleFunc(path2, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("error", "error")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData(e2))
	})

	// create client
	c := NewClient(
		BaseUrl(server.URL),
	)

	t.Run("ErrorResponseRaw", func(t *testing.T) {
		// create request
		req := NewEmptyRequest()
		req.Path = path1

		res := NewResponse()
		err := c.Request(req, res)
		assertEqual(t, err != nil, true)

		re := err.(ResponseError)
		assertEqual(t, re.Error(), fmt.Sprintf("status code: 400, response: %s", e1))
		assertEqual(t, re.Headers().Get("error"), "error")
		assertEqual(t, re.StatusCode(), http.StatusBadRequest)
		assertEqual(t, string(re.Data().([]byte)), e1)
	})

	t.Run("ErrorResponseCustomErrorOk", func(t *testing.T) {
		// create request
		req := NewEmptyRequest()
		req.Path = path2

		res := NewResponse()
		err := c.Request(req, res, CustomError(&errResp{}))
		assertEqual(t, err != nil, true)

		re := err.(ResponseError)
		assertEqual(t, re.Error(), fmt.Sprintf(`status code: 400, response: %+v`, &e2))
		assertEqual(t, re.Headers().Get("error"), "error")
		assertEqual(t, re.StatusCode(), http.StatusBadRequest)

		d, ok := re.Data().(*errResp)
		assertEqual(t, ok, true)
		assertEqual(t, d.This, "this")
		assertEqual(t, d.Is, "is")
		assertEqual(t, d.Error, "error")
		assertEqual(t, d.Num, 12345)
	})

	t.Run("ErrorResponseCustomErrorWrong", func(t *testing.T) {
		// create request
		req := NewEmptyRequest()
		req.Path = path1

		res := NewResponse()
		err := c.Request(req, res, CustomError(&errResp{}))
		assertEqual(t, err != nil, true)

		re := err.(ResponseError)
		assertEqual(t, re.Error(), fmt.Sprintf(`status code: 400, response: %s`, e1))
		assertEqual(t, re.Headers().Get("error"), "error")
		assertEqual(t, re.StatusCode(), http.StatusBadRequest)

		_, ok := re.Data().(*errResp)
		assertEqual(t, ok, false)

		_, ok2 := re.Data().([]byte)
		assertEqual(t, ok2, true)
	})

	t.Run("ErrorResponseCustomErrorNil", func(t *testing.T) {
		// create request
		req := NewEmptyRequest()
		req.Path = path1

		res := NewResponse()
		err := c.Request(req, res, CustomError(nil))
		assertEqual(t, err != nil, true)

		re := err.(ResponseError)
		assertEqual(t, re.Error(), fmt.Sprintf(`status code: 400, response: %s`, e1))
		assertEqual(t, re.Headers().Get("error"), "error")
		assertEqual(t, re.StatusCode(), http.StatusBadRequest)

		_, ok := re.Data().(*errResp)
		assertEqual(t, ok, false)

		_, ok2 := re.Data().([]byte)
		assertEqual(t, ok2, true)
	})
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
		Headers(http.Header{
			FOO: {"this value will get overwritten by request options"},
		}),
		QueryParams(url.Values{
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

func TestCustomResponseParse(t *testing.T) {
	// initial test setup
	mux, server, shutdown := testServer()
	defer shutdown()

	// ----------------------------------------------------------------------------

	type good struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	pathGood := "/good"
	mux.HandleFunc(pathGood, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData(good{
			Success: true,
			Message: "ok",
		}))
	})

	type bad struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}

	pathBad := "/bad"
	mux.HandleFunc(pathBad, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData(bad{
			Success: false,
			Error:   "not ok",
		}))
	})

	// ----------------------------------------------------------------------------

	// client
	c := NewClient(
		BaseUrl(server.URL),
	)

	// custom handler function
	handler := func(res []byte, statusCode int, headers http.Header) (any, error) {
		if statusCode == 200 {
			g := &good{}
			err := json.Unmarshal(res, g)
			return g, err
		} else {
			b := &bad{}
			err := json.Unmarshal(res, b)
			return b, err
		}
	}

	// response
	res := NewCustomResponse(handler)

	// good request
	reqGood := NewEmptyRequest()
	reqGood.Path = pathGood
	err := c.Request(reqGood, res)
	assertEqual(t, err == nil, true)
	assertEqual(t, res.StatusCode(), http.StatusOK)
	resGoodData, ok := res.Data().(*good)
	assertEqual(t, ok, true)
	assertEqual(t, resGoodData.Success, true)
	assertEqual(t, resGoodData.Message, "ok")

	// bad request
	reqBad := NewEmptyRequest()
	reqBad.Path = pathBad
	err = c.Request(reqBad, res)
	assertEqual(t, err == nil, true)
	assertEqual(t, res.StatusCode(), http.StatusBadRequest)
	resBadData, ok := res.Data().(*bad)
	assertEqual(t, ok, true)
	assertEqual(t, resBadData.Success, false)
	assertEqual(t, resBadData.Error, "not ok")
}

func TestComplex(t *testing.T) {
	// initial test setup
	mux, server, shutdown := testServer()
	defer shutdown()

	// ----------------------------------------------------------------------------

	type resData struct {
		Int int64    `json:"int"`
		Str string   `json:"str"`
		Slc []string `json:"slc"`
	}

	path1 := "/get-data"
	mux.HandleFunc(path1, func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-KEY")
		if apiKey != API_KEY {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		assertEqual(t, apiKey, API_KEY)

		var b bytes.Buffer
		gw := gzip.NewWriter(&b)
		gw.Write(jsonData(resData{
			Int: 10,
			Str: "test",
			Slc: []string{"a", "b", "c"},
		}))
		gw.Close()

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b.Bytes())
	})

	// ----------------------------------------------------------------------------

	path2 := "/post-data"
	mux.HandleFunc(path2, func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-KEY")
		if apiKey != API_KEY {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		assertEqual(t, apiKey, API_KEY)

		a := r.FormValue("A")
		b := r.FormValue("B")
		c := r.FormValue("C")
		i := r.FormValue("int")
		s := r.FormValue("str")
		f := r.FormValue(FOO)
		u := r.FormValue("unexported")

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(a + b + c + i + s + f + u))
	})

	// ----------------------------------------------------------------------------

	// client
	c := NewClient(
		BaseUrl(server.URL),
		Headers(http.Header{
			"X-API-KEY": {API_KEY},
		}),
	)

	// first request
	req1 := NewEmptyRequest()
	req1.Method = GET
	req1.Path = path1

	res1 := NewJsonResponse(&resData{})
	err := c.Request(req1, res1, Gzip())

	assertEqual(t, err == nil, true)
	assertEqual(t, res1.StatusCode(), http.StatusOK)

	resd := res1.Data().(*resData)
	assertEqual(t, resd.Int, 10)
	assertEqual(t, resd.Str, "test")
	assertEqual(t, len(resd.Slc), 3)
	assertEqual(t, resd.Slc[0], "a")
	assertEqual(t, resd.Slc[1], "b")
	assertEqual(t, resd.Slc[2], "c")

	// test setup for next request
	type SlcData struct {
		A string `form:"A"`
		B string `form:"B"`
		C string `form:"C"`
	}

	type fData struct {
		SlcData
		Int        int64  `form:"int"`
		Str        string `form:"str"`
		unexported string `form:"ue"`
	}

	// second request
	req2, err := NewFormRequest(fData{
		SlcData: SlcData{
			A: resd.Slc[0],
			B: resd.Slc[1],
			C: resd.Slc[2],
		},
		Int:        resd.Int,
		Str:        resd.Str,
		unexported: "unexported",
	})
	req2.Method = POST
	req2.Path = path2
	req2.QueryParams.Set(FOO, BAR)
	assertEqual(t, err == nil, true)

	res2 := NewResponse()

	err = c.Request(req2, res2)
	assertEqual(t, res2.StatusCode(), http.StatusOK)
	assertEqual(t, string(res2.Data().([]byte)), "abc10test"+BAR)
}
