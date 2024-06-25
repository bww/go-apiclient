package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/bww/go-metrics/v1"
	"github.com/bww/go-ratelimit/v1"
	errutil "github.com/bww/go-util/v1/errors"
	"github.com/bww/go-util/v1/text"
	"github.com/dustin/go-humanize"
	"github.com/google/go-querystring/query"
)

var (
	requestDurationSampler = metrics.RegisterSamplerVec("rest_client_perform_request", "Perform an HTTP request", []string{"domain"})
	rateLimitDelaySampler  = metrics.RegisterSamplerVec("rest_client_rate_limit_delay", "Request delayed due to rate limiting", []string{"domain"})
	rateLimitRetrySampler  = metrics.RegisterSamplerVec("rest_client_rate_limit_retry", "Request retried due to rate limiting", []string{"domain"})
	failureRetrySampler    = metrics.RegisterSamplerVec("rest_client_failure_retry", "Request retried due to recoverable failure", []string{"domain"})
)

const (
	maxRetries     = 3
	backoffDefault = time.Minute * 3
)

var reqctr int64

const (
	JSON       = "application/json"
	URLEncoded = "application/x-www-form-urlencoded"
	Multipart  = "multipart/form-data"
	PlainText  = "text/plain"
)

// shared HTTP client
var sharedClient = &http.Client{
	Timeout: time.Second * 60,
}

// An API client
type Client struct {
	*http.Client
	auth    Authorizer
	limiter ratelimit.Limiter
	retry   map[int]struct{}
	backoff time.Duration
	base    *url.URL
	header  http.Header
	dctype  string
	debug   Debug
}

// Create a new client
func New(opts ...Option) (*Client, error) {
	return NewWithConfig(Config{
		Client: sharedClient,
	}.WithOptions(opts))
}

// Create a new client with a configuration
func NewWithConfig(conf Config) (*Client, error) {
	var err error

	var base *url.URL
	if u := conf.BaseURL; u != "" {
		base, err = url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("Invalid base URL: %v", err)
		}
	}

	var client *http.Client
	if conf.Client != nil {
		client = conf.Client
	} else if conf.Timeout > 0 {
		client = &http.Client{Timeout: conf.Timeout}
	} else {
		client = sharedClient
	}

	ctype := conf.ContentType
	if ctype == "" {
		ctype = JSON
	}

	retry := make(map[int]struct{})
	for _, e := range conf.RetryStatus {
		retry[e] = struct{}{}
	}

	debug, err := Debug{
		Debug:   conf.Debug,
		Verbose: conf.Verbose,
	}.WithEnv()
	if err != nil {
		return nil, err
	}

	return &Client{
		Client:  client,
		auth:    conf.Authorizer,
		limiter: conf.RateLimiter,
		retry:   retry,
		backoff: conf.RetryDelay,
		base:    base,
		header:  conf.Header,
		dctype:  ctype,
		debug:   debug,
	}, nil
}

func (c *Client) Base() *url.URL {
	return c.base
}

func (c *Client) WithBase(b *url.URL) *Client {
	return &Client{
		Client:  c.Client,
		auth:    c.auth,
		limiter: c.limiter,
		base:    b,
		header:  c.header,
		dctype:  c.dctype,
		debug:   c.debug,
	}
}

func (c *Client) Authorizer() Authorizer {
	return c.auth
}

func (c *Client) WithAuthorizer(a Authorizer) *Client {
	return &Client{
		Client:  c.Client,
		auth:    a,
		limiter: c.limiter,
		base:    c.base,
		header:  c.header,
		dctype:  c.dctype,
		debug:   c.debug,
	}
}

func (c *Client) isVerbose(req *http.Request) bool {
	if !c.debug.Verbose {
		return false
	}
	return c.debug.Matches(req)
}

func (c *Client) isDebug(req *http.Request) bool {
	if !c.debug.Debug {
		return false
	}
	return c.debug.Matches(req)
}

// A convenience for Exec with a GET request
func (c *Client) Get(cxt context.Context, u string, output interface{}, opts ...Option) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	return c.Exec(req.WithContext(cxt), output, opts...)
}

// A convenience for Exec with a POST request
func (c *Client) Post(cxt context.Context, u string, input, output interface{}, opts ...Option) (*http.Response, error) {
	data, err := entityReader(c.dctype, input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, err
	}
	return c.Exec(req.WithContext(cxt), output, opts...)
}

// A convenience for Exec with a PUT request
func (c *Client) Put(cxt context.Context, u string, input, output interface{}, opts ...Option) (*http.Response, error) {
	data, err := entityReader(c.dctype, input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, u, data)
	if err != nil {
		return nil, err
	}
	return c.Exec(req.WithContext(cxt), output, opts...)
}

// A convenience for Exec with a DELETE request
func (c *Client) Delete(cxt context.Context, u string, input, output interface{}, opts ...Option) (*http.Response, error) {
	data, err := entityReader(c.dctype, input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodDelete, u, data)
	if err != nil {
		return nil, err
	}
	return c.Exec(req.WithContext(cxt), output, opts...)
}

// Perform a request and attempt to unmarshal the response into an entity.
func (c *Client) Exec(req *http.Request, entity interface{}, opts ...Option) (*http.Response, error) {
	conf := Config{}.With(opts)
	for k, v := range conf.Header {
		for _, e := range v {
			req.Header.Set(k, e)
		}
	}

	rsp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	if entity != nil {
		err = c.unmarshal(rsp, req, entity)
		if err != nil {
			return nil, err
		}
	}
	return rsp, nil
}

// Unmarshal the provided response into the provided entity. The caller must close
// the response body, this method will not do so.
func (c *Client) unmarshal(rsp *http.Response, req *http.Request, entity interface{}) error {
	var ent *Entity
	if c.isDebug(req) || c.isVerbose(req) {
		data, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		ent = &Entity{
			ContentType: rsp.Header.Get("Content-Type"),
			Data:        data,
		}
		rsp.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	}
	err := Unmarshal(rsp, entity)
	if err != nil {
		return Errorf(rsp.StatusCode, "Could not unmarshal response").
			SetRequest(req).
			SetEntity(ent).
			SetCause(wrapErr(err, ErrCouldNotUnmarshalResponse))
	}
	return nil
}

// Perform a request. The client may mutate the parameter request.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.RoundTrip(req)
}

// Route-trip a request. The client may mutate the parameter request.
func (c *Client) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	reqid := atomic.AddInt64(&reqctr, 1)
	cxt := req.Context()

	if c.base != nil {
		req.URL = c.base.ResolveReference(req.URL)
	}

	domain := req.URL.Host
	defer func() {
		requestDurationSampler.With(metrics.Tags{"domain": domain}).Observe(float64(time.Since(start)))
	}()

	if c.auth != nil {
		err := c.auth.Authorize(req)
		if err != nil {
			return nil, errutil.Redact(fmt.Errorf("Could not authorize request: %v", err), ErrCouldNotAuthorize)
		}
	}
	for k, v := range c.header {
		n := http.CanonicalHeaderKey(k)
		if _, set := req.Header[n]; !set { // don't overrwrite explicitly set headers
			req.Header[n] = v
		}
	}

	if l := c.limiter; l != nil {
		if c.isVerbose(req) {
			state := c.limiter.State(start)
			fmt.Printf("api: [%06d] %v %v: rate limit state: limit=%d, remaining=%d, reset=%v (in %v)\n", reqid, req.Method, req.URL, state.Limit, state.Remaining, state.Reset, state.Reset.Sub(start))
		}
		next, err := l.Next(start, ratelimit.WithRequest(req))
		if err != nil {
			return nil, fmt.Errorf("Could not compute next rate-limited request window: %w", err)
		}
		delay := next.Sub(time.Now())
		rateLimitDelaySampler.With(metrics.Tags{"domain": domain}).Observe(float64(delay))
		if delay > 0 {
			if c.isVerbose(req) {
				fmt.Printf("api: [%06d] %v %v: delaying %v for rate limits\n", reqid, req.Method, req.URL, delay)
			}
			select {
			case <-time.After(delay):
			case <-cxt.Done():
				return nil, context.Canceled
			}
		}
	}

	if c.isVerbose(req) || c.isDebug(req) {
		fmt.Printf("api: [%06d] %v %v\n", reqid, req.Method, req.URL)
	}
	if c.isDebug(req) {
		b := &bytes.Buffer{}
		req.Header.Write(b)
		fmt.Println(text.Indent(string(b.Bytes()), "   - "))
		if c.isVerbose(req) && req.Body != nil {
			defer req.Body.Close()
			d, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			req.Body = ioutil.NopCloser(bytes.NewBuffer(d))
			if len(d) > 0 {
				fmt.Println(text.Indent(string(d), "   > "))
			}
		}
	}

	var rsp *http.Response
retries:
	for i := 0; ; i++ {
		tsp, err := c.Client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { // note that all these defers queue up and unravel on return
			if tsp != nil { // if set, this temporary response never converted; clean up
				tsp.Body.Close()
			}
		}()

		var rlerr error
		if l := c.limiter; l != nil {
			rlerr = l.Update(start, ratelimit.WithResponse(tsp)) // first, update rate limiter state to avoid an error response going unaccounted for
			if rlerr != nil {
				var retry ratelimit.RetryError
				if errors.As(rlerr, &retry) { // special handling for retries; insert a specific delay and re-perform the same request
					if i >= maxRetries {
						return nil, rlerr
					}
					delay := retry.RetryAfter.Sub(time.Now())
					rateLimitRetrySampler.With(metrics.Tags{"domain": domain}).Observe(float64(delay))
					if c.isVerbose(req) {
						fmt.Printf("api: [%06d] %v %v: retrying after %v due to rate limits\n", reqid, req.Method, req.URL, retry.RetryAfter)
					}
					select {
					case <-time.After(delay):
						continue retries
					case <-cxt.Done():
						return nil, context.Canceled
					}
				}
			}
		}

		if c.retry != nil && i < maxRetries && !isSuccess(tsp.StatusCode) {
			if _, ok := c.retry[tsp.StatusCode]; ok { // recoverable failure; wait and then try again up to our retry limit
				var delay time.Duration
				if c.backoff > 0 {
					delay = c.backoff
				} else {
					delay = backoffDefault
				}
				delay = delay * time.Duration(i+1) // progressive backoff
				failureRetrySampler.With(metrics.Tags{"domain": domain}).Observe(float64(delay))
				if c.isVerbose(req) {
					fmt.Printf("api: [%06d] %v %v: retrying after %v due to recoverable failure: %s\n", reqid, req.Method, req.URL, delay, tsp.Status)
				}
				select {
				case <-time.After(delay):
					continue retries
				case <-cxt.Done():
					return nil, context.Canceled
				}
			}
		}

		err = checkErr(reqid, req, tsp)
		if err != nil { // first, check for non-2XX/application-level errors
			return nil, err
		}
		if rlerr != nil { // second, handle any non-retry rate limiting errors that may have occurred
			return nil, fmt.Errorf("api: [%06d] %v %v: rate limit error: %v", reqid, req.Method, req.URL, rlerr)
		}

		// the response will be returned; convert it and clear the temporary value
		rsp, tsp = tsp, nil
		break
	}

	if c.isVerbose(req) || c.isDebug(req) {
		var l string
		if rsp.ContentLength >= 0 {
			l = humanize.Bytes(uint64(rsp.ContentLength))
		} else {
			l = "<unknown>"
		}
		fmt.Printf("api: [%06d] %v %v -> %v (%v)\n", reqid, req.Method, req.URL, rsp.Status, l)
	}

	if c.isDebug(req) {
		b := &bytes.Buffer{}
		rsp.Header.Write(b)
		fmt.Println(text.Indent(string(b.Bytes()), "   - "))
		if c.isVerbose(req) {
			d, err := ioutil.ReadAll(rsp.Body)
			if err != nil {
				return nil, err
			}
			if len(d) > 0 {
				fmt.Println(text.Indent(string(d), "   < "))
			}
			rsp.Body = ioutil.NopCloser(bytes.NewBuffer(d))
		}
	}

	return rsp, nil
}

func URLWithParams(s string, params interface{}) (string, error) {
	v := reflect.ValueOf(params)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}
	q, err := query.Values(params)
	if err != nil {
		return s, err
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
