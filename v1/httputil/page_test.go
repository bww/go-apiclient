package httputil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLinks(t *testing.T) {
	tests := []struct {
		Header string
		Expect map[string]link
		Error  error
	}{
		{
			"<https://this.is.dumb/okay?yeah>; rel=\"example\"",
			map[string]link{
				"example": link{
					URL: "https://this.is.dumb/okay?yeah",
					Rel: "example",
					Params: map[string]string{
						"rel": "example",
					},
				},
			},
			nil,
		},
		{
			"<https://this.is.dumb/okay?yeah>; foo=\"another\"; rel=\"example\",\t<https://this.is.stupid/bammo?ok>; rel=\"another\"",
			map[string]link{
				"example": link{
					URL: "https://this.is.dumb/okay?yeah",
					Rel: "example",
					Params: map[string]string{
						"rel": "example",
						"foo": "another",
					},
				},
				"another": link{
					URL: "https://this.is.stupid/bammo?ok",
					Rel: "another",
					Params: map[string]string{
						"rel": "another",
					},
				},
			},
			nil,
		},
		{
			"<https://this.is.dumb/okay?yeah>; this=\"example\",\t<https://this.is.stupid/bammo?ok>; this=\"another\"",
			map[string]link{},
			nil,
		},
		{
			"<https://this.is.dumb/okay?yeah; rel=\"example\",\t<https://this.is.stupid/bammo?ok>; rel=\"another\"",
			map[string]link{},
			errMalformedLinks,
		},
		{
			"<https://this.is.dumb/okay?yeah>; rel=\"example\",\thttps://this.is.stupid/bammo?ok>; rel=\"another\"",
			map[string]link{},
			errMalformedLinks,
		},
		{
			"<https://this.is.dumb/okay?yeah>; =\"example\"",
			map[string]link{},
			errMalformedParam,
		},
		{
			"<https://this.is.dumb/okay?yeah>; foo=\"This is another one\"; rel=example",
			map[string]link{
				"example": link{
					URL: "https://this.is.dumb/okay?yeah",
					Rel: "example",
					Params: map[string]string{
						"rel": "example",
						"foo": "This is another one",
					},
				},
			},
			nil,
		},
		{
			"<https://this.is.dumb/okay?yeah>; rel=\"\\\"example\\\"\"",
			map[string]link{
				"\"example\"": link{
					URL: "https://this.is.dumb/okay?yeah",
					Rel: "\"example\"",
					Params: map[string]string{
						"rel": "\"example\"",
					},
				},
			},
			nil,
		},
		{
			"<https://this.is.dumb/okay?yeah>; rel=",
			map[string]link{
				"": link{
					URL: "https://this.is.dumb/okay?yeah",
					Rel: "",
					Params: map[string]string{
						"rel": "",
					},
				},
			},
			nil,
		},
	}
	for i, e := range tests {
		r, err := parseLinks(e.Header)
		if e.Error != nil {
			fmt.Printf("*** [#%d] %v\n", i, err)
			assert.Equal(t, e.Error, err, fmt.Sprintf("[#%d]", i))
		} else if assert.Nil(t, err, fmt.Sprint(err)) {
			fmt.Printf("--> [#%d] %v\n", i, r)
			assert.Equal(t, e.Expect, r, fmt.Sprintf("[#%d]", i))
		}
	}
}
