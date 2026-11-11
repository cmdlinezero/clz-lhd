package cmd

import (
	"net/url"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

func TestRoutesSummarySortsByHost(t *testing.T) {
	routes := []config.Route{
		{Host: "docs.localhost", Target: mustURL(t, "http://localhost:5173")},
		{Host: "api.localhost", Target: mustURL(t, "http://localhost:8080")},
		{Host: "app.localhost", Target: mustURL(t, "http://localhost:3000")},
	}

	got := routesSummary(routes)
	want := "HOST\tTARGET\n" +
		"api.localhost\thttp://localhost:8080\n" +
		"app.localhost\thttp://localhost:3000\n" +
		"docs.localhost\thttp://localhost:5173\n"

	if got != want {
		t.Fatalf("routesSummary() = %q, want %q", got, want)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", raw, err)
	}
	return u
}

func TestLiveRoutesSummaryIncludesAllOwners(t *testing.T) {
	data := []byte(`{"routes":[{"host":"work.localhost","target":"http://127.0.0.1:20000","source":"runtime","owner":"worktree"},{"host":"agent.localhost","target":"http://127.0.0.1:34567","source":"runtime","owner":"agent"},{"host":"config.localhost","target":"http://localhost:3000","source":"config"},{"host":"manual.localhost","target":"http://localhost:4000","source":"runtime","owner":"manual"}]}`)
	got, err := liveRoutesSummary(data)
	if err != nil {
		t.Fatal(err)
	}
	want := "HOST\tTARGET\tSOURCE\tOWNER\n" +
		"agent.localhost\thttp://127.0.0.1:34567\truntime\tagent\n" +
		"config.localhost\thttp://localhost:3000\tconfig\t-\n" +
		"manual.localhost\thttp://localhost:4000\truntime\tmanual\n" +
		"work.localhost\thttp://127.0.0.1:20000\truntime\tworktree\n"
	if got != want {
		t.Fatalf("live routes = %q, want %q", got, want)
	}
	if _, err := liveRoutesSummary([]byte("broken")); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
