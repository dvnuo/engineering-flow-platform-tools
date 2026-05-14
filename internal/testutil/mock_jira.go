package testutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

func NewMockJira() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(401)
			w.Write([]byte(`{"errorMessages":["auth"]}`))
			return
		}
		p := r.URL.Path
		switch {
		case p == "/rest/api/2/myself":
			w.Write([]byte(`{"name":"bot"}`))
		case p == "/rest/api/2/serverInfo":
			w.Write([]byte(`{"baseUrl":"x"}`))
		case p == "/rest/api/2/issue/EFP-123":
			w.Write([]byte(`{"key":"EFP-123"}`))
		case p == "/rest/api/2/search":
			w.Write([]byte(`{"issues":[{"key":"EFP-123"}]}`))
		case p == "/rest/api/2/issue":
			w.Write([]byte(`{"id":"1"}`))
		case strings.HasSuffix(p, "/transitions"), strings.HasSuffix(p, "/comment"), strings.HasSuffix(p, "/attachments"), strings.HasPrefix(p, "/rest/api/2/attachment/"), p == "/rest/api/2/project", p == "/rest/api/2/field", p == "/rest/agile/1.0/board":
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
}
