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

type FailureObserver interface {
	DidFailWithError(req *http.Request, err error) error
}

type FailureObserverFunc func(req *http.Request, err error) error

func (o FailureObserverFunc) DidFailWithError(req *http.Request, err error) error {
	return o(req, err)
}
