package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/cmdlinezero/clz-lhd/internal/statushttp"
)

func TestGroupRequestsMilestonesAndSelection(t *testing.T) {
	now := time.Now()
	events := []statushttp.RequestEvent{
		{ID: 1, Phase: proxy.LifecycleRequestCompleted, At: now.Add(time.Second), Status: 200, DurationMS: 1000},
		{ID: 2, Phase: proxy.LifecycleRequestStarted, At: now, Host: "new.localhost", Method: "GET", Path: "/stream"},
		{ID: 1, Phase: proxy.LifecycleRequestStarted, At: now, Host: "old.localhost", Method: "POST", Path: "/api"},
		{ID: 2, Phase: proxy.LifecycleResponseStarted, At: now, Status: 200},
	}
	requests := GroupRequests(events)
	if len(requests) != 2 || requests[0].ID != 2 || requests[1].Status != 200 || requests[1].Path != "/api" {
		t.Fatalf("unexpected groups: %+v", requests)
	}
	if !strings.Contains(requestState(requests[0]), "ACTIVE") {
		t.Fatalf("missing active state: %+v", requests[0])
	}
	s := RequestSelection{}
	s.Normalize(requests)
	if s.ID != 2 {
		t.Fatalf("selected %d", s.ID)
	}
	s.Move(requests, 1)
	if s.ID != 1 {
		t.Fatalf("moved to %d", s.ID)
	}
	s.Normalize(requests)
	if s.ID != 1 {
		t.Fatal("selection changed on refresh")
	}
	s.First(requests)
	if s.ID != 2 {
		t.Fatal("first failed")
	}
	s.Last(requests)
	if s.ID != 1 {
		t.Fatal("last failed")
	}
	lines := RequestLines(statushttp.Response{At: now}, requests, nil, 1, 80, 24, now)
	output := strings.Join(lines, "\n")
	for _, want := range []string{"RECENT REQUESTS", "REQUEST DETAIL", "old.localhost", "/api", "Tab [ routes/requests ]"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q:\n%s", want, output)
		}
	}
	for _, line := range lines {
		if len([]rune(line)) > 80 {
			t.Fatalf("wide line %q", line)
		}
	}
}

func TestRequestTimingAndStates(t *testing.T) {
	start := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	events := []statushttp.RequestEvent{
		{ID: 1, Phase: proxy.LifecycleRequestStarted, At: start, Host: "ollama.localhost"},
		{ID: 1, Phase: proxy.LifecycleResponseStarted, At: start.Add(125 * time.Millisecond), Status: 200},
	}
	r := GroupRequests(events)[0]
	if r.TTFBMS != 125 || requestTTFB(r) != "125.0ms" {
		t.Fatalf("unexpected TTFB: %+v", r)
	}
	if state := requestState(r); state != "200 · ACTIVE" {
		t.Fatalf("active request should not imply streaming: %s", state)
	}
	if got := requestDuration(r, start.Add(2*time.Second)); got != "2000.0ms (ongoing)" {
		t.Fatalf("ongoing duration: %s", got)
	}
	events = append(events, statushttp.RequestEvent{ID: 1, Phase: proxy.LifecycleRequestCompleted, At: start.Add(3 * time.Second), Status: 200, DurationMS: 3000})
	r = GroupRequests(events)[0]
	if requestTTFB(r) != "125.0ms" || requestDuration(r, start.Add(10*time.Second)) != "3000.0ms" {
		t.Fatalf("completed timing: %+v", r)
	}
	if requestState(r) != "200 · 3000.0ms" {
		t.Fatalf("completed state: %s", requestState(r))
	}
}

func TestRequestTimingWithMissingMilestonesAndCancellation(t *testing.T) {
	now := time.Now()
	r := GroupRequests([]statushttp.RequestEvent{{ID: 4, Phase: proxy.LifecycleRequestCompleted, At: now, Status: 502, DurationMS: 5, Cancelled: true}})[0]
	if requestTTFB(r) != "—" || requestDuration(r, now) != "5.0ms" || requestState(r) != "CANCELLED" {
		t.Fatalf("partial cancelled request: %+v", r)
	}
}
