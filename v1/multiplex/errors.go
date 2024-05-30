package multiplex

import (
	"net/http"
)

type ErrorHandler interface {
	Handle(*http.Response, error) (*http.Response, error)
}

type ErrorHandlerFunc func(*http.Response, error) (*http.Response, error)

func (f ErrorHandlerFunc) Handle(rsp *http.Response, err error) (*http.Response, error) {
	return f(rsp, err)
}
