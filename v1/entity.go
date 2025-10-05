package api

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/bww/go-util/v1/text"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/schema"
)

type EntityMarshaler interface {
	MarshalEntity() ([]byte, error)
}
type EntityUnmarshaler interface {
	UnmarshalEntity(string, []byte) error
}

type Entity struct {
	ContentType string
	Data        []byte
}

func (e Entity) String() string {
	var d string
	if isMimetypeBinary(e.ContentType) {
		b := &strings.Builder{}
		text.Hexdump(b, e.Data, 20)
		d = b.String()
	} else {
		d = string(e.Data)
	}
	return fmt.Sprintf("---\n%s (%s)\n---\n%s\n#", e.ContentType, humanize.Bytes(uint64(len(e.Data))), d)
}

var (
	formEncoder *schema.Encoder
	formDecoder *schema.Decoder
)

func init() {
	formEncoder = schema.NewEncoder()
	formDecoder = schema.NewDecoder()
	formDecoder.IgnoreUnknownKeys(true)
}

func entityReader(ctype string, entity interface{}) (io.Reader, error) {
	switch v := entity.(type) {
	case []byte:
		return bytes.NewReader(v), nil
	case io.ReadCloser:
		return v, nil
	case io.Reader:
		return v, nil
	default:
		return Marshal(ctype, entity)
	}
}

func Marshal(ctype string, entity interface{}) (io.Reader, error) {
	if entity == nil {
		return nil, nil
	}

	// first, try marshaling based on the content type
	m, _, err := mime.ParseMediaType(ctype)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(m) {
	case JSON:
		d, err := json.Marshal(entity)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(d), nil

	case URLEncoded, Multipart:
		val := make(url.Values)
		err := formEncoder.Encode(entity, val)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader([]byte(val.Encode())), nil
	}

	// second, try marshaling based on the entity's conformance to known interfaces
	switch e := entity.(type) {
	case EntityMarshaler:
		val, err := e.MarshalEntity()
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(val), nil

	case encoding.TextMarshaler:
		val, err := e.MarshalText()
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(val), nil

	case encoding.BinaryMarshaler:
		val, err := e.MarshalBinary()
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(val), nil
	}

	// couldn't identify a marshaler
	return nil, ErrUnsupportedMimetype
}

func Unmarshal(rsp *http.Response, entity interface{}) error {
	if rsp.StatusCode == http.StatusNoContent { // no content; just set the entity to nil
		val := reflect.ValueOf(entity)
		switch val.Kind() {
		case reflect.Interface, reflect.Pointer:
			p := val.Elem()
			p.Set(reflect.Zero(p.Type()))
		}
		return nil
	}

	m, _, err := mime.ParseMediaType(rsp.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	if rsp.Body != nil {
		defer rsp.Body.Close()
	}

	// first, try unmarshaling based on the content type
	switch strings.ToLower(m) {
	case JSON:
		return json.NewDecoder(rsp.Body).Decode(entity)

	case URLEncoded, Multipart:
		data, err := io.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		form, err := url.ParseQuery(string(data))
		if err != nil {
			return err
		}
		return formDecoder.Decode(entity, form)

	case PlainText:
		val, err := io.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		switch e := entity.(type) {
		case encoding.TextUnmarshaler:
			return e.UnmarshalText(val)
		case *string:
			*e = string(val)
			return nil
		case *[]byte:
			*e = val
			return nil
		default:
			return fmt.Errorf("attempting to unmarshal text/plain response into %T not supported, must be either encoding.TextMarshaler, *[]byte, or *string", e)
		}
	}

	// second, try unmarshaling based on the entity's conformance to known interfaces
	switch e := entity.(type) {
	case EntityUnmarshaler:
		val, err := io.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return e.UnmarshalEntity(m, val)
	}

	// couldn't identify a marshaler
	return ErrUnsupportedMimetype
}

func isMimetypeBinary(t string) bool {
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
