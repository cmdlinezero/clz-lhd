package cmd

import (
	"encoding/json"
	"github.com/cmdlinezero/clz-lhd/internal/worktree"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func worktreeClient(t *testing.T, handler http.Handler) (routeAPIClient, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	return routeAPIClient{port: port, token: "secret", httpClient: &http.Client{Timeout: time.Second}}, srv.Close
}
func TestWorktreeEnvCreatesAndReuses(t *testing.T) {
	routes := []managedRoute{}
	posts := 0
	client, closeServer := worktreeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(struct {
				Routes []managedRoute `json:"routes"`
			}{routes})
		case http.MethodPost:
			posts++
			var item managedRoute
			if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
				t.Error(err)
			}
			item.Source = "runtime"
			routes = append(routes, item)
			w.WriteHeader(204)
		}
	}))
	defer closeServer()
	info := worktree.Info{Root: "/tmp/repo-feature", Host: "feature.repo.localhost"}
	first, err := prepareWorktreeEnv(client, info, func(int) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareWorktreeEnv(client, info, func(int) bool { return false })
	if err != nil || first != second || posts != 1 {
		t.Fatalf("reuse: %q %q posts=%d err=%v", first, second, posts, err)
	}
	if !strings.Contains(first, "export LHD_HOST='feature.repo.localhost'") {
		t.Fatal(first)
	}
}
func TestWorktreeEnvConflictAndNoPartialOutput(t *testing.T) {
	client, closeServer := worktreeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"routes": []managedRoute{{Host: "x.repo.localhost", Target: "http://127.0.0.1:4000", Source: "config"}}})
			return
		}
		w.WriteHeader(http.StatusConflict)
	}))
	defer closeServer()
	_, err := prepareWorktreeEnv(client, worktree.Info{Root: "/x", Host: "x.repo.localhost"}, func(int) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "configuration") {
		t.Fatal(err)
	}
	_, err = prepareWorktreeEnv(client, worktree.Info{Root: "/x", Host: "other.repo.localhost"}, func(int) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "no environment exported") {
		t.Fatal(err)
	}
	client.token = ""
	_, err = prepareWorktreeEnv(client, worktree.Info{Root: "/x", Host: "other.repo.localhost"}, func(int) bool { return true })
	if err == nil {
		t.Fatal("missing token accepted")
	}
}
func TestWorktreeEnvRefusesNoncanonicalExistingRoute(t *testing.T) {
	client, closeServer := worktreeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected mutation: %s", r.Method)
			w.WriteHeader(http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"routes": []managedRoute{{Host: "feature.repo.localhost", Target: "http://localhost:23456", Source: "runtime"}}})
	}))
	defer closeServer()
	_, err := prepareWorktreeEnv(client, worktree.Info{Root: "/tmp/feature", Host: "feature.repo.localhost"}, func(int) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("expected canonical target refusal, got %v", err)
	}
}

func TestWorktreeShellQuote(t *testing.T) {
	if got := shellQuote("a'b"); got != "'a'\\''b'" {
		t.Fatal(got)
	}
}
