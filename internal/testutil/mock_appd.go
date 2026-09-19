package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
)

// MockAppD is an httptest AppDynamics Controller: it issues OAuth tokens for
// one API client (client_id must carry the @account suffix), accepts that
// bearer token or one user@account basic login on every /controller/rest/
// GET, insists on output=JSON, and serves a small fixed application model.
// The issued token, client secret, and password reuse the shared Secrets
// canaries so tests can assert they never reach stdout.
type MockAppD struct {
	Server        *httptest.Server
	Account       string
	ClientName    string
	ClientSecret  string
	BasicUser     string
	BasicPassword string
	Token         string
	ExpiresIn     int

	Hits            int
	TokenHits       int
	LastMethod      string
	LastPath        string
	LastQuery       url.Values
	LastAuth        string
	LastClientID    string
	LastGrantType   string
	LastContentType string
}

func NewMockAppD(t TestingT) *MockAppD {
	t.Helper()
	m := &MockAppD{
		Account:       "customer1",
		ClientName:    "efp-reader",
		ClientSecret:  "secret-api-key-should-not-appear",
		BasicUser:     "reader",
		BasicPassword: "secret-password-should-not-appear",
		Token:         "secret-token-should-not-appear",
		ExpiresIn:     300,
	}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.Server.Close)
	return m
}

// AppDConfig renders a YAML config with one api_client instance for the mock.
func AppDConfig(base string) string {
	return fmt.Sprintf("appd:\n  default_instance: local\n  instances:\n    - name: local\n      base_url: %s\n      account: customer1\n      auth:\n        type: api_client\n        username: efp-reader\n        api_key: secret-api-key-should-not-appear\n", base)
}

// AppDBasicConfig renders a YAML config with one basic_password instance.
func AppDBasicConfig(base string) string {
	return fmt.Sprintf("appd:\n  default_instance: local\n  instances:\n    - name: local\n      base_url: %s\n      account: customer1\n      auth:\n        type: basic_password\n        username: reader\n        password: secret-password-should-not-appear\n", base)
}

func (m *MockAppD) handle(w http.ResponseWriter, r *http.Request) {
	m.Hits++
	m.LastMethod = r.Method
	m.LastPath = r.URL.Path
	m.LastQuery = r.URL.Query()
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/controller/api/oauth/access_token" {
		m.handleToken(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, `{"error":"read-only mock"}`)
		return
	}
	m.LastAuth = r.Header.Get("Authorization")
	if !m.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, `{"error":"unauthorized"}`)
		return
	}
	if r.URL.Query().Get("output") != "JSON" {
		writeJSON(w, http.StatusBadRequest, `{"error":"output=JSON expected"}`)
		return
	}
	p := r.URL.Path
	if p == "/controller/rest/applications" {
		writeJSON(w, http.StatusOK, `[{"id":1,"name":"ecommerce","description":"Web shop"},{"id":2,"name":"payments","description":""}]`)
		return
	}
	const prefix = "/controller/rest/applications/"
	if !strings.HasPrefix(p, prefix) {
		writeJSON(w, http.StatusNotFound, `{"error":"unknown path"}`)
		return
	}
	rest := strings.TrimPrefix(p, prefix)
	app, sub, _ := strings.Cut(rest, "/")
	if app != "ecommerce" && app != "1" && app != "payments" && app != "2" {
		writeJSON(w, http.StatusNotFound, `{"error":"application not found"}`)
		return
	}
	q := r.URL.Query()
	switch {
	case sub == "tiers":
		writeJSON(w, http.StatusOK, `[{"id":10,"name":"web","type":"Application Server","agentType":"APP_AGENT","numberOfNodes":2},{"id":11,"name":"inventory","type":"Application Server","agentType":"APP_AGENT","numberOfNodes":1}]`)
	case strings.HasPrefix(sub, "tiers/"):
		tier := strings.TrimPrefix(sub, "tiers/")
		tier, tierSub, _ := strings.Cut(tier, "/")
		if tier != "web" && tier != "10" {
			writeJSON(w, http.StatusNotFound, `{"error":"tier not found"}`)
			return
		}
		if tierSub == "nodes" {
			writeJSON(w, http.StatusOK, `[{"id":100,"name":"web-node-1","tierId":10,"tierName":"web","machineName":"ip-10-0-0-1","agentType":"APP_AGENT"},{"id":101,"name":"web-node-2","tierId":10,"tierName":"web","machineName":"ip-10-0-0-2","agentType":"APP_AGENT"}]`)
			return
		}
		writeJSON(w, http.StatusOK, `[{"id":10,"name":"web","type":"Application Server","agentType":"APP_AGENT","numberOfNodes":2}]`)
	case sub == "nodes":
		writeJSON(w, http.StatusOK, `[{"id":100,"name":"web-node-1","tierId":10,"tierName":"web","machineName":"ip-10-0-0-1","agentType":"APP_AGENT"},{"id":101,"name":"web-node-2","tierId":10,"tierName":"web","machineName":"ip-10-0-0-2","agentType":"APP_AGENT"},{"id":110,"name":"inventory-node-1","tierId":11,"tierName":"inventory","machineName":"ip-10-0-0-3","agentType":"APP_AGENT"}]`)
	case strings.HasPrefix(sub, "nodes/"):
		node := strings.TrimPrefix(sub, "nodes/")
		if node != "web-node-1" && node != "100" {
			writeJSON(w, http.StatusNotFound, `{"error":"node not found"}`)
			return
		}
		writeJSON(w, http.StatusOK, `[{"id":100,"name":"web-node-1","tierId":10,"tierName":"web","machineName":"ip-10-0-0-1","agentType":"APP_AGENT","appAgentVersion":"Server Agent v26.4.0"}]`)
	case sub == "business-transactions":
		writeJSON(w, http.StatusOK, `[{"id":200,"name":"/checkout","tierId":10,"tierName":"web","entryPointType":"SERVLET","internalName":"/checkout"},{"id":201,"name":"/inventory/lookup","tierId":11,"tierName":"inventory","entryPointType":"SERVLET","internalName":"/inventory/lookup"},{"id":202,"name":"/cart","tierId":10,"tierName":"web","entryPointType":"SERVLET","internalName":"/cart"}]`)
	case sub == "backends":
		writeJSON(w, http.StatusOK, `[{"id":300,"name":"db.example.test:5432","exitPointType":"JDBC","tierId":10,"applicationComponentNodeId":0},{"id":301,"name":"cache.example.test:6379","exitPointType":"CUSTOM","tierId":10}]`)
	case sub == "metrics":
		if q.Get("metric-path") == "" {
			writeJSON(w, http.StatusOK, `[{"name":"Application Infrastructure Performance","type":"folder"},{"name":"Business Transaction Performance","type":"folder"},{"name":"Overall Application Performance","type":"folder"}]`)
			return
		}
		writeJSON(w, http.StatusOK, `[{"name":"Average Response Time (ms)","type":"leaf"},{"name":"Calls per Minute","type":"leaf"},{"name":"Errors per Minute","type":"leaf"}]`)
	case sub == "metric-data-v2" || sub == "metric-data":
		if q.Get("metric-path") == "" || q.Get("time-range-type") == "" {
			writeJSON(w, http.StatusBadRequest, `{"error":"metric-path and time-range-type are required"}`)
			return
		}
		path, _ := json.Marshal(q.Get("metric-path"))
		writeJSON(w, http.StatusOK, `[{"metricId":123,"metricName":"BTM|BTs|BT:200|Component:10|Average Response Time (ms)","metricPath":`+string(path)+`,"frequency":"ONE_MIN","metricValues":[{"startTimeInMillis":1700000000000,"occurrences":1,"current":120,"min":80,"max":250,"useRange":true,"count":10,"sum":1200,"value":120,"standardDeviation":0}]}]`)
	case sub == "request-snapshots":
		if q.Get("time-range-type") == "" {
			writeJSON(w, http.StatusBadRequest, `{"error":"time-range-type is required"}`)
			return
		}
		writeJSON(w, http.StatusOK, m.snapshots(q))
	case sub == "problems/healthrule-violations":
		if q.Get("time-range-type") == "" {
			writeJSON(w, http.StatusBadRequest, `{"error":"time-range-type is required"}`)
			return
		}
		writeJSON(w, http.StatusOK, `[{"id":400,"name":"CPU utilization is too high","description":"","severity":"CRITICAL","status":"OPEN","startTimeInMillis":1700000000000,"endTimeInMillis":0,"detectedTimeInMillis":1700000060000,"affectedEntityDefinition":{"entityType":"APPLICATION_COMPONENT","name":"web","entityId":10},"triggeredEntityDefinition":{"entityType":"HEALTH_RULE","name":"CPU utilization is too high","entityId":41},"incidentStatus":"OPEN","deepLinkUrl":"`+m.Server.URL+`/controller/#/location=APP_INCIDENT_DETAIL&incident=400"}]`)
	case sub == "events":
		if q.Get("time-range-type") == "" || q.Get("event-types") == "" || q.Get("severities") == "" {
			writeJSON(w, http.StatusBadRequest, `{"error":"time-range-type, event-types, and severities are required"}`)
			return
		}
		writeJSON(w, http.StatusOK, m.events(q))
	default:
		writeJSON(w, http.StatusNotFound, `{"error":"unknown path"}`)
	}
}

func (m *MockAppD) handleToken(w http.ResponseWriter, r *http.Request) {
	m.TokenHits++
	m.LastContentType = r.Header.Get("Content-Type")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, `{"error":"POST required"}`)
		return
	}
	if !strings.HasPrefix(m.LastContentType, "application/x-www-form-urlencoded") {
		writeJSON(w, http.StatusBadRequest, `{"error":"form encoding required"}`)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, `{"error":"bad form"}`)
		return
	}
	m.LastGrantType = r.PostForm.Get("grant_type")
	m.LastClientID = r.PostForm.Get("client_id")
	if m.LastGrantType != "client_credentials" {
		writeJSON(w, http.StatusBadRequest, `{"error":"unsupported_grant_type"}`)
		return
	}
	if !strings.HasSuffix(m.LastClientID, "@"+m.Account) {
		writeJSON(w, http.StatusUnauthorized, `{"error":"invalid_client","error_description":"client_id must be <name>@<account>"}`)
		return
	}
	if m.LastClientID != m.ClientName+"@"+m.Account || r.PostForm.Get("client_secret") != m.ClientSecret {
		writeJSON(w, http.StatusUnauthorized, `{"error":"invalid_client","error_description":"bad client credentials"}`)
		return
	}
	writeJSON(w, http.StatusOK, `{"access_token":"`+m.Token+`","expires_in":`+strconv.Itoa(m.ExpiresIn)+`,"token_type":"Bearer"}`)
}

func (m *MockAppD) authorized(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "Bearer "+m.Token {
		return true
	}
	user, pass, ok := r.BasicAuth()
	return ok && user == m.BasicUser+"@"+m.Account && pass == m.BasicPassword
}

func (m *MockAppD) snapshots(q url.Values) string {
	all := []map[string]any{
		{
			"requestGUID": "4b9c6f2e-1d3a-4c7e-9f10-1a2b3c4d5e6f", "summary": "NullPointerException in CheckoutService", "userExperience": "ERROR",
			"timeTakenInMilliSecs": 1830, "businessTransactionId": 200, "applicationComponentId": 10, "applicationComponentNodeId": 100,
			"serverStartTime": 1700000000000, "localStartTime": 1700000000000, "errorOccured": true, "errorDetails": "java.lang.NullPointerException",
			"URL": "/checkout", "snapshotExitSequence": "|1|", "firstInChain": true, "archived": false, "callChain": "Component:10",
			"exitCalls":             []map[string]any{{"exitPointName": "JDBC", "toEntityId": 300, "toEntityType": "BACKEND", "timeTakenInMillis": 25, "detailString": "jdbc:postgresql://db.example.test:5432/shop"}},
			"httpParameters":        []map[string]any{{"name": "session", "value": "secret-token-should-not-appear"}},
			"transactionProperties": []any{}, "businessData": []any{}, "unresolvedCallInCallChain": false,
		},
		{
			"requestGUID": "9d8e7f6a-5b4c-4d3e-8f2a-1b2c3d4e5f60", "summary": "Slow checkout", "userExperience": "VERY_SLOW",
			"timeTakenInMilliSecs": 5200, "businessTransactionId": 200, "applicationComponentId": 10, "applicationComponentNodeId": 101,
			"serverStartTime": 1700000060000, "localStartTime": 1700000060000, "errorOccured": false, "errorDetails": nil,
			"URL": "/checkout", "snapshotExitSequence": "|1|2|", "firstInChain": true, "archived": false, "callChain": "Component:10",
			"exitCalls":      []any{},
			"httpParameters": []any{}, "transactionProperties": []any{}, "businessData": []any{}, "unresolvedCallInCallChain": false,
		},
		{
			"requestGUID": "c1d2e3f4-0a1b-4c2d-9e3f-4a5b6c7d8e9f", "summary": "Normal cart view", "userExperience": "NORMAL",
			"timeTakenInMilliSecs": 90, "businessTransactionId": 202, "applicationComponentId": 10, "applicationComponentNodeId": 100,
			"serverStartTime": 1700000120000, "localStartTime": 1700000120000, "errorOccured": false, "errorDetails": nil,
			"URL": "/cart", "snapshotExitSequence": "|1|", "firstInChain": true, "archived": false, "callChain": "Component:10",
			"exitCalls": []any{}, "httpParameters": []any{}, "transactionProperties": []any{}, "businessData": []any{}, "unresolvedCallInCallChain": false,
		},
	}
	guids := csvSet(q.Get("guids"))
	experiences := csvSet(q.Get("user-experience"))
	btIDs := csvSet(q.Get("business-transaction-ids"))
	errorsOnly := q.Get("error-occurred") == "true"
	max := len(all)
	if n, err := strconv.Atoi(q.Get("maximum-results")); err == nil && n > 0 && n < max {
		max = n
	}
	out := []map[string]any{}
	for _, s := range all {
		if len(guids) > 0 && !guids[s["requestGUID"].(string)] {
			continue
		}
		if len(experiences) > 0 && !experiences[s["userExperience"].(string)] {
			continue
		}
		if len(btIDs) > 0 && !btIDs[strconv.Itoa(s["businessTransactionId"].(int))] {
			continue
		}
		if errorsOnly && s["errorOccured"] != true {
			continue
		}
		out = append(out, s)
		if len(out) >= max {
			break
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func (m *MockAppD) events(q url.Values) string {
	all := []map[string]any{
		{"id": 500, "type": "APPLICATION_DEPLOYMENT", "severity": "INFO", "summary": "Deployed build 42 by ci-bot", "eventTime": 1700000000000, "affectedEntities": []map[string]any{{"entityType": "APPLICATION", "entityId": 1, "name": "ecommerce"}}, "details": "password=secret-password-should-not-appear"},
		{"id": 501, "type": "APPLICATION_ERROR", "severity": "ERROR", "summary": "java.lang.NullPointerException", "eventTime": 1700000030000, "affectedEntities": []map[string]any{{"entityType": "APPLICATION_COMPONENT_NODE", "entityId": 100, "name": "web-node-1"}}},
		{"id": 502, "type": "DIAGNOSTIC_SESSION", "severity": "WARN", "summary": "Diagnostic session started for /checkout", "eventTime": 1700000090000},
	}
	types := csvSet(q.Get("event-types"))
	severities := csvSet(q.Get("severities"))
	out := []map[string]any{}
	for _, e := range all {
		if !types[e["type"].(string)] || !severities[e["severity"].(string)] {
			continue
		}
		out = append(out, e)
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func csvSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out[part] = true
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
