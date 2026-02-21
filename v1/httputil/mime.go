package httputil

import (
	"mime"
	"net/http"
	"strings"
)

// IsRequestPrintable attempts to determine if the the entity of the provided
// request can be printed for human consumption.
func IsRequestPrintable(req *http.Request) bool {
	return isEntityPrintable(req.Header)
}

// IsResponsePrintable attempts to determine if the the entity of the provided
// response can be printed for human consumption.
func IsResponsePrintable(rsp *http.Response) bool {
	return isEntityPrintable(rsp.Header)
}

// isEntityPrintable attempts to determine if the the entity of the request or
// response associated with the provied header can be printed for human
// consumption.
func isEntityPrintable(hdr http.Header) bool {
	return hdr.Get("Content-Encoding") == "" && IsMimetypePrintable(hdr.Get("Content-Type"))
}

// IsMimetypePrintable attempts to determine if the provided mimetype can be
// printed for human consumption.
func IsMimetypePrintable(t string) bool {
	m, p, err := mime.ParseMediaType(t)
	if err != nil {
		return true
	}
	if m == "application/json" {
		return false
	} else if strings.HasPrefix(m, "text/") {
		return false
	} else if _, ok := p["charset"]; ok {
		return false
	} else {
		return true
	}
}
