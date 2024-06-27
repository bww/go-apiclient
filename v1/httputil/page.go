package httputil

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

var (
	errMalformedLinks = errors.New("Malformed links")
	errMalformedParam = errors.New("Malformed params")
)

type link struct {
	URL, Rel string
	Params   map[string]string
}

// Parse parses a raw Link header in the form:
//
//	<url>; rel="foo", <url>; rel="bar"; wat="dis"
//
// ...returning a slice of Link structs
//
// NOTE: there is a known bug in link parsing which we can't be bothered to fix
// at the moment. Specifically, we will not correctly handle a parameter that
// contains a ';' character in its text due to the naieve approach we take to
// delimiter handling. Specifically, the following will not work as intended:
//
//	<url>; rel="foo"; foo="contains; a literal semicolon"
func parseLinks(src string) (map[string]link, error) {
	links := make(map[string]link)

	for len(src) > 0 {
		var part string
		if x := strings.Index(src, ","); x > 0 {
			part, src = src[:x], src[x+1:]
		} else {
			part, src = src, ""
		}

		var url, arg string
		if x := strings.Index(part, ";"); x > 0 {
			url, arg = strings.TrimSpace(part[:x]), strings.TrimSpace(part[x+1:])
		} else {
			url = strings.TrimSpace(part)
		}

		if l := len(url); l < 2 {
			return nil, errMalformedLinks
		} else if url[0] != '<' || url[l-1] != '>' {
			return nil, errMalformedLinks
		} else {
			url = url[1 : l-1]
		}

		params := make(map[string]string)
		args := strings.Split(arg, ";")
		for _, a := range args {
			if len(a) > 0 {
				key, val, err := parseParam(a)
				if err != nil {
					return nil, err
				}
				key = strings.TrimSpace(key)
				val = strings.TrimSpace(val)
				params[key] = val
			}
		}

		if rel, ok := params["rel"]; ok {
			links[rel] = link{
				URL:    url,
				Rel:    rel,
				Params: params,
			}
		}

		for i, r := range src {
			if !unicode.IsSpace(r) {
				src = src[i:]
				break
			}
		}
	}

	return links, nil
}

func parseParam(src string) (string, string, error) {
	var key, val string
	if x := strings.Index(src, "="); x < 0 {
		return "", "", errMalformedParam
	} else {
		key, val = src[:x], src[x+1:]
	}
	if len(key) < 1 {
		return "", "", errMalformedParam
	}
	if len(val) > 0 && val[0] == '"' {
		var err error
		val, err = strconv.Unquote(val)
		if err != nil {
			return "", "", err
		}
	}
	return key, val, nil
}
