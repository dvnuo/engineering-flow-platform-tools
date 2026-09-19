package testutil

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// Well-known ids and content served by MockNexus.
const (
	NexusComponentID    = "bWF2ZW4tcmVsZWFzZXM6Y29tcG9uZW50LTE"
	NexusAssetID        = "bWF2ZW4tcmVsZWFzZXM6YXNzZXQtMQ"
	NexusOffsiteAssetID = "b2Zmc2l0ZTphc3NldC0x"
	NexusArtifactPath   = "/repository/maven-releases/com/example/app/1.0.0/app-1.0.0.jar"
	NexusArtifactBody   = "jar-bytes-for-tests"
	NexusRESTPrefix     = "/service/rest/v1"
	NexusPageTwoToken   = "page-2-token"
)

// MockNexus is an httptest server that speaks enough of the Nexus Repository 3
// REST API v1 for the nexus CLI tests: repositories, paged search/list
// endpoints, component/asset metadata by id, and an asset download served by
// the same server. It requires an Authorization header unless Public is set,
// and always rejects requests when Reject is set.
type MockNexus struct {
	Server            *httptest.Server
	Public            bool
	Reject            bool
	Hits              int
	LastMethod        string
	LastPath          string
	LastQuery         url.Values
	LastAuthorization string
	Paths             []string
}

func NewMockNexus(t TestingT) *MockNexus {
	t.Helper()
	m := &MockNexus{}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.Server.Close)
	return m
}

// NexusArtifactSHA1 is the SHA-1 of NexusArtifactBody.
func NexusArtifactSHA1() string {
	sum := sha1.Sum([]byte(NexusArtifactBody))
	return hex.EncodeToString(sum[:])
}

func (m *MockNexus) handle(w http.ResponseWriter, r *http.Request) {
	m.Hits++
	m.LastMethod = r.Method
	m.LastPath = r.URL.Path
	m.LastQuery = r.URL.Query()
	m.LastAuthorization = r.Header.Get("Authorization")
	m.Paths = append(m.Paths, r.URL.Path)
	if r.Method != http.MethodGet {
		m.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"message": "read-only mock"})
		return
	}
	if m.Reject || (!m.Public && r.Header.Get("Authorization") == "") {
		m.writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "Authentication required"})
		return
	}
	p := r.URL.Path
	q := r.URL.Query()
	switch {
	case p == NexusRESTPrefix+"/repositories":
		m.writeJSON(w, http.StatusOK, m.repositories())
	case strings.HasPrefix(p, NexusRESTPrefix+"/repositories/"):
		name := strings.TrimPrefix(p, NexusRESTPrefix+"/repositories/")
		for _, repo := range m.repositories() {
			if repo["name"] == name {
				m.writeJSON(w, http.StatusOK, repo)
				return
			}
		}
		m.writeJSON(w, http.StatusNotFound, map[string]any{"message": "repository not found"})
	case p == NexusRESTPrefix+"/search":
		m.writePage(w, q.Get("continuationToken"), m.componentItems())
	case p == NexusRESTPrefix+"/search/assets":
		m.writePage(w, q.Get("continuationToken"), m.assetItems())
	case p == NexusRESTPrefix+"/components":
		if q.Get("repository") == "" {
			m.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"message": "repository is required"})
			return
		}
		m.writePage(w, q.Get("continuationToken"), m.componentItems())
	case strings.HasPrefix(p, NexusRESTPrefix+"/components/"):
		if strings.TrimPrefix(p, NexusRESTPrefix+"/components/") == NexusComponentID {
			m.writeJSON(w, http.StatusOK, m.componentItems()[0])
			return
		}
		m.writeJSON(w, http.StatusNotFound, map[string]any{"message": "component not found"})
	case p == NexusRESTPrefix+"/assets":
		if q.Get("repository") == "" {
			m.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"message": "repository is required"})
			return
		}
		m.writePage(w, q.Get("continuationToken"), m.assetItems())
	case strings.HasPrefix(p, NexusRESTPrefix+"/assets/"):
		switch strings.TrimPrefix(p, NexusRESTPrefix+"/assets/") {
		case NexusAssetID:
			m.writeJSON(w, http.StatusOK, m.assetItems()[0])
		case NexusOffsiteAssetID:
			asset := m.assetItems()[0]
			asset["id"] = NexusOffsiteAssetID
			asset["downloadUrl"] = "https://evil.example/repository/maven-releases/com/example/app/1.0.0/app-1.0.0.jar"
			m.writeJSON(w, http.StatusOK, asset)
		default:
			m.writeJSON(w, http.StatusNotFound, map[string]any{"message": "asset not found"})
		}
	case p == NexusArtifactPath:
		w.Header().Set("Content-Type", "application/java-archive")
		w.Header().Set("Content-Disposition", `attachment; filename="app-1.0.0.jar"`)
		_, _ = w.Write([]byte(NexusArtifactBody))
	case p == NexusRESTPrefix+"/status":
		w.WriteHeader(http.StatusOK)
	case p == NexusRESTPrefix+"/status/check":
		m.writeJSON(w, http.StatusOK, map[string]any{"File Blob Stores": map[string]any{"healthy": true, "message": "ok"}})
	default:
		m.writeJSON(w, http.StatusNotFound, map[string]any{"message": "not found: " + p})
	}
}

func (m *MockNexus) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writePage serves two pages: the first two items with a continuation token,
// then the remaining item without one.
func (m *MockNexus) writePage(w http.ResponseWriter, token string, items []map[string]any) {
	switch token {
	case "":
		m.writeJSON(w, http.StatusOK, map[string]any{"items": items[:2], "continuationToken": NexusPageTwoToken})
	case NexusPageTwoToken:
		m.writeJSON(w, http.StatusOK, map[string]any{"items": items[2:], "continuationToken": nil})
	default:
		m.writeJSON(w, http.StatusBadRequest, map[string]any{"message": "unknown continuation token"})
	}
}

func (m *MockNexus) repositories() []map[string]any {
	return []map[string]any{
		{"name": "maven-releases", "format": "maven2", "type": "hosted", "url": m.Server.URL + "/repository/maven-releases", "attributes": map[string]any{}},
		{"name": "npm-proxy", "format": "npm", "type": "proxy", "url": m.Server.URL + "/repository/npm-proxy", "attributes": map[string]any{"proxy": map[string]any{"remoteUrl": "https://registry.npmjs.org"}}},
		{"name": "docker-hosted", "format": "docker", "type": "hosted", "url": m.Server.URL + "/repository/docker-hosted", "attributes": map[string]any{}},
	}
}

func (m *MockNexus) assetItems() []map[string]any {
	return []map[string]any{
		{
			"id": NexusAssetID, "downloadUrl": m.Server.URL + NexusArtifactPath, "path": "com/example/app/1.0.0/app-1.0.0.jar",
			"repository": "maven-releases", "format": "maven2", "contentType": "application/java-archive", "fileSize": len(NexusArtifactBody),
			"checksum":     map[string]any{"sha1": NexusArtifactSHA1(), "md5": "d41d8cd98f00b204e9800998ecf8427e"},
			"lastModified": "2026-09-01T10:00:00.000+00:00", "blobCreated": "2026-09-01T10:00:00.000+00:00", "uploader": "ci-bot",
		},
		{
			"id": "bWF2ZW4tcmVsZWFzZXM6YXNzZXQtMg", "downloadUrl": m.Server.URL + "/repository/maven-releases/com/example/app/1.0.0/app-1.0.0.pom", "path": "com/example/app/1.0.0/app-1.0.0.pom",
			"repository": "maven-releases", "format": "maven2", "contentType": "application/xml", "fileSize": 812,
			"checksum": map[string]any{"sha1": "0000000000000000000000000000000000000002"},
		},
		{
			"id": "bWF2ZW4tcmVsZWFzZXM6YXNzZXQtMw", "downloadUrl": m.Server.URL + "/repository/maven-releases/com/example/app/1.1.0/app-1.1.0.jar", "path": "com/example/app/1.1.0/app-1.1.0.jar",
			"repository": "maven-releases", "format": "maven2", "contentType": "application/java-archive", "fileSize": 4096,
			"checksum": map[string]any{"sha1": "0000000000000000000000000000000000000003"},
		},
	}
}

func (m *MockNexus) componentItems() []map[string]any {
	assets := m.assetItems()
	return []map[string]any{
		{"id": NexusComponentID, "repository": "maven-releases", "format": "maven2", "group": "com.example", "name": "app", "version": "1.0.0", "assets": assets[:2]},
		{"id": "bWF2ZW4tcmVsZWFzZXM6Y29tcG9uZW50LTI", "repository": "maven-releases", "format": "maven2", "group": "com.example", "name": "app", "version": "1.1.0", "assets": assets[2:]},
		{"id": "bWF2ZW4tcmVsZWFzZXM6Y29tcG9uZW50LTM", "repository": "maven-releases", "format": "maven2", "group": "com.example", "name": "lib", "version": "2.0.0", "assets": []map[string]any{}},
	}
}
