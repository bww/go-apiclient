package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bww/go-ratelimit/v1"
	"github.com/bww/go-rest/v2"
	"github.com/bww/go-router/v2"
	"github.com/bww/go-util/v1/debug"
	"github.com/stretchr/testify/assert"
)

var (
	sharedCI = os.Getenv("CIRCLECI") != ""
	fence    = int64(1)
)

func params(p map[string]interface{}) string {
	q := make(url.Values)
	for k, v := range p {
		switch c := v.(type) {
		case string:
			q.Set(k, c)
		case time.Time:
			q.Set(k, c.Format(time.RFC3339Nano))
		default:
			q.Set(k, fmt.Sprint(c))
		}
	}
	return "?" + q.Encode()
}

type testService struct {
	svc *rest.Service
	svr *http.Server
	lnr net.Listener
}

func (s *testService) Addr() string {
	if s.lnr != nil {
		return fmt.Sprintf("localhost:%d", s.lnr.Addr().(*net.TCPAddr).Port)
	} else {
		return ""
	}
}

func (s *testService) Run() {
	lnr, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	svc, err := rest.New(rest.WithVerbose(debug.VERBOSE), rest.WithDebug(debug.DEBUG))
	if err != nil {
		panic(err)
	}

	svc.Add("/limited", s.handleRateLimited).Methods("GET")

	svr := &http.Server{
		Handler:      svc,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go svr.Serve(lnr)

	s.svc = svc
	s.svr = svr
	s.lnr = lnr
}

func (s *testService) handleRateLimited(req *router.Request, cxt router.Context) (*router.Response, error) {
	q := req.URL.Query()
	rsp := router.NewResponse(http.StatusOK)

	rsp.Header.Set("X-RateLimit-Limit", q.Get("lim"))
	rsp.Header.Set("X-RateLimit-Remaining", q.Get("rem"))
	rsp.Header.Set("X-RateLimit-Reset", q.Get("rst"))

	if v := q.Get("bck"); v != "" {
		fnc, err := strconv.Atoi(q.Get("fnc"))
		if err != nil {
			return nil, err
		}
		if atomic.AddInt64(&fence, 1) == int64(fnc+1) {
			rsp.Header.Set("Retry-After", v)
		}
	}

	return rsp, nil
}

var service testService

func TestMain(m *testing.M) {
	debug.DumpRoutinesOnInterrupt()
	service.Run()
	os.Exit(m.Run())
}

func windowBound(b, w time.Duration, c int) time.Duration {
	return time.Duration(float64((w*time.Duration(c))+b) * 0.95)
}

func TestBurstRateLimit(t *testing.T) {
	if sharedCI {
		fmt.Println("NOTICE: This test depends on wall time durations and is therefore too flakey to run on shared CI infrastructure")
		return
	}

	now := time.Now()
	win := time.Millisecond * 100
	bck := time.Millisecond * 500
	cxt := context.Background()

	api, err := NewWithConfig(Config{
		BaseURL: fmt.Sprintf("http://%s/", service.Addr()),
		Timeout: time.Second * 10,
		RateLimiter: ratelimit.NewHeaders(ratelimit.Config{
			Events:     10,
			Start:      now,
			Window:     win,
			Mode:       ratelimit.Burst,
			Durationer: ratelimit.Milliseconds,
		}),
		Verbose: debug.VERBOSE,
		Debug:   debug.DEBUG,
	})
	if err != nil {
		panic(err)
	}

	var n, c int
	var r int64
	var start time.Time
	var dur time.Duration

	n, c = 10, 1
	start = time.Now()

	fmt.Println("---")
	for i := 0; i <= (n*c)+c; i++ {
		if (i % n) == 0 {
			r = time.Now().Add(win).UnixNano() / int64(time.Millisecond)
		}
		api.Get(cxt, "/limited"+params(map[string]interface{}{
			"lim": n,
			"rem": n - (i % (n + 1)),
			"rst": r,
		}), nil)
	}

	dur = time.Since(start)
	fmt.Printf(">>> dur=%v, start=%v, n=%d, c=%d; (%v, %v)\n", dur, start, n, c, (win * time.Duration(c)), windowBound(0, win, c))
	assert.Equal(t, true, dur >= (win*time.Duration(c)) && dur < (win*time.Duration(c+1)))

	n, c = 10, 2
	start = time.Now()
	r = start.Add(win).UnixNano() / int64(time.Millisecond)

	fmt.Println("---")
	for i := 0; i <= (n*c)+c; i++ {
		if (i % n) == 0 {
			r = time.Now().Add(win).UnixNano() / int64(time.Millisecond)
		}
		api.Get(cxt, "/limited"+params(map[string]interface{}{
			"lim": n,
			"rem": n - (i % (n + 1)),
			"rst": r,
		}), nil)
	}

	dur = time.Since(start)
	fmt.Printf(">>> dur=%v, start=%v, n=%d, c=%d; (%v, %v)\n", dur, start, n, c, (win * time.Duration(c)), windowBound(0, win, c))
	assert.Equal(t, true, dur >= (win*time.Duration(c)) && dur < (win*time.Duration(c+1)))

	n, c = 10, 2
	start = time.Now()
	r = start.Add(win).UnixNano() / int64(time.Millisecond)

	fmt.Println("---")
	for i := 0; i <= (n*c)+c; i++ {
		if (i % n) == 0 {
			r = time.Now().Add(win).UnixNano() / int64(time.Millisecond)
		}
		p := map[string]interface{}{
			"lim": n,
			"rem": n - (i % (n + 1)),
			"rst": r,
		}
		if i == 5 {
			p["bck"] = int64(bck / time.Millisecond)
			p["fnc"] = atomic.LoadInt64(&fence)
		}
		api.Get(cxt, "/limited"+params(p), nil)
	}

	dur = time.Since(start)
	fmt.Printf(">>> dur=%v, start=%v, n=%d, c=%d; (%v, %v)\n", dur, start, n, c, (win * time.Duration(c)), windowBound(0, win, c))
	assert.Equal(t, true, dur >= (win*time.Duration(c))+bck && dur < (win*time.Duration(c+1))+bck)
}

func TestMeterRateLimit(t *testing.T) {
	if sharedCI {
		fmt.Println("NOTICE: This test depends on wall time durations and is therefore too flakey to run on shared CI infrastructure")
		return
	}

	now := time.Now()
	win := time.Millisecond * 100
	cxt := context.Background()

	api, err := NewWithConfig(Config{
		BaseURL: fmt.Sprintf("http://%s/", service.Addr()),
		Timeout: time.Second * 10,
		RateLimiter: ratelimit.NewHeaders(ratelimit.Config{
			Events:     10,
			Start:      now,
			Window:     win,
			Mode:       ratelimit.Meter,
			Durationer: ratelimit.Milliseconds,
		}),
		Verbose: debug.VERBOSE,
		Debug:   debug.DEBUG,
	})
	if err != nil {
		panic(err)
	}

	var n, c int
	var r int64
	var start time.Time
	var sum, dur, del, avg time.Duration

	sum = 0
	n, c = 10, 1
	start = time.Now()

	fmt.Println("---")
	for i := 0; i <= n*c; i++ {
		s := time.Now()
		if (i % n) == 0 {
			r = time.Now().Add(win).UnixNano() / int64(time.Millisecond)
		}
		api.Get(cxt, "/limited"+params(map[string]interface{}{
			"lim": n,
			"rem": n - (i % (n + 1)),
			"rst": r,
		}), nil)
		sum += time.Since(s)
	}

	dur = time.Since(start)
	del = sum / time.Duration(n*c)
	avg = win * time.Duration(c) / time.Duration(n*c)
	fmt.Printf(">>> dur=%v, start=%v, n=%d, c=%d, avg=%v, del=%v\n", dur, start, n, c, avg, del)
	assert.InEpsilon(t, avg, del, 0.025)

	max := time.Millisecond * 5
	api, err = NewWithConfig(Config{
		BaseURL: fmt.Sprintf("http://%s/", service.Addr()),
		Timeout: time.Second * 10,
		RateLimiter: ratelimit.NewHeaders(ratelimit.Config{
			Events:     10,
			Start:      now,
			Window:     win,
			Mode:       ratelimit.Meter,
			Durationer: ratelimit.Milliseconds,
			MaxDelay:   max,
		}),
		Verbose: debug.VERBOSE,
		Debug:   debug.DEBUG,
	})
	if err != nil {
		panic(err)
	}

	sum = 0
	n, c = 10, 1
	start = time.Now()

	fmt.Println("---")
	for i := 0; i <= n*c; i++ {
		s := time.Now()
		if (i % n) == 0 {
			r = time.Now().Add(win).UnixNano() / int64(time.Millisecond)
		}
		api.Get(cxt, "/limited"+params(map[string]interface{}{
			"lim": n,
			"rem": n - (i % (n + 1)),
			"rst": r,
		}), nil)
		sum += time.Since(s)
	}

	dur = time.Since(start)
	del = sum / time.Duration(n*c)
	avg = max
	fmt.Printf(">>> dur=%v, start=%v, n=%d, c=%d, avg=%v, del=%v\n", dur, start, n, c, avg, del)
	assert.InEpsilon(t, avg, del, 0.333)
}
