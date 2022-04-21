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
	"encoding/json"
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

func TestGetRequest(t *testing.T) {
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

	path1 := "/get-test-json"
	mux.HandleFunc(path1, func(w http.ResponseWriter, r *http.Request) {
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
	req.Path = path1

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
	resRaw := NewRawResponse()
	err = c.Request(req, resRaw)
	assertEqual(t, string(resRaw.Data().([]byte)) == `{"foo":"bar","bar":"foo"}`, true)
}
