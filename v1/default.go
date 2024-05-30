package api

import (
	"context"
	"net/http"

	"github.com/bww/go-util/v1/errors"
)

// A convenience for one-off requests
var defaultClient = &Client{
	Client: sharedClient,
	dctype: JSON,
	header: http.Header{
		http.CanonicalHeaderKey("Content-Type"): []string{JSON},
		http.CanonicalHeaderKey("Accept"):       []string{JSON},
	},
	debug: errors.Must(Debug{}.WithEnv()),
}

// A convenience for Exec with a GET request
func Get(cxt context.Context, u string, entity interface{}) (*http.Response, error) {
	return defaultClient.Get(cxt, u, entity)
}

// A convenience for Exec with a POST request
func Post(cxt context.Context, u string, input, output interface{}) (*http.Response, error) {
	return defaultClient.Post(cxt, u, input, output)
}

// A convenience for Exec with a PUT request
func Put(cxt context.Context, u string, input, output interface{}) (*http.Response, error) {
	return defaultClient.Put(cxt, u, input, output)
}

// A convenience for Exec with a DELETE request
func Delete(cxt context.Context, u string, input, output interface{}) (*http.Response, error) {
	return defaultClient.Delete(cxt, u, input, output)
}
