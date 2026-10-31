package dashboard

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/status"
	"github.com/cmdlinezero/clz-lhd/internal/statushttp"
)

func TestLinesRenderLiveStatus(t *testing.T) {
	now := time.Date(2026, 10, 31, 6, 30, 0, 0, time.UTC)
	snapshot := statushttp.Response{
		At:       now.Add(-time.Second),
		HTTP:     statushttp.Listener{Address: "[::]:80", Listening: true},
		HTTPS:    statushttp.Listener{Address: "[::]:8443", Listening: true},
		InFlight: 2,
		Routes: []statushttp.Route{{
			Host: "app.localhost", Target: "http://localhost:3000", Reachability: status.ReachabilityConnected,
			Requests: 8, LastStatus: 200, LastDurationMS: 7.6,
		}},
		Traffic: []statushttp.TrafficBucket{{At: now, Requests: 4}},
	}

	output := strings.Join(Lines(snapshot, nil, 100, 30, now), "\n")
	for _, want := range []string{"CLZ LHD", "LIVE", "In flight: 2", "TRAFFIC", "app.localhost", "CONNECTED", "200 8ms", "updated 1s ago"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestLinesRenderDisconnectedState(t *testing.T) {
	output := strings.Join(Lines(statushttp.Response{}, errors.New("connection refused"), 80, 20, time.Now()), "\n")
	if !strings.Contains(output, "DISCONNECTED") || !strings.Contains(output, "connection refused") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestLinesRespectTerminalBounds(t *testing.T) {
	lines := Lines(statushttp.Response{}, errors.New("a deliberately long connection error"), 12, 3, time.Now())
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	for _, line := range lines {
		if len([]rune(line)) > 12 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
}
