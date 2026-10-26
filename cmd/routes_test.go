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
