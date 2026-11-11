package statushttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/cmdlinezero/clz-lhd/internal/status"
)

func TestManagementMutationsAndStatus(t *testing.T) {
	configured, err := config.ParseRoute("fixed.localhost=http://127.0.0.1:1111")
	if err != nil {
		t.Fatal(err)
	}
	registry := proxy.NewRouteRegistry([]config.Route{configured})
	model := status.New([]config.Route{configured})
	handler := NewManagementHandler(model, proxy.NewLifecycleRecorder(4), registry, "secret")
	call := func(method, path, body, token, origin string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "http://127.0.0.1:9099"+path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	body := `{"host":"temp.localhost","target":"http://127.0.0.1:2222"}`
	if w := call("POST", "/v1/routes", body, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: %d", w.Code)
	}
	if w := call("POST", "/v1/routes", body, "secret", "http://evil.example"); w.Code != http.StatusForbidden {
		t.Fatalf("origin: %d", w.Code)
	}
	if w := call("POST", "/v1/routes", body, "secret", ""); w.Code != http.StatusNoContent {
		t.Fatalf("create: %d: %s", w.Code, w.Body.String())
	}
	if route, ok := registry.Lookup("temp.localhost"); !ok || route.Target.Port() != "2222" {
		t.Fatalf("route: %+v, %v", route, ok)
	}
	if got := model.Snapshot().Routes; len(got) != 2 {
		t.Fatalf("status routes: %+v", got)
	}
	if w := call("POST", "/v1/routes", body, "secret", ""); w.Code != http.StatusConflict {
		t.Fatalf("duplicate: %d", w.Code)
	}
	if w := call("DELETE", "/v1/routes/fixed.localhost", "", "secret", ""); w.Code != http.StatusConflict {
		t.Fatalf("configured: %d", w.Code)
	}
	if w := call("PUT", "/v1/routes", `{"host":"temp.localhost","target":"http://127.0.0.1:3333"}`, "secret", ""); w.Code != http.StatusNoContent {
		t.Fatalf("update: %d: %s", w.Code, w.Body.String())
	}
	if route, _ := registry.Lookup("temp.localhost"); route.Target.Port() != "3333" {
		t.Fatalf("updated route: %+v", route)
	}
	w := call("GET", "/v1/routes", "", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	var listed struct {
		Routes []struct {
			Host   string `json:"host"`
			Source string `json:"source"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil || len(listed.Routes) != 2 || listed.Routes[0].Source != "config" || listed.Routes[1].Source != "runtime" {
		t.Fatalf("list: %+v %v", listed, err)
	}
	if w := call("DELETE", "/v1/routes/temp.localhost", "", "secret", ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	if _, ok := registry.Lookup("temp.localhost"); ok {
		t.Fatal("deleted route still present")
	}
	if got := model.Snapshot().Routes; len(got) != 1 {
		t.Fatalf("status after delete: %+v", got)
	}
}

func TestManagementDisabledWithoutToken(t *testing.T) {
	handler := NewManagementHandler(status.New(nil), proxy.NewLifecycleRecorder(1), proxy.NewRouteRegistry(nil), "")
	req := httptest.NewRequest("POST", "http://127.0.0.1:9099/v1/routes", strings.NewReader(`{"host":"x.localhost","target":"http://localhost:3000"}`))
	req.Header.Set("Authorization", "Bearer anything")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("disabled mutation: %d", w.Code)
	}
}

func TestManagementOwnershipIsolation(t *testing.T) {
	registry := proxy.NewRouteRegistry(nil)
	handler := NewManagementHandler(status.New(nil), proxy.NewLifecycleRecorder(1), registry, "secret")
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://127.0.0.1:9099"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret")
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	const owned = `{"host":"branch.localhost","target":"http://127.0.0.1:23456","owner":"worktree"}`
	if w := call("POST", "/v1/routes", owned); w.Code != 204 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w := call("PUT", "/v1/routes", `{"host":"branch.localhost","target":"http://127.0.0.1:23457"}`); w.Code != 409 {
		t.Fatalf("cross-owner update: %d", w.Code)
	}
	if w := call("DELETE", "/v1/routes/branch.localhost", ""); w.Code != 409 {
		t.Fatalf("cross-owner delete: %d", w.Code)
	}
	if w := call("GET", "/v1/routes", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"owner":"worktree"`) {
		t.Fatalf("owner listing: %d %s", w.Code, w.Body.String())
	}
	if w := call("DELETE", "/v1/routes/branch.localhost?owner=worktree", ""); w.Code != 204 {
		t.Fatalf("owned delete: %d %s", w.Code, w.Body.String())
	}
	if w := call("POST", "/v1/routes", `{"host":"branch.localhost","target":"http://127.0.0.1:23456","owner":"alien"}`); w.Code != 400 {
		t.Fatalf("invalid owner: %d", w.Code)
	}
}
