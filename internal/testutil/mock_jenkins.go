package testutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

type TestingT interface {
	Helper()
	Cleanup(func())
	Fatal(args ...any)
}

type MockJenkins struct {
	Server       *httptest.Server
	Hits         int
	CrumbHits    int
	LastMethod   string
	LastPath     string
	LastBody     string
	LastParamRef string
}

func NewMockJenkins(t TestingT) *MockJenkins {
	t.Helper()
	m := &MockJenkins{}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.Server.Close)
	return m
}

func (m *MockJenkins) handle(w http.ResponseWriter, r *http.Request) {
	m.Hits++
	m.LastMethod = r.Method
	m.LastPath = r.URL.Path
	body, _ := io.ReadAll(r.Body)
	m.LastBody = string(body)
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/crumbIssuer/api/json" {
		m.CrumbHits++
		_, _ = w.Write([]byte(`{"crumbRequestField":"Jenkins-Crumb","crumb":"mock-crumb"}`))
		return
	}
	if r.Header.Get("Authorization") == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"auth required"}`))
		return
	}
	if r.Method != http.MethodGet && r.Header.Get("Jenkins-Crumb") != "mock-crumb" {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"missing crumb"}`))
		return
	}
	switch {
	case r.URL.Path == "/whoAmI/api/json":
		_, _ = w.Write([]byte(`{"authenticated":true,"name":"agent"}`))
	case r.URL.Path == "/api/json":
		_, _ = w.Write([]byte(`{"mode":"NORMAL","jobs":[{"name":"app-main","url":"` + m.Server.URL + `/job/app-main/"}],"views":[{"name":"All"}]}`))
	case strings.HasSuffix(r.URL.Path, "/build") || strings.HasSuffix(r.URL.Path, "/buildWithParameters"):
		values, _ := url.ParseQuery(m.LastBody)
		m.LastParamRef = values.Get("BRANCH")
		w.Header().Set("Location", m.Server.URL+"/queue/item/123/")
		w.WriteHeader(http.StatusCreated)
	case r.URL.Path == "/queue/api/json":
		_, _ = w.Write([]byte(`{"items":[{"id":123,"task":{"name":"app-main"}}]}`))
	case r.URL.Path == "/queue/item/123/api/json":
		_, _ = w.Write([]byte(`{"id":123,"executable":{"number":42,"url":"` + m.Server.URL + `/job/app-main/42/"}}`))
	case r.URL.Path == "/queue/cancelItem":
		_, _ = w.Write([]byte(`{"cancelled":true}`))
	case strings.HasSuffix(r.URL.Path, "/api/json") && strings.Contains(r.URL.Path, "/job/"):
		_, _ = w.Write([]byte(`{"name":"app-main","fullName":"folder/app-main","number":42,"building":false,"result":"SUCCESS","artifacts":[{"fileName":"app.jar","relativePath":"target/app.jar"}]}`))
	case strings.HasSuffix(r.URL.Path, "/config.xml"):
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<project/>`))
	case strings.HasSuffix(r.URL.Path, "/consoleText"):
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("line 1\nline 2\n"))
	case strings.HasSuffix(r.URL.Path, "/logText/progressiveText"):
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Text-Size", "7")
		w.Header().Set("X-More-Data", "false")
		_, _ = w.Write([]byte("chunk\n"))
	case strings.Contains(r.URL.Path, "/artifact/target/app.jar"):
		w.Header().Set("Content-Type", "application/java-archive")
		_, _ = w.Write([]byte("binary"))
	case strings.HasSuffix(r.URL.Path, "/wfapi/runs"):
		_, _ = w.Write([]byte(`[{"id":"42","name":"#42","status":"SUCCESS"}]`))
	case strings.HasSuffix(r.URL.Path, "/wfapi/describe"):
		_, _ = w.Write([]byte(`{"id":"42","status":"SUCCESS","stages":[{"id":"6","name":"Build"}]}`))
	case strings.HasSuffix(r.URL.Path, "/wfapi/log"):
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("stage log\n"))
	case strings.HasSuffix(r.URL.Path, "/wfapi/artifacts"):
		_, _ = w.Write([]byte(`[{"name":"app.jar","path":"target/app.jar"}]`))
	case r.URL.Path == "/pluginManager/api/json":
		_, _ = w.Write([]byte(`{"plugins":[{"shortName":"workflow-job","version":"1.0"}]}`))
	case r.URL.Path == "/computer/api/json":
		_, _ = w.Write([]byte(`{"computer":[{"displayName":"built-in"}]}`))
	case strings.HasPrefix(r.URL.Path, "/computer/"):
		_, _ = w.Write([]byte(`{"displayName":"built-in","offline":false}`))
	case strings.HasPrefix(r.URL.Path, "/view/") || r.URL.Path == "/createView":
		_, _ = w.Write([]byte(`{"name":"All"}`))
	case r.URL.Path == "/quietDown" || r.URL.Path == "/cancelQuietDown" || r.URL.Path == "/safeRestart":
		_, _ = w.Write([]byte(`{"ok":true}`))
	default:
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}
