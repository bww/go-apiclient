package api

import (
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/bww/go-apiclient/v1/events"
	"github.com/bww/go-ratelimit/v1"
)

type Debug struct {
	Debug     bool
	Verbose   bool
	FilterURL *regexp.Regexp
}

func (d Debug) Matches(req *http.Request) bool {
	if f := d.FilterURL; f != nil {
		if !f.MatchString(req.URL.Path) {
			return false
		}
	}
	return true
}

func (d Debug) WithEnv() (Debug, error) {
	e := d
	e.Debug = d.Debug || os.Getenv("DEBUG_API_CLIENT") != ""
	e.Verbose = e.Debug || d.Verbose || os.Getenv("VERBOSE_API_CLIENT") != ""

	if v := os.Getenv("DEBUG_API_CLIENT_FILTER"); v != "" {
		m, err := regexp.Compile(v)
		if err != nil {
			return e, err
		}
		e.FilterURL = m
	}

	return e, nil
}

// Client configuration
type Config struct {
	BaseURL     string
	Timeout     time.Duration
	Client      *http.Client
	Authorizer  Authorizer
	Observers   *events.Observers
	RateLimiter ratelimit.Limiter
	RetryStatus []int
	RetryDelay  time.Duration
	Header      http.Header
	ContentType string
	Verbose     bool
	Debug       bool
}

func (c Config) With(opts []Option) Config {
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}

type Option func(Config) Config

func WithAuthorizer(auth Authorizer) Option {
	return func(c Config) Config {
		c.Authorizer = auth
		return c
	}
}

func WithObservers(obs *events.Observers) Option {
	return func(c Config) Config {
		c.Observers = obs
		return c
	}
}

func WithBaseURL(base string) Option {
	return func(c Config) Config {
		c.BaseURL = base
		return c
	}
}

func WithHeader(key, val string) Option {
	return func(c Config) Config {
		if c.Header == nil {
			c.Header = make(http.Header)
		}
		c.Header.Set(key, val)
		return c
	}
}

func WithHeaders(hdr http.Header) Option {
	return func(c Config) Config {
		if c.Header == nil {
			c.Header = hdr
		} else {
			for k, v := range hdr {
				c.Header[k] = v
			}
		}
		return c
	}
}

func WithDebug(on bool) Option {
	return func(c Config) Config {
		c.Debug, c.Verbose = on, on
		return c
	}
}

func WithRateLimiter(l ratelimit.Limiter) Option {
	return func(c Config) Config {
		c.RateLimiter = l
		return c
	}
}

func WithRetryStatus(s ...int) Option {
	return func(c Config) Config {
		c.RetryStatus = s
		return c
	}
}

func WithRetryDelay(d time.Duration) Option {
	return func(c Config) Config {
		c.RetryDelay = d
		return c
	}
}

func (c Config) WithOptions(opts []Option) Config {
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}
