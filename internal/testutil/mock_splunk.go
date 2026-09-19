package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// MockSplunkSessionKey is the session key the mock issues on login. Tests
// assert it never reaches stdout.
const MockSplunkSessionKey = "secret-session-key-should-not-appear"

// MockSplunk emulates the subset of the Splunk Enterprise management REST API
// the splunk CLI uses. Every request must carry output_mode=json and either
// `Authorization: Bearer <Token>` or `Authorization: Splunk <SessionKey>`
// (obtained from POST /services/auth/login with Username/Password).
type MockSplunk struct {
	Server *httptest.Server

	Token, Username, Password, SessionKey string

	// StatusCallsUntilDone is how many status reads return RUNNING before a
	// job reports DONE; NeverDone keeps every job RUNNING and FailJob makes
	// every job report isFailed.
	StatusCallsUntilDone int
	NeverDone            bool
	FailJob              bool

	// Results and Fields are served by oneshot searches and the results
	// endpoints (paged by count/offset, filtered by repeated f).
	Results []map[string]any
	Fields  []string

	Hits, LoginHits, OutputModeMissing int
	LastMethod, LastPath, LastAuth     string
	LastForm, LastQuery                url.Values
	JobsCreated                        []url.Values
	Dispatches                         map[string]url.Values
	Controls                           []string

	mu          sync.Mutex
	statusCalls map[string]int
}

var mockSavedSearches = []map[string]any{
	{"name": "Errors last hour", "search": "index=main error | head 100", "disabled": false, "description": "Recent application errors", "is_scheduled": false, "cron_schedule": ""},
	{"name": "Dump to lookup", "search": "index=main | outputlookup dump.csv", "disabled": false, "description": "Writes a lookup", "is_scheduled": false, "cron_schedule": ""},
	{"name": "Nightly summary", "search": "index=main | stats count by host", "disabled": false, "description": "Scheduled host summary", "is_scheduled": true, "cron_schedule": "0 1 * * *"},
}

func NewMockSplunk(t TestingT) *MockSplunk {
	t.Helper()
	m := &MockSplunk{
		Token:                "secret-token-should-not-appear",
		Username:             "agent",
		Password:             "secret-password-should-not-appear",
		SessionKey:           MockSplunkSessionKey,
		StatusCallsUntilDone: 1,
		Results: []map[string]any{
			{"_time": "2026-09-19T10:00:00.000+08:00", "host": "web-1", "source": "/var/log/app.log", "_raw": "2026-09-19 10:00:00 ERROR checkout failed order=1001"},
			{"_time": "2026-09-19T10:01:00.000+08:00", "host": "web-2", "source": "/var/log/app.log", "_raw": "2026-09-19 10:01:00 ERROR checkout failed order=1002"},
			{"_time": "2026-09-19T10:02:00.000+08:00", "host": "web-1", "source": "/var/log/app.log", "_raw": "2026-09-19 10:02:00 ERROR checkout failed order=1003"},
		},
		Fields:      []string{"_time", "host", "source", "_raw"},
		Dispatches:  map[string]url.Values{},
		statusCalls: map[string]int{},
	}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.Server.Close)
	return m
}

func (m *MockSplunk) writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (m *MockSplunk) writeError(w http.ResponseWriter, status int, kind, text string) {
	m.writeJSON(w, status, map[string]any{"messages": []map[string]any{{"type": kind, "text": text}}})
}

func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = append([]string{}, vs...)
	}
	return out
}

func (m *MockSplunk) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Hits++
	m.LastMethod = r.Method
	m.LastPath = r.URL.Path
	m.LastAuth = r.Header.Get("Authorization")
	_ = r.ParseForm()
	m.LastForm = cloneValues(r.PostForm)
	m.LastQuery = cloneValues(r.URL.Query())
	w.Header().Set("Content-Type", "application/json")
	if r.Form.Get("output_mode") != "json" {
		m.OutputModeMissing++
		m.writeError(w, http.StatusBadRequest, "ERROR", "output_mode=json is required by the mock")
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/services/auth/login" {
		m.LoginHits++
		if r.PostForm.Get("username") == m.Username && r.PostForm.Get("password") == m.Password {
			m.writeJSON(w, http.StatusOK, map[string]any{"sessionKey": m.SessionKey, "message": "", "code": ""})
			return
		}
		m.writeError(w, http.StatusUnauthorized, "WARN", "Login failed")
		return
	}
	if m.LastAuth != "Bearer "+m.Token && m.LastAuth != "Splunk "+m.SessionKey {
		m.writeError(w, http.StatusUnauthorized, "ERROR", "call not properly authenticated")
		return
	}
	path := r.URL.Path
	switch {
	case path == "/services/authentication/current-context":
		m.writeJSON(w, http.StatusOK, map[string]any{"entry": []map[string]any{{"name": "context", "content": map[string]any{"username": "agent", "roles": []string{"user", "power"}}}}})
	case path == "/services/search/jobs" && r.Method == http.MethodPost:
		form := cloneValues(r.PostForm)
		m.JobsCreated = append(m.JobsCreated, form)
		if form.Get("exec_mode") == "oneshot" {
			m.writeJSON(w, http.StatusOK, map[string]any{"preview": false, "init_offset": 0, "messages": []any{}, "fields": m.fieldList(), "results": m.Results})
			return
		}
		m.writeJSON(w, http.StatusCreated, map[string]any{"sid": fmt.Sprintf("1700000000.%d", len(m.JobsCreated))})
	case strings.HasPrefix(path, "/services/search/jobs/"):
		parts := strings.Split(strings.TrimPrefix(path, "/services/search/jobs/"), "/")
		sid, _ := url.PathUnescape(parts[0])
		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			m.writeJSON(w, http.StatusOK, map[string]any{"entry": []map[string]any{{"name": sid, "content": m.jobContent(sid)}}})
		case len(parts) == 1 && r.Method == http.MethodDelete:
			m.Controls = append(m.Controls, sid+":delete")
			m.writeJSON(w, http.StatusOK, map[string]any{"messages": []any{}})
		case len(parts) == 2 && r.Method == http.MethodPost && parts[1] == "control":
			m.Controls = append(m.Controls, sid+":"+r.PostForm.Get("action"))
			m.writeJSON(w, http.StatusOK, map[string]any{"messages": []map[string]any{{"type": "INFO", "text": "Search job " + r.PostForm.Get("action") + "led."}}})
		case len(parts) == 2 && r.Method == http.MethodGet && (parts[1] == "results" || parts[1] == "results_preview" || parts[1] == "events"):
			m.writeJSON(w, http.StatusOK, m.pagedResults(r.URL.Query(), parts[1] != "results"))
		default:
			m.writeError(w, http.StatusNotFound, "ERROR", "Not Found")
		}
	case path == "/services/saved/searches" && r.Method == http.MethodGet:
		filter := strings.ToLower(r.URL.Query().Get("search"))
		entries := []map[string]any{}
		for _, item := range mockSavedSearches {
			name, _ := item["name"].(string)
			if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
				continue
			}
			entries = append(entries, savedEntry(item))
		}
		m.writeJSON(w, http.StatusOK, map[string]any{"entry": entries, "paging": map[string]any{"total": len(entries)}})
	case strings.HasPrefix(path, "/services/saved/searches/"):
		rest := strings.TrimPrefix(path, "/services/saved/searches/")
		dispatch := strings.HasSuffix(rest, "/dispatch")
		name, _ := url.PathUnescape(strings.TrimSuffix(rest, "/dispatch"))
		var found map[string]any
		for _, item := range mockSavedSearches {
			if item["name"] == name {
				found = item
			}
		}
		if found == nil {
			m.writeError(w, http.StatusNotFound, "ERROR", "Could not find object id="+name)
			return
		}
		if dispatch && r.Method == http.MethodPost {
			m.Dispatches[name] = cloneValues(r.PostForm)
			m.writeJSON(w, http.StatusCreated, map[string]any{"sid": fmt.Sprintf("saved-1700000000.%d", len(m.Dispatches))})
			return
		}
		m.writeJSON(w, http.StatusOK, map[string]any{"entry": []map[string]any{savedEntry(found)}})
	case path == "/services/data/indexes":
		m.writeJSON(w, http.StatusOK, map[string]any{"entry": []map[string]any{
			{"name": "main", "content": map[string]any{"totalEventCount": 123456, "maxTime": "2026-09-19T10:02:00+08:00", "minTime": "2026-01-01T00:00:00+08:00", "disabled": false}},
			{"name": "_internal", "content": map[string]any{"totalEventCount": 999, "maxTime": "2026-09-19T10:02:00+08:00", "minTime": "2026-09-01T00:00:00+08:00", "disabled": false}},
		}})
	case path == "/services/server/info":
		m.writeJSON(w, http.StatusOK, map[string]any{"entry": []map[string]any{{"name": "server-info", "content": map[string]any{"version": "9.4.0", "serverName": "splunk-mock"}}}})
	default:
		m.writeError(w, http.StatusNotFound, "ERROR", "Not Found")
	}
}

func savedEntry(item map[string]any) map[string]any {
	content := map[string]any{}
	for k, v := range item {
		if k != "name" {
			content[k] = v
		}
	}
	return map[string]any{"name": item["name"], "content": content}
}

func (m *MockSplunk) fieldList() []map[string]any {
	out := make([]map[string]any, 0, len(m.Fields))
	for _, f := range m.Fields {
		out = append(out, map[string]any{"name": f})
	}
	return out
}

func (m *MockSplunk) jobContent(sid string) map[string]any {
	m.statusCalls[sid]++
	calls := m.statusCalls[sid]
	content := map[string]any{"sid": sid, "search": "search index=main error", "earliestTime": "2026-09-19T09:00:00.000+08:00", "latestTime": "2026-09-19T10:00:00.000+08:00", "ttl": 600, "isFinalized": false, "isPreviewEnabled": false}
	switch {
	case m.FailJob:
		content["isDone"] = true
		content["isFailed"] = true
		content["dispatchState"] = "FAILED"
		content["doneProgress"] = 1
		content["eventCount"] = 0
		content["resultCount"] = 0
		content["scanCount"] = 0
		content["messages"] = []map[string]any{{"type": "ERROR", "text": "Error in 'search': Unable to parse the search"}}
	case m.NeverDone || calls <= m.StatusCallsUntilDone:
		content["isDone"] = false
		content["isFailed"] = false
		content["dispatchState"] = "RUNNING"
		content["doneProgress"] = 0.5
		content["eventCount"] = 1
		content["resultCount"] = 1
		content["scanCount"] = 10
		content["messages"] = []any{}
	default:
		content["isDone"] = true
		content["isFailed"] = false
		content["dispatchState"] = "DONE"
		content["doneProgress"] = 1
		content["eventCount"] = len(m.Results)
		content["resultCount"] = len(m.Results)
		content["scanCount"] = 42
		content["runDuration"] = 0.5
		content["messages"] = []map[string]any{{"type": "INFO", "text": "Your timerange was substituted based on your search string"}}
	}
	return content
}

func (m *MockSplunk) pagedResults(q url.Values, preview bool) map[string]any {
	count, _ := strconv.Atoi(q.Get("count"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	fields := q["f"]
	rows := []map[string]any{}
	for i, row := range m.Results {
		if i < offset {
			continue
		}
		if count > 0 && len(rows) >= count {
			break
		}
		if len(fields) == 0 {
			rows = append(rows, row)
			continue
		}
		kept := map[string]any{}
		for _, f := range fields {
			if v, ok := row[f]; ok {
				kept[f] = v
			}
		}
		rows = append(rows, kept)
	}
	fieldList := m.fieldList()
	if len(fields) > 0 {
		fieldList = []map[string]any{}
		for _, f := range fields {
			fieldList = append(fieldList, map[string]any{"name": f})
		}
	}
	return map[string]any{"preview": preview, "init_offset": offset, "messages": []any{}, "fields": fieldList, "results": rows}
}
