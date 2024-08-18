package multiplex

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	api "github.com/bww/go-apiclient/v1"

	siter "github.com/bww/go-iterator/v1"
	"github.com/bww/go-rest/v2"
	"github.com/bww/go-router/v2"
	"github.com/bww/go-util/v1/debug"
	"github.com/bww/go-util/v1/errors"
	"github.com/stretchr/testify/assert"
)

func init() {
	debug.DumpRoutinesOnInterrupt()
}

type number int

func (n *number) UnmarshalText(text []byte) error {
	x, err := strconv.Atoi(string(text))
	*n = number(x)
	return err
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

	svc := errors.Must(rest.New(rest.WithVerbose(debug.VERBOSE), rest.WithDebug(debug.DEBUG)))
	svc.Add("/hello/{index}", s.handleRequest).Methods("GET")

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

func (s *testService) handleRequest(req *router.Request, cxt router.Context) (*router.Response, error) {
	return router.NewResponse(http.StatusOK).SetString("text/plain", cxt.Vars["index"])
}

func TestMultiplex(t *testing.T) {
	svc := &testService{}
	svc.Run()

	cli, err := api.NewWithConfig(api.Config{BaseURL: fmt.Sprintf("http://%s/", svc.Addr())})
	assert.NoError(t, err)
	px := New(cli, 20)

	n := 1000
	t.Run("Expect errors", func(t *testing.T) {
		urls := make([]string, n)
		for i := 0; i < n; i++ {
			urls[i] = fmt.Sprintf("%d", i)
		}

		cxt, cancel := context.WithCancel(context.Background())
		defer cancel()

		iter, err := px.Do(cxt, NewGet(urls))
		if assert.NoError(t, err) {
			for {
				_, err := iter.Next()
				var apierr *api.Error
				if assert.ErrorAs(t, err, &apierr) {
					assert.Equal(t, http.StatusNotFound, apierr.Status)
				}
				break
			}
		}
	})

	t.Run("Expect success", func(t *testing.T) {
		urls := make([]string, n)
		for i := 0; i < n; i++ {
			urls[i] = fmt.Sprintf("hello/%d", i)
		}

		cxt, cancel := context.WithCancel(context.Background())
		defer cancel()

		iter, err := px.Do(cxt, NewGet(urls))
		if assert.NoError(t, err) {
			for {
				res, err := iter.Next()
				if err != nil {
					assert.ErrorIs(t, err, siter.ErrClosed)
					break
				}
				data, err := io.ReadAll(res.Response.Body)
				if assert.NoError(t, err) {
					assert.Equal(t, []byte(fmt.Sprintf("%d", res.Index)), data)
					fmt.Println(">>> ", res.Index, string(data))
				}
			}
		}
	})

	t.Run("Collect results", func(t *testing.T) {
		urls := make([]string, n)
		for i := 0; i < n; i++ {
			urls[i] = fmt.Sprintf("hello/%d", i)
		}

		cxt, cancel := context.WithCancel(context.Background())
		defer cancel()

		rsps, err := Collect(px.Do(cxt, NewGet(urls)))
		if assert.NoError(t, err) {
			if assert.Len(t, rsps, n) {
				for i, e := range rsps {
					data, err := io.ReadAll(e.Body)
					if assert.NoError(t, err) {
						assert.Equal(t, []byte(fmt.Sprintf("%d", i)), data)
						fmt.Println(">>> ", i, string(data))
					}
				}
			}
		}
	})

	t.Run("Unmarshal results", func(t *testing.T) {
		urls := make([]string, n)
		for i := 0; i < n; i++ {
			urls[i] = fmt.Sprintf("hello/%d", i)
		}

		cxt, cancel := context.WithCancel(context.Background())
		defer cancel()

		iter, err := px.Do(cxt, NewGet(urls))
		if assert.NoError(t, err) {
			var nums []number
			nums, err = Unmarshal(iter, nums)
			if assert.NoError(t, err) {
				if assert.Len(t, nums, n) {
					for i, e := range nums {
						assert.Equal(t, i, int(e))
						fmt.Println(">>> ", i, e)
					}
				}
			}
		}
	})
}
