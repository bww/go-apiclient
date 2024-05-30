package multiplex

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync/atomic"
	"time"

	api "github.com/bww/go-apiclient/v1"

	"github.com/bww/go-exec/v1"
	siter "github.com/bww/go-iterator/v1"
	"github.com/bww/go-util/v1/ext"
)

var _reqid uint64

func nextReq() uint64 {
	return atomic.AddUint64(&_reqid, 1)
}

type Config struct {
	Errors  ErrorHandler
	Headers map[string]string
	Verbose bool
	Debug   bool
}

func (c Config) WithOptions(opts []Option) Config {
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}

func (c Config) ConfigureRequest(req *http.Request) (*http.Request, error) {
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

type Option func(Config) Config

func WithErrorHandler(h ErrorHandler) Option {
	return func(c Config) Config {
		c.Errors = h
		return c
	}
}

func WithHeaders(h map[string]string) Option {
	return func(c Config) Config {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		for k, v := range h {
			c.Headers[k] = v
		}
		return c
	}
}

type RequestProducer interface {
	Request(int) (*http.Request, error)
}

type RequestProducerFunc func(int) (*http.Request, error)

func (p RequestProducerFunc) Request(i int) (*http.Request, error) {
	return p(i)
}

type StaticRequestProducer []*http.Request

func (p StaticRequestProducer) Request(i int) (*http.Request, error) {
	if i < len(p) {
		return p[i], nil
	} else {
		return nil, nil
	}
}

type URLRequestProducer struct {
	method string
	urls   []string
}

func NewGet(u []string) URLRequestProducer {
	return URLRequestProducer{
		method: http.MethodGet,
		urls:   u,
	}
}

func NewDelete(u []string) URLRequestProducer {
	return URLRequestProducer{
		method: http.MethodDelete,
		urls:   u,
	}
}

func (p URLRequestProducer) Request(i int) (*http.Request, error) {
	if i >= len(p.urls) {
		return nil, nil
	}
	req, err := http.NewRequest(p.method, p.urls[i], nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

type Result struct {
	Index    int
	Response *http.Response
}

type resultSet []*Result

func (r resultSet) Len() int           { return len(r) }
func (r resultSet) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r resultSet) Less(i, j int) bool { return r[i].Index < r[j].Index }

func Collect(iter siter.Iterator[*Result], err error) ([]*http.Response, error) {
	if err != nil {
		return nil, err
	}

	var buf []*Result
	for {
		res, err := iter.Next()
		if errors.Is(err, siter.ErrClosed) {
			break
		} else if err != nil {
			return nil, err
		}
		buf = append(buf, res)
	}

	sort.Sort(resultSet(buf))
	rsp := make([]*http.Response, len(buf))
	for i, e := range buf {
		rsp[i] = e.Response
	}

	return rsp, nil
}

func Unmarshal[E any](iter siter.Iterator[*Result], ents []E) ([]E, error) {
	rsps, err := Collect(iter, nil)
	if err != nil {
		return nil, fmt.Errorf("Could not collect responses: %w", err)
	}
	ents = ents[0:0:len(ents)]
	for _, r := range rsps {
		var e E
		err := api.Unmarshal(r, &e)
		if err != nil {
			return nil, err
		}
		ents = append(ents, e)
	}
	return ents, nil
}

type Mux struct {
	*api.Client
	concur  int
	errors  ErrorHandler
	verbose bool
	debug   bool
}

func New(c *api.Client, n int) *Mux {
	return &Mux{
		Client:  c,
		concur:  max(1, n),
		verbose: os.Getenv("VERBOSE_API_MUX") != "",
		debug:   os.Getenv("DEBUG_API_MUX") != "",
	}
}

// Create a block for execution on a dispatcher
func block(cxt context.Context, conf Config, mux *Mux, i int, req *http.Request, iter siter.Writer[*Result]) func() error {
	reqid := nextReq()
	errh := ext.Coalesce(conf.Errors, mux.errors)
	return func() error {
		start := time.Now()
		if mux.debug && mux.verbose {
			fmt.Printf("api: mux: [%06d, %d] >>> %s %v\n", reqid, i, req.Method, req.URL)
		}
		rsp, err := mux.Client.Do(req.WithContext(cxt))
		if err != nil && errh != nil { // let the error handler process first if we have one
			rsp, err = errh.Handle(rsp, err)
		}
		if err != nil {
			return fmt.Errorf("Could not multiplex request: %w", err)
		} else if rsp == nil {
			return nil // error handler consumed response
		}
		if mux.debug {
			fmt.Printf("api: mux: [%06d, %d] <<< %s %v: %s in %v\n", reqid, i, req.Method, req.URL, rsp.Status, time.Now().Sub(start))
		}
		return iter.Write(&Result{
			Index:    i,
			Response: rsp,
		})
	}
}

// Do executes requests in parallel, returning a set of counterpart responses.
func (m *Mux) Do(cxt context.Context, p RequestProducer, opts ...Option) (siter.Iterator[*Result], error) {
	conf := Config{}.WithOptions(opts)

	dsp := exec.NewDispatcher(m.concur, m.concur)
	err := dsp.Run(cxt)
	if err != nil {
		return nil, err
	}

	proc := make(chan siter.Result[*Result], m.concur)
	iter := siter.New[*Result](proc)

	go func() {
		defer func() {
			iter.Cancel(dsp.Error())
		}()
	outer:
		for i := 0; ; i++ {
			select {
			case <-cxt.Done():
				break outer
			default:
				// proceed
			}
			req, err := p.Request(i)
			if err != nil {
				iter.Cancel(err)
				return
			} else if req == nil {
				break outer // no more requests
			}
			req, err = conf.ConfigureRequest(req)
			if err != nil {
				iter.Cancel(err)
				return
			}
			err = dsp.Exec(block(cxt, conf, m, i, req, iter))
			if errors.Is(err, exec.ErrCanceled) {
				break outer // dispatcher stopped, probably due to a previous error
			} else if err != nil {
				iter.Cancel(err)
				return
			}
		}
	}()

	return iter, nil
}
