package dashboard

import (
	"errors"
	"fmt"
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

	output := strings.Join(Lines(snapshot, nil, "app.localhost", 100, 40, now), "\n")
	for _, want := range []string{"CLZ LHD", "LIVE", "In flight: 2", "TRAFFIC", "app.localhost", "● CONNECTED", "200 8ms", "updated 1s ago"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestLinesRenderDisconnectedState(t *testing.T) {
	output := strings.Join(Lines(statushttp.Response{}, errors.New("connection refused"), "", 80, 20, time.Now()), "\n")
	if !strings.Contains(output, "DISCONNECTED") || !strings.Contains(output, "connection refused") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestLinesRespectTerminalBounds(t *testing.T) {
	lines := Lines(statushttp.Response{}, errors.New("a deliberately long connection error"), "", 12, 3, time.Now())
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	for _, line := range lines {
		if len([]rune(line)) > 12 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
}

func TestGraphUsesReadableLowTrafficScale(t *testing.T) {
	lines := graph([]statushttp.TrafficBucket{{Requests: 1}}, 80)
	joined := strings.Join(lines, "\n")
	if strings.Count(joined, "  1 ┤") != 1 {
		t.Fatalf("expected a single peak label for one request/sec:\n%s", joined)
	}
	if !strings.Contains(joined, "█") {
		t.Fatalf("expected traffic to be visible:\n%s", joined)
	}
}

func TestRouteStatesHaveDistinctMarkers(t *testing.T) {
	cases := []struct {
		state status.Reachability
		want  string
	}{
		{status.ReachabilityConnected, "● CONNECTED"},
		{status.ReachabilityRefused, "× REFUSED"},
		{status.ReachabilityTimeout, "! TIMEOUT"},
		{status.ReachabilityUnknown, "○ UNKNOWN"},
	}
	for _, tc := range cases {
		if got := routeState(tc.state); got != tc.want {
			t.Fatalf("routeState(%q) = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestRouteLineUsesCompactLayoutOnNarrowTerminal(t *testing.T) {
	route := statushttp.Route{Host: "app.localhost", Target: "http://localhost:3000", Reachability: status.ReachabilityConnected, Requests: 12, LastStatus: 200, LastDurationMS: 4}
	line := routeLine(route, false, 60)
	if strings.Contains(line, "12 req") || !strings.Contains(line, "● CONNECTED") || !strings.Contains(line, "200 4ms") {
		t.Fatalf("unexpected compact route line: %q", line)
	}
}

func TestSelectionUsesVimStyleMovement(t *testing.T) {
	routes := []statushttp.Route{{Host: "a.localhost"}, {Host: "b.localhost"}, {Host: "c.localhost"}}
	var selection Selection
	selection.Normalize(routes)
	if selection.Host != "a.localhost" {
		t.Fatalf("initial selection = %q", selection.Host)
	}
	selection.Move(routes, 1)
	if selection.Host != "b.localhost" {
		t.Fatalf("next selection = %q", selection.Host)
	}
	selection.Last(routes)
	if selection.Host != "c.localhost" {
		t.Fatalf("last selection = %q", selection.Host)
	}
	selection.First(routes)
	selection.Move(routes, -1)
	if selection.Host != "a.localhost" {
		t.Fatalf("selection should clamp at first: %q", selection.Host)
	}
}

func TestLinesRenderSelectedRouteDetail(t *testing.T) {
	now := time.Date(2026, 10, 31, 7, 0, 10, 0, time.UTC)
	snapshot := statushttp.Response{At: now, Routes: []statushttp.Route{{
		Host: "app.localhost", Target: "http://localhost:3000", Reachability: status.ReachabilityConnected,
		Requests: 12, LastStatus: 200, LastDurationMS: 4, LastRequest: now.Add(-3 * time.Second),
	}}}
	output := strings.Join(Lines(snapshot, nil, "app.localhost", 100, 40, now), "\n")
	for _, want := range []string{"> ● CONNECTED", "ROUTE DETAIL", "Target        http://localhost:3000", "Last request  200 · 4ms · 3s ago", "j/k [ navigate ]    g/G [ first/last ]   q [ quit ]"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestLinesKeepSelectedRouteAndControlsVisibleOnShortTerminal(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 30, 0, 0, time.UTC)
	routes := make([]statushttp.Route, 12)
	for i := range routes {
		routes[i] = statushttp.Route{
			Host:         fmt.Sprintf("route-%02d.localhost", i),
			Target:       fmt.Sprintf("http://localhost:%d", 3000+i),
			Reachability: status.ReachabilityConnected,
		}
	}
	snapshot := statushttp.Response{At: now, Routes: routes}
	output := strings.Join(Lines(snapshot, nil, "route-10.localhost", 100, 20, now), "\n")
	for _, want := range []string{"> ● CONNECTED route-10.localhost", "ROUTE DETAIL", "Host          route-10.localhost", "q [ quit ]"} {
		if !strings.Contains(output, want) {
			t.Fatalf("short dashboard missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "TRAFFIC —") {
		t.Fatalf("short dashboard should prioritize route detail over traffic graph:\n%s", output)
	}
}

func TestVisibleRoutesKeepsSelectionInViewport(t *testing.T) {
	routes := []statushttp.Route{
		{Host: "a.localhost"}, {Host: "b.localhost"}, {Host: "c.localhost"},
		{Host: "d.localhost"}, {Host: "e.localhost"}, {Host: "f.localhost"},
	}
	visible := visibleRoutes(routes, "e.localhost", 3)
	if len(visible) != 3 || visible[0].Host != "d.localhost" || visible[1].Host != "e.localhost" || visible[2].Host != "f.localhost" {
		t.Fatalf("unexpected viewport: %#v", visible)
	}
}
