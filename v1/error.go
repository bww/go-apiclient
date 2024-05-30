package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

var (
	ErrUnsupportedMimetype       = errors.New("Unsupported content type")
	ErrUnexpectedStatusCode      = errors.New("Unexpected status code")
	ErrCouldNotAuthorize         = errors.New("Could not authorize request")
	ErrCouldNotUnmarshalResponse = errors.New("Could not unmarshal response")
)

var RecoverableStatuses = []int{
	http.StatusInternalServerError,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}

func wrapErr(err, base error) error {
	return wrappedErr{
		Err:  err,
		Base: base,
	}
}

type wrappedErr struct {
	Err, Base error
}

func (e wrappedErr) Error() string {
	return e.Err.Error()
}

func (e wrappedErr) Unwrap() error {
	return e.Base
}

func isSuccess(status int) bool {
	return status >= 200 && status < 300
}

func checkErr(reqid int64, req *http.Request, rsp *http.Response) error {
	if !isSuccess(rsp.StatusCode) {
		return Errorf(rsp.StatusCode, "Unexpected status code: %d %s", rsp.StatusCode, http.StatusText(rsp.StatusCode)).SetId(reqid).SetRequest(req).SetEntityFromResponse(rsp)
	}
	return nil
}

type Error struct {
	ReqId   int64
	Status  int
	Method  string
	URL     string
	Entity  *Entity
	Message string
	Cause   error
}

func Errorf(s int, f string, a ...interface{}) *Error {
	return &Error{
		Status:  s,
		Message: fmt.Sprintf(f, a...),
	}
}

func (e *Error) SetId(id int64) *Error {
	e.ReqId = id
	return e
}

func (e *Error) SetRequest(req *http.Request) *Error {
	e.Method = req.Method
	e.URL = req.URL.String()
	return e
}

func (e *Error) SetEntity(ent *Entity) *Error {
	e.Entity = ent
	return e
}

func (e *Error) SetEntityFromResponse(rsp *http.Response) *Error {
	data, err := ioutil.ReadAll(rsp.Body)
	if err == nil {
		e.SetEntity(&Entity{
			ContentType: rsp.Header.Get("Content-Type"),
			Data:        data,
		})
	}
	return e
}

func (e *Error) SetCause(err error) *Error {
	e.Cause = err
	return e
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func (e *Error) Error() string {
	b := fmt.Sprintf("%s %s: %s", e.Method, e.URL, e.Message)
	if c := e.Cause; c != nil {
		b += fmt.Sprintf("; because: %s", c.Error())
	}
	if x := e.Entity; x != nil {
		b += "\n" + x.String()
	}
	return b
}

func (e *Error) Redacted() error {
	return encodableError{
		Method:  e.Method,
		URL:     e.URL,
		Message: e.Message,
	}
}

type encodableError struct {
	Method  string
	URL     string
	Message string
}

func (e encodableError) Error() string {
	return fmt.Sprintf("%s %s: %v", e.Method, e.URL, e.Message)
}

func (e encodableError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"request": fmt.Sprintf("%s %s", e.Method, e.URL),
		"message": e.Message,
	})
}
