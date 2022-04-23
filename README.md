![GitHub release (latest by date)](https://img.shields.io/github/v/release/mauserzjeh/pingo?style=flat-square)

# Pingo
Pingo is a general purpose, high level API client library implemented in Go. It is built on top of the standard net/http package and aims to make sending requests and processing the responses much more convenient.

# Features
- Zero dependencies
- Create a client with multiple options
- Set additional options for requests
- Send raw, json or form requests
- Receive raw or json responses
- Client will handle data marshalling and unmarshaling

# Installation
```
go get -u github.com/mauserzjeh/controller
```

# Tests
```
go test -v
```

# Examples and usage

A working example.

```go
package main

import (
	"log"
	"time"

	"github.com/mauserzjeh/pingo"
)

type CreateUser struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

type UserResponse struct {
	Name      string    `json:"name"`
	Job       string    `json:"job"`
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

func main() {

	// create client
	client := pingo.NewClient(
		pingo.BaseUrl("https://reqres.in/api"),
		pingo.Timeout(10*time.Second),
	)

	// create a request
	req, err := pingo.NewJsonRequest(CreateUser{
		Name: "Pingo",
		Job:  "Developer",
	})

	if err != nil {
		log.Fatal(err)
	}

	// set method and path
	req.Method = pingo.POST
	req.Path = "/users"

	// create a response
	res := pingo.NewJsonResponse(&UserResponse{})

	// make request
	err = client.Request(req, res)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Status code: %+v\n", res.StatusCode())
	log.Printf("Headers: %+v\n", res.Headers())
	log.Printf("Response: %+v\n", res.Data().(*UserResponse))
}
```

<hr>

## Client options
```go
// Client options can be set by initializing a `Option` struct or by using option functions exposed by the package.
Options struct {
    BaseUrl     string               // Base URL for the client
    Timeout     time.Duration        // Client timeout
    Logf        func(string, ...any) // Logger function
    Debug       bool                 // Debug mode
    Client      *http.Client         // http.Client that the client will use
    Headers     http.Header          // Client headers
    QueryParams url.Values           // Client query parameters
}
```
```go
// Client options can also be changed later by the exposed `SetOptions` function.
pingo.SetOptions(client, 
    pingo.BaseUrl("http://new-base-url.tld"),
    pingo.Logf(log.Printf),
    pingo.Debug(true),
    pingo.Timeout(15*time.Second)
)
```

| Option function  | Description                                                 |
|------------------|-------------------------------------------------------------|
| SetOptionsStruct | Set options via `Option` struct                             |
| BaseUrl          | Set the base URL                                            |
| Timeout          | Set timeout                                                 |
| Logf             | Set a logging function                                      |
| Debug            | Set debug mode                                              |
| Client           | Set a custom http.Client                                    |
| Headers          | Set headers that will be included in every request          |
| QueryParams      | Set query parameters that will be included in every request |

<hr>

## Request object
```go
// Create a request object with no body
req := NewEmptysRequest()
```

```go
// Create a request with a body from a byte slice
req := NewRequest([]byte(`this is the request body`))
```

```go
// Create a json request with a body from any data.
// Data can be any type of variable or a struct with proper `json:"fieldname"` tags.
// An error is returned if the marshaling of the data resulted in an error.
// "Content-Type: application/json" header is added to the request.
req, err := NewJsonRequest(data)
```

```go
// Create a form request with a body from any data.
// Data can be any type of map or a struct with proper `form:"fieldname"` tags.
// An error is returned if the marshaling of the data resulted in an error.
// "Content-Type: application/x-www-form-urlencoded" header is added to the request
req, err := NewFormRequest(data)
```

```go
// After creating the request, additional options can be set.
req.Method = pingo.POST // GET by default
req.Path = "/path/to/the/endpoint" // Path to the endpoint
req.Headers.Add("X-Custom-Header", "FooBar") // If necessary additional headers can be added or set
req.QueryParams.Set("Foo", "Bar") // If necessary addtional query parameters can be added or set

```

<hr>

## Response object
```go
// Create a response object
res := NewResponse()
```

```go
type MyData struct {
    Field1 string `json:"field1"`
    Field2 int64 `json:"field2"`
}

// Create a json response object with a certain type of data
// When a request is made and a json response is supplied, then the library
// will try to unmarshal the response into the given data type.
res := NewJsonResponse(&MyData{})
```
After a request mas made then response data can be accessed by the following methods.
```go
// Access response headers
headers := res.Headers()

// Access response status code
statusCode := res.StatusCode()

// Access response data
data := res.Data()

// If the response was created with NewResponse the returned data will be a byte slice. If the response was created with NewJsonResponse the resuturned data will be the same type as the parameter that was given to NewJsonResponse
byteData := data.([]byte) // NewResponse
myData := data.(*MyData) // NewJsonResponse
```

## Make requests
```go
// A request can be made by giving a request and a response object to the client
err := client.Request(req, res)
```

```go
// Additional request options can be set by passing request option functions to the request
err := client.Request(req, res, 
    pingo.Gzip(),
    pingo.OverWriteHeaders(),
)

// Access response data
headers := res.Headers()
statusCode := res.StatusCode()
data := res.Data()
```
| Option function      | Description                                                                         |
|----------------------|-------------------------------------------------------------------------------------|
| Gzip                 | Turns on the gzip processing of the response                                        |
| OverWriteHeaders     | Headers set in the request will overwrite existing client headers                   |
| OverWriteQueryParams | Query parameters set in the request will overwrite existing client query parameters |

## Error handling
```go
// Make request
err := client.Request(req, res)

// If the request was not successful then a non nil error is returned.
// Errors can have multiple causes e.g.: invalid or missing options, but also a response with
// the status code that is not between 200 and 299 (inclusive) is also considered as an error.
// In this case the package exposes a `ResponseError` struct.
if resErr, ok := err.(ResponseError); ok {
    headers := resErr.Headers // Access response headers
    statusCode := resErr.StatusCode // Access response status code
    body := resErr.Response // Access response body
}
```

