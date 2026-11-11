package statushttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/cmdlinezero/clz-lhd/internal/status"
)

func TestAgentAuthorizationBoundary(t *testing.T) {
	registry := proxy.NewRouteRegistry(nil)
	h := NewScopedManagementHandler(status.New(nil), proxy.NewLifecycleRecorder(1), registry, "admin-secret", "agent-secret")
	call := func(method, path, body, credential string) int {
		t.Helper()
		r := httptest.NewRequest(method, "http://127.0.0.1:9099"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+credential)
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	good := `{"host":"tool.localhost","target":"http://127.0.0.1:23000","owner":"agent"}`
	if got := call("POST", "/v1/routes", good, "agent-secret"); got != 204 {
		t.Fatalf("create: %d", got)
	}
	for _, body := range []string{
		`{"host":"manual.localhost","target":"http://127.0.0.1:23001","owner":"manual"}`,
		`{"host":"bad.localhost","target":"http://example.com:3000","owner":"agent"}`,
		`{"host":"bad.localhost","target":"http://localhost:3000","owner":"agent"}`,
		`{"host":"bad.localhost","target":"http://127.0.0.1:80","owner":"agent"}`,
		`{"host":"bad.example","target":"http://127.0.0.1:23000","owner":"agent"}`,
	} {
		if got := call("POST", "/v1/routes", body, "agent-secret"); got != 403 {
			t.Fatalf("unexpected status %d for %s", got, body)
		}
	}
	if got := call("DELETE", "/v1/routes/tool.localhost?owner=manual", "", "agent-secret"); got != 403 {
		t.Fatalf("delete wrong owner: %d", got)
	}
	if got := call("PUT", "/v1/routes", `{"host":"tool.localhost","target":"http://127.0.0.1:23002","owner":"manual"}`, "agent-secret"); got != 403 {
		t.Fatalf("update wrong owner: %d", got)
	}
	if got := call("DELETE", "/v1/routes/tool.localhost?owner=agent", "", "wrong"); got != 401 {
		t.Fatalf("bad credential: %d", got)
	}
	if got := call("DELETE", "/v1/routes/tool.localhost?owner=agent", "", "agent-secret"); got != 204 {
		t.Fatalf("delete: %d", got)
	}
	if got := call("POST", "/v1/routes", good, "admin-secret"); got != 204 {
		t.Fatalf("admin create: %d", got)
	}
}

func TestAgentCredentialDisabledWhenSameAsAdmin(t *testing.T) {
	h := NewScopedManagementHandler(status.New(nil), proxy.NewLifecycleRecorder(1), proxy.NewRouteRegistry(nil), "same", "same")
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9099/v1/routes", strings.NewReader(`{"host":"x.localhost","target":"http://127.0.0.1:23000","owner":"agent"}`))
	r.Header.Set("Authorization", "Bearer same")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	// The administrator credential retains its normal privileges, but no
	// independently authenticated agent identity is enabled.
	if w.Code != 204 {
		t.Fatalf("admin access: %d", w.Code)
	}
}
