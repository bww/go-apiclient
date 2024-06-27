package httputil

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func responseWithLink(s string) *http.Response {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Link": []string{s},
		},
	}
}

func TestNextLink(t *testing.T) {
	tests := []struct {
		Response *http.Response
		Expect   string
		Error    error
	}{
		{
			Response: responseWithLink("<https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=1&per_page=3>; rel=\"prev\"; another=foo, <https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=3&per_page=3>; rel=\"next\", <https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=1&per_page=3>; rel=\"first\", <https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=3&per_page=3>; rel=\"last\""),
			Expect:   "https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=3&per_page=3",
			Error:    nil,
		},
		{
			Response: responseWithLink("<https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=1&per_page=3>; another=foo; rel=\"prev\""),
			Expect:   "",
			Error:    nil,
		},
		{
			Response: responseWithLink("<https://this.is.dumb/okay?yeah>; rel=\"example\",\thttps://this.is.stupid/bammo?ok>; another=\"foo\"; rel=\"another\""),
			Expect:   "",
			Error:    errMalformedLinks,
		},
		{
			Response: responseWithLink("<https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=1&per_page=3>; another=\"foo\"; rel=\"last\""),
			Expect:   "",
			Error:    nil,
		},
		{
			Response: responseWithLink(""),
			Expect:   "",
			Error:    nil,
		},
		{
			Response: responseWithLink("<https://sentry.io/api/0/organizations/?&cursor=1495610229497:0:1>; rel=\"previous\"; results=\"false\"; cursor=\"1495610229497:0:1\", <https://sentry.io/api/0/organizations/?&cursor=1495610229498:100:0>; rel=\"next\"; results=true; cursor=\"1495610229498:100:0\""),
			Expect:   "https://sentry.io/api/0/organizations/?&cursor=1495610229498:100:0",
			Error:    nil,
		},
		{
			Response: responseWithLink("<https://sentry.io/api/0/organizations/?&cursor=1495610229497:0:1>; rel=\"previous\"; results=\"false\"; cursor=\"1495610229497:0:1\", <https://sentry.io/api/0/organizations/?&cursor=1495610229498:100:0>; rel=\"next\"; results=\"false\"; cursor=\"1495610229498:100:0\""),
			Expect:   "",
			Error:    nil,
		},
	}

	for _, tt := range tests {
		r, err := NextPage(tt.Response)
		if tt.Error != nil {
			assert.Equal(t, tt.Error, err)
		} else if assert.Nil(t, err, fmt.Sprint(err)) {
			assert.Equal(t, tt.Expect, r)
		}
	}
}
