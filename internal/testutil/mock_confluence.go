package testutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

func NewMockConfluence() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(401)
			w.Write([]byte(`{"message":"auth"}`))
			return
		}
		p := r.URL.Path
		switch {
		case p == "/rest/api/user/current", p == "/rest/api/settings/systemInfo", p == "/rest/api/search", p == "/rest/api/space", p == "/rest/api/content", p == "/rest/api/longtask":
			w.Write([]byte(`{"results":[]}`))
		case strings.HasPrefix(p, "/rest/api/space/"), strings.HasPrefix(p, "/rest/api/content/"), strings.Contains(p, "/child/attachment"), strings.Contains(p, "/child/comment"), strings.Contains(p, "/label"), strings.Contains(p, "/property"):
			w.Write([]byte(`{"id":"1"}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
}
