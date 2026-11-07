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
	if !strings.Contains(requestState(requests[0]), "streaming") {
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
