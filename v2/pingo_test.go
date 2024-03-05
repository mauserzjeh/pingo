package pingo

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
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

	resp, err := NewRequest().SetBaseUrl(server.URL).SetPath("/ping").Do()
	if err != nil {
		t.Fatal(err)
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

	resp, err := NewRequest().
		SetBaseUrl(server.URL).
		SetPath("/echo").
		SetMethod(http.MethodPost).
		BodyJson(r).
		Do()

	if err != nil {
		t.Fatal(err)

	}

	assertEqual(t, resp.Status(), fmt.Sprintf("%v %s", http.StatusOK, http.StatusText(http.StatusOK)))
	assertEqual(t, resp.StatusCode(), http.StatusOK)
	assertEqual(t, resp.GetHeader(headerUserAgent), headerUserAgentDefaultValue)
	assertEqual(t, resp.GetHeader(headerContentType), ContentTypeJson)

	rr := req{}
	err = json.Unmarshal(resp.BodyRaw(), &rr)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, reflect.DeepEqual(r, rr), true)
}

func TestRawRequest(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	body := []byte("echo")

	resp, err := NewRequest().
		SetBaseUrl(server.URL).
		SetPath("/echo").
		SetMethod(http.MethodPost).
		BodyRaw(body).
		Do()

	if err != nil {
		t.Fatal(err)

	}

	assertEqual(t, resp.StatusCode(), http.StatusOK)
	assertEqual(t, resp.Headers().Get(headerUserAgent), headerUserAgentDefaultValue)
	assertEqual(t, reflect.DeepEqual(resp.BodyRaw(), body), true)
}

func TestXmlRequest(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	type req struct {
		Foo string `json:"foo" xml:"foo"`
		Bar string `json:"bar" xml:"bar"`
	}

	r := req{
		Foo: "foo",
		Bar: "bar",
	}

	resp, err := NewRequest().
		SetBaseUrl(server.URL).
		SetPath("/echo").
		SetMethod(http.MethodPost).
		BodyXml(r).
		Do()

	if err != nil {
		t.Fatal(err)
	}

	rr, err := xml.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, resp.StatusCode(), http.StatusOK)
	assertEqual(t, resp.GetHeader(headerContentType), ContentTypeXml)
	assertEqual(t, reflect.DeepEqual(resp.BodyRaw(), rr), true)
}

func TestFormUrlEncodedRequest(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	r := url.Values{}
	r.Set("foo", "bar")
	r.Set("bar", "foo")

	resp, err := NewRequest().
		SetBaseUrl(server.URL).
		SetPath("/echo").
		SetMethod(http.MethodPost).
		BodyFormUrlEncoded(r).
		Do()

	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, resp.StatusCode(), http.StatusOK)
	assertEqual(t, resp.GetHeader(headerContentType), ContentTypeFormUrlEncoded)
	assertEqual(t, resp.BodyString(), r.Encode())
}

func TestCustomRequest(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	b := "hello"
	buf := bytes.NewBuffer([]byte(b))

	resp, err := NewRequest().
		SetBaseUrl(server.URL).
		SetPath("/echo").
		SetMethod(http.MethodPost).
		BodyCustom(func() (*bytes.Buffer, error) {
			return buf, nil
		}).
		Do()

	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, resp.StatusCode(), http.StatusOK)
	assertEqual(t, resp.BodyString(), b)

	e := "yikes"
	resp, err = NewRequest().
		SetBaseUrl(server.URL).
		SetPath("/echo").
		SetMethod(http.MethodPost).
		BodyCustom(func() (*bytes.Buffer, error) {
			return nil, errors.New(e)
		}).
		Do()

	if err == nil {
		t.Fatal(err)
	}

	assertEqual(t, err.Error(), e)
	assertEqual(t, resp, nil)
}
