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
	failure    []ErrorObserver
}

func (o *Observers) Add(add interface{}) {
	o.observers = append(o.observers, add)
	if c, ok := add.(PreflightObserver); ok {
		o.preflight = append(o.preflight, c)
	}
	if c, ok := add.(PostflightObserver); ok {
		o.postflight = append(o.postflight, c)
	}
	if c, ok := add.(ErrorObserver); ok {
		o.failure = append(o.failure, c)
	}
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

func (o *Observers) RequestFailedWithError(req *http.Request, rsp *http.Response, err error) error {
	if o == nil {
		return nil
	}
	for _, obs := range o.failure {
		err := obs.RequestFailedWithError(req, rsp, err)
		if err != nil {
			return err
		}
	}
	return nil
}
