// The events interface provides a mechanism to observe events for an API client
// in a central place. This may be useful for logging certain types of errors in
// a uniform way or debugging a variety of operations centrally.
package events

import (
	"net/http"
)

type Observers struct {
	observers  []interface{} // all observers
	preflight  []PreflightObserver
	postflight []PostflightObserver
	failure    []FailureObserver
}

func NewObservers() *Observers {
	return &Observers{}
}

func (o *Observers) Add(adds ...interface{}) *Observers {
	for _, add := range adds {
		o.observers = append(o.observers, add)
		if c, ok := add.(PreflightObserver); ok {
			o.preflight = append(o.preflight, c)
		}
		if c, ok := add.(PostflightObserver); ok {
			o.postflight = append(o.postflight, c)
		}
		if c, ok := add.(FailureObserver); ok {
			o.failure = append(o.failure, c)
		}
	}
	return o
}

func (o *Observers) WillSendRequest(req *http.Request) error {
	if o == nil {
		return nil
	}
	for _, obs := range o.preflight {
		err := obs.WillSendRequest(req)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Observers) DidReceiveResponse(req *http.Request, rsp *http.Response) error {
	if o == nil {
		return nil
	}
	for _, obs := range o.postflight {
		err := obs.DidReceiveResponse(req, rsp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Observers) DidFailWithError(req *http.Request, err error) error {
	if o == nil {
		return nil
	}
	for _, obs := range o.failure {
		err := obs.DidFailWithError(req, err)
		if err != nil {
			return err
		}
	}
	return nil
}
