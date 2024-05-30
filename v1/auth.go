package api

import (
	"encoding/base64"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

// An authorizer authorizes requests
type Authorizer interface {
	Authorize(*http.Request) error
}

type HeaderAuthorizer struct {
	header http.Header
}

func NewHeaderAuthorizer(h http.Header) HeaderAuthorizer {
	return HeaderAuthorizer{h}
}

func (a HeaderAuthorizer) Authorize(req *http.Request) error {
	for k, v := range a.header {
		if len(v) > 0 {
			req.Header.Set(k, v[0])
		}
	}
	return nil
}

type QueryAuthorizer struct {
	Params url.Values
}

func NewQueryAuthorizer(params url.Values) QueryAuthorizer {
	return QueryAuthorizer{
		params,
	}
}

func (a QueryAuthorizer) Authorize(req *http.Request) error {

	q := req.URL.Query()
	for k, v := range a.Params {
		for _, i := range v {
			q.Add(k, i)
		}
	}
	req.URL.RawQuery = q.Encode()
	return nil
}

type BasicAuthorizer struct {
	user, pass string
}

func NewBasicAuthorizer(u, p string) BasicAuthorizer {
	return BasicAuthorizer{u, p}
}

func (a BasicAuthorizer) Authorize(req *http.Request) error {
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(a.user+":"+a.pass)))
	return nil
}

type BearerAuthorizer struct {
	token string
}

func NewBearerAuthorizer(t string) BearerAuthorizer {
	return BearerAuthorizer{t}
}

func (a BearerAuthorizer) Authorize(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}

type OAuthAuthorizer struct {
	src oauth2.TokenSource
}

func NewOAuthAuthorizer(src oauth2.TokenSource) OAuthAuthorizer {
	return OAuthAuthorizer{src}
}

func (a OAuthAuthorizer) Token() (*oauth2.Token, error) {
	return a.src.Token()
}

func (a OAuthAuthorizer) Authorize(req *http.Request) error {
	tok, err := a.src.Token()
	if err != nil {
		return err
	}
	tok.SetAuthHeader(req)
	return nil
}
