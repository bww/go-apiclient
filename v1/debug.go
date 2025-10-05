package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/bww/go-util/v1/text"
)

var sensitiveHeaders = map[string]struct{}{
	http.CanonicalHeaderKey("Authorization"): {},
}

func defaultAllowHeader(n string) bool {
	_, ok := sensitiveHeaders[n]
	return !ok // if it's not sensitive, it is allowed
}

func sanitizeHeaders(hdr http.Header, allowed func(string) bool) http.Header {
	res := make(http.Header)
	for k, v := range hdr {
		n := http.CanonicalHeaderKey(k)
		if allowed(n) {
			for _, e := range v {
				res.Add(n, e)
			}
		} else {
			for _, e := range v {
				hash := sha256.Sum256([]byte(e))
				res.Add(n, fmt.Sprintf("<apiclient: redacted %d bytes; SHA256=%s>", len(e), hex.EncodeToString(hash[:])))
			}
		}
	}
	return res
}

func (c *Client) dumpReq(w io.Writer, req *http.Request) error {
	b := &bytes.Buffer{}
	sanitizeHeaders(req.Header, defaultAllowHeader).Write(b)
	fmt.Println(text.Indent(b.String(), "   - "))
	if c.isVerbose(req) && req.Body != nil {
		defer req.Body.Close()
		d, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewBuffer(d))
		if len(d) > 0 {
			fmt.Fprintln(w, text.Indent(string(d), "   > "))
		}
	}
	return nil
}

func (c *Client) dumpRsp(w io.Writer, req *http.Request, rsp *http.Response) error {
	b := &bytes.Buffer{}
	sanitizeHeaders(rsp.Header, defaultAllowHeader).Write(b)
	fmt.Println(text.Indent(b.String(), "   - "))
	if c.isVerbose(req) {
		d, err := io.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		if len(d) > 0 {
			fmt.Fprintln(w, text.Indent(string(d), "   < "))
		}
		rsp.Body = io.NopCloser(bytes.NewBuffer(d))
	}
	return nil
}
