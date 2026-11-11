package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/worktree"
)

func TestWorktreeRoutesFiltersAndSorts(t *testing.T) {
	routes := []managedRoute{
		{Host: "z-123456789a.repo.localhost", Target: "http://127.0.0.1:23456", Source: "runtime"},
		{Host: "a-123456789a.repo.localhost", Target: "http://localhost:23457", Source: "runtime"},
		{Host: "b-123456789a.repo.localhost", Target: "http://127.0.0.1:23458", Source: "config"},
		{Host: "ordinary.localhost", Target: "http://127.0.0.1:23459", Source: "runtime"},
		{Host: "x-123456789a.repo.localhost", Target: "http://remote:23460", Source: "runtime"},
	}
	got := worktreeRoutes(routes)
	if len(got) != 2 || got[0].Host != "a-123456789a.repo.localhost" || got[1].Host != "z-123456789a.repo.localhost" {
		t.Fatalf("unexpected routes: %+v", got)
	}
}

func TestRemoveWorktreeRoute(t *testing.T) {
	const host = "feature-123456789a.repo.localhost"
	info := worktree.Info{Host: host}
	for _, tc := range []struct {
		name        string
		routes      []managedRoute
		wantRemoved bool
		wantError   string
		wantDeletes int
	}{
		{name: "success", routes: []managedRoute{{Host: host, Target: "http://127.0.0.1:23456", Source: "runtime"}}, wantRemoved: true, wantDeletes: 1},
		{name: "absent"},
		{name: "yaml protected", routes: []managedRoute{{Host: host, Target: "http://127.0.0.1:23456", Source: "config"}}, wantError: "configuration"},
		{name: "non loopback protected", routes: []managedRoute{{Host: host, Target: "http://example.com:23456", Source: "runtime"}}, wantError: "loopback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deletes := 0
			client, closeServer := worktreeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(map[string]any{"routes": tc.routes})
					return
				}
				if r.Method == http.MethodDelete {
					deletes++
					if r.URL.Path != "/v1/routes/"+host || r.Header.Get("Authorization") != "Bearer secret" {
						t.Errorf("unexpected delete: %s %s", r.URL.Path, r.Header.Get("Authorization"))
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
				t.Errorf("unexpected method %s", r.Method)
			}))
			defer closeServer()
			removed, err := removeWorktreeRoute(client, info)
			if removed != tc.wantRemoved || deletes != tc.wantDeletes {
				t.Fatalf("removed=%v deletes=%d err=%v", removed, deletes, err)
			}
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected %q, got %v", tc.wantError, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRemoveWorktreeRouteRequiresToken(t *testing.T) {
	client := routeAPIClient{}
	if _, err := removeWorktreeRoute(client, worktree.Info{Host: "a-123456789a.repo.localhost"}); err == nil {
		t.Fatal("expected token error")
	}
}
