package httpclient

import "io"

type Request struct {
	Method         string
	Path           string
	Query          map[string]string
	JSONBody       interface{}
	Headers        map[string]string
	MultipartField string
	MultipartName  string
	Multipart      io.Reader
}
