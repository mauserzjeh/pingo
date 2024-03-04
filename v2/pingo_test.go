package pingo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
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

func testServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
			return
		}

		for h, v := range r.Header {
			if len(v) > 0 {
				w.Header().Set(h, v[0])
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	server := httptest.NewServer(mux)
	return server
}

type customLogger struct{}

func (c customLogger) Debug(msg string, args ...any) {
	fmt.Printf(msg, args...)
}

func (c customLogger) Info(msg string, args ...any) {
	fmt.Printf(msg, args...)
}

func TestClientSettings(t *testing.T) {
	c := NewClient()

	cc := &http.Client{}
	c.SetClient(cc)
	assertEqual(t, c.client, cc)

	baseUrl := "foo"
	c.SetBaseUrl(baseUrl)
	assertEqual(t, c.baseUrl, baseUrl)

	hs := make(http.Header)
	hs.Set("foo", "bar")
	hs.Set(headerUserAgent, "")

	c.SetHeaders(hs)
	hs.Del(headerUserAgent)
	assertEqual(t, reflect.DeepEqual(c.headers, hs), true)

	hs.Set("foo", "foo")
	c.SetHeader("foo", "foo")
	assertEqual(t, reflect.DeepEqual(c.headers, hs), true)

	hs.Set("bar", "bar")
	c.AddHeaders(http.Header{"bar": []string{"bar"}})
	assertEqual(t, reflect.DeepEqual(c.headers, hs), true)

	hs.Add("bar", "bar2")
	c.AddHeader("bar", "bar2")
	assertEqual(t, reflect.DeepEqual(c.headers, hs), true)

	qs := make(url.Values)
	qs.Set("foo", "bar")
	qs.Set("bar", "foo")

	c.SetQueryParams(qs)
	assertEqual(t, reflect.DeepEqual(c.queryParams, qs), true)

	qs.Set("bar", "")
	c.SetQueryParams(qs)
	qs.Del("bar")
	assertEqual(t, reflect.DeepEqual(c.queryParams, qs), true)

	qs.Set("foo", "foo")
	c.SetQueryParam("foo", "foo")
	assertEqual(t, reflect.DeepEqual(c.queryParams, qs), true)

	qs.Set("bar", "bar")
	c.AddQueryParams(url.Values{"bar": []string{"bar"}})
	assertEqual(t, reflect.DeepEqual(c.queryParams, qs), true)

	qs.Add("bar", "bar2")
	c.AddQueryParam("bar", "bar2")
	assertEqual(t, reflect.DeepEqual(c.queryParams, qs), true)

	timeout := 5 * time.Second
	c.SetTimeout(timeout)
	assertEqual(t, c.timeout, timeout)

	debug := true
	c.SetDebug(debug)
	assertEqual(t, c.debug, debug)

	cl := customLogger{}
	c.SetLogger(cl)
	assertEqual(t, reflect.DeepEqual(c.logger, cl), true)
}

func TestRequestSettings(t *testing.T) {
	r := NewClient().NewRequest()

	method := http.MethodPost
	r.SetMethod(method)
	assertEqual(t, r.method, method)

	baseUrl := "foo"
	r.SetBaseUrl(baseUrl)
	assertEqual(t, r.baseUrl, baseUrl)

	path := "/bar"
	r.SetPath(path)
	assertEqual(t, r.path, path)

	hs := make(http.Header)
	hs.Set("foo", "bar")
	hs.Set(headerUserAgent, headerUserAgentDefaultValue)

	r.SetHeaders(hs)
	assertEqual(t, reflect.DeepEqual(r.headers, hs), true)

	hs.Set("foo", "foo")
	r.SetHeader("foo", "foo")
	assertEqual(t, reflect.DeepEqual(r.headers, hs), true)

	hs.Set("bar", "bar")
	r.AddHeaders(http.Header{"bar": []string{"bar"}})
	assertEqual(t, reflect.DeepEqual(r.headers, hs), true)

	hs.Add("bar", "bar2")
	r.AddHeader("bar", "bar2")
	assertEqual(t, reflect.DeepEqual(r.headers, hs), true)

	qs := make(url.Values)
	qs.Set("foo", "bar")

	r.SetQueryParams(qs)
	assertEqual(t, reflect.DeepEqual(r.queryParams, qs), true)

	qs.Set("foo", "foo")
	r.SetQueryParam("foo", "foo")
	assertEqual(t, reflect.DeepEqual(r.queryParams, qs), true)

	qs.Set("bar", "bar")
	r.AddQueryParams(url.Values{"bar": []string{"bar"}})
	assertEqual(t, reflect.DeepEqual(r.queryParams, qs), true)

	qs.Add("bar", "bar2")
	r.AddQueryParam("bar", "bar2")
	assertEqual(t, reflect.DeepEqual(r.queryParams, qs), true)

	timeout := 5 * time.Second
	r.SetTimeout(timeout)
	assertEqual(t, r.timeout, timeout)
}

func TestEmptyRequest(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	resp, err := NewClient().SetBaseUrl(server.URL).NewRequest().SetPath("/ping").Do()
	assertEqual(t, err, nil)
	assertNotEqual(t, resp, nil)
	if err != nil {
		return
	}

	assertEqual(t, resp.StatusCode(), http.StatusOK)
	assertEqual(t, resp.BodyString(), "pong")
}

func TestJsonRequest(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	type req struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}

	r := req{
		Foo: "foo",
		Bar: "bar",
	}

	resp, err := NewClient().
		SetBaseUrl(server.URL).
		NewRequest().
		SetPath("/echo").
		SetMethod(http.MethodPost).
		BodyJson(r).
		Do()

	assertNotEqual(t, resp, nil)
	assertEqual(t, err, nil)
	if err != nil {
		return
	}

	assertEqual(t, resp.Status(), fmt.Sprintf("%v %s", http.StatusOK, http.StatusText(http.StatusOK)))
	assertEqual(t, resp.StatusCode(), http.StatusOK)
	assertEqual(t, resp.GetHeader(headerUserAgent), headerUserAgentDefaultValue)

	rr := req{}
	err = json.Unmarshal(resp.BodyRaw(), &rr)
	assertEqual(t, err, nil)
	if err != nil {
		return
	}

	assertEqual(t, reflect.DeepEqual(r, rr), true)
}

func TestRawRequest(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	body := []byte("echo")

	resp, err := NewClient().
		SetBaseUrl(server.URL).
		NewRequest().
		SetPath("/echo").
		SetMethod(http.MethodPost).
		BodyRaw(body).
		Do()

	assertNotEqual(t, resp, nil)
	assertEqual(t, err, nil)
	if err != nil {
		return
	}

	assertEqual(t, resp.StatusCode(), http.StatusOK)
	assertEqual(t, resp.Headers().Get(headerUserAgent), headerUserAgentDefaultValue)
	assertEqual(t, reflect.DeepEqual(resp.BodyRaw(), body), true)
}
