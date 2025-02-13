package events

import (
	"net/http"
)

type PreflightObserver interface {
	WillSendRequest(req *http.Request) error
}

type PreflightObserverFunc func(req *http.Request) error

func (o PreflightObserverFunc) WillSendRequest(req *http.Request) error {
	return o(req)
}

type PostflightObserver interface {
	DidReceiveResponse(req *http.Request, rsp *http.Response) error
}

type PostflightObserverFunc func(req *http.Request, rsp *http.Response) error

func (o PostflightObserverFunc) DidReceiveResponse(req *http.Request, rsp *http.Response) error {
	return o(req, rsp)
}

type ErrorObserver interface {
	RequestFailedWithError(req *http.Request, rsp *http.Response, err error) error
}

type ErrorObserverFunc func(req *http.Request, rsp *http.Response, err error) error

func (o ErrorObserverFunc) RequestFailedWithError(req *http.Request, rsp *http.Response, err error) error {
	return o(req, rsp, err)
}
