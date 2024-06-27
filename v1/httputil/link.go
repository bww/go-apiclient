package httputil

import (
	"net/http"
)

type Link struct {
	URL    string
	Params map[string]string
}

func (l *Link) Results() bool {
	return l.Params != nil && l.Params["results"] == "true"
}

// ParseNext parses the next link from the response header
func ParseNext(rsp *http.Response) (*Link, error) {
	if rsp == nil {
		return nil, nil
	}
	hdr := rsp.Header.Get("Link")
	if hdr == "" {
		return nil, nil
	}
	links, err := parseLinks(hdr)
	if err != nil {
		return nil, err
	}
	next, ok := links["next"]
	if !ok {
		return nil, nil
	}
	return &Link{
		URL:    next.URL,
		Params: next.Params,
	}, nil
}

// NextPage returns the URL of the next link from the response header
func NextPage(rsp *http.Response) (string, error) {
	next, err := ParseNext(rsp)
	if err != nil {
		return "", err
	} else if next == nil {
		return "", nil
	}
	if p := next.Params; p != nil {
		// if the results parameter is present and it is not "true", we have
		// no further results. normally, this parameter will not be present.
		if v, ok := p["results"]; ok && v != "true" {
			return "", nil
		}
	}

	return next.URL, nil
}
