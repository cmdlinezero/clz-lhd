package cmd

import (
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

func TestStartupSummary(t *testing.T) {
	routes, err := config.ParseRoutes([]string{
		"app.localhost=3000",
		"api.localhost=http://127.0.0.1:4000",
	})
	if err != nil {
		t.Fatalf("ParseRoutes() error = %v", err)
	}

	got := startupSummary("8088", routes)
	want := "Routes:\n" +
		"  http://app.localhost -> http://localhost:3000\n" +
		"  http://api.localhost -> http://127.0.0.1:4000\n" +
		"\n" +
		"Listening on :8088\n" +
		"Metrics: http://localhost:8088/metrics"

	if got != want {
		t.Fatalf("startupSummary() = %q, want %q", got, want)
	}
}
