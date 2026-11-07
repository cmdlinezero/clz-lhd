package dashboard

import (
	"fmt"
	"sort"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/cmdlinezero/clz-lhd/internal/statushttp"
)

// Request combines milestones by ID, retaining incomplete requests too.
type Request struct {
	ID                            uint64
	Host, Method, Path            string
	Started, Responded, Completed time.Time
	Status                        int
	DurationMS                    float64
	TTFBMS                        float64
	Cancelled                     bool
}

// GroupRequests returns newest-started requests first. An event snapshot may
// contain only part of a request when the ring buffer wraps.
func GroupRequests(events []statushttp.RequestEvent) []Request {
	byID := make(map[uint64]*Request)
	for _, event := range events {
		r := byID[event.ID]
		if r == nil {
			r = &Request{ID: event.ID}
			byID[event.ID] = r
		}
		if r.Host == "" {
			r.Host, r.Method, r.Path = event.Host, event.Method, event.Path
		}
		switch event.Phase {
		case proxy.LifecycleRequestStarted:
			r.Started = event.At
		case proxy.LifecycleResponseStarted:
			r.Responded = event.At
			r.Status = event.Status
		case proxy.LifecycleRequestCompleted:
			r.Completed = event.At
			r.DurationMS = event.DurationMS
			r.Cancelled = event.Cancelled
			if event.Status != 0 {
				r.Status = event.Status
			}
		}
	}
	result := make([]Request, 0, len(byID))
	for _, r := range byID {
		result = append(result, *r)
	}
	for i := range result {
		r := &result[i]
		if !r.Started.IsZero() && !r.Responded.IsZero() {
			r.TTFBMS = r.Responded.Sub(r.Started).Seconds() * 1000
			if r.TTFBMS < 0 {
				r.TTFBMS = 0
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID > result[j].ID })
	return result
}

type RequestSelection struct{ ID uint64 }

func (s *RequestSelection) Normalize(requests []Request) {
	if len(requests) == 0 {
		s.ID = 0
		return
	}
	for _, r := range requests {
		if r.ID == s.ID {
			return
		}
	}
	s.ID = requests[0].ID
}
func (s *RequestSelection) Move(requests []Request, delta int) {
	s.Normalize(requests)
	if len(requests) == 0 {
		return
	}
	index := 0
	for i, r := range requests {
		if r.ID == s.ID {
			index = i
			break
		}
	}
	index = max(0, min(len(requests)-1, index+delta))
	s.ID = requests[index].ID
}
func (s *RequestSelection) First(requests []Request) {
	if len(requests) > 0 {
		s.ID = requests[0].ID
	}
}
func (s *RequestSelection) Last(requests []Request) {
	if len(requests) > 0 {
		s.ID = requests[len(requests)-1].ID
	}
}

func RequestLines(snapshot statushttp.Response, requests []Request, fetchErr error, selectedID uint64, width, height int, now time.Time) []string {
	if width < 1 || height < 1 {
		return nil
	}
	if fetchErr != nil {
		return fit([]string{"CLZ LHD", "", "○ DISCONNECTED", trim("Status API: "+fetchErr.Error(), width), "", "Tab [ routes/requests ]    j/k [ navigate ]    g/G [ first/last ]   q [ quit ]"}, width, height)
	}
	lines := []string{"CLZ LHD — REQUESTS", "", trim(fmt.Sprintf("HTTP %s    HTTPS %s    In flight: %d", listener(snapshot.HTTP), listener(snapshot.HTTPS), snapshot.InFlight), width), "", "RECENT REQUESTS"}
	tail := []string{"", "REQUEST DETAIL"}
	for _, r := range requests {
		if r.ID == selectedID {
			tail = append(tail, fmt.Sprintf("ID            %d", r.ID), "Host          "+r.Host, "Method        "+r.Method, "Path          "+r.Path, fmt.Sprintf("Status        %s", requestState(r)), fmt.Sprintf("TTFB          %s", requestTTFB(r)), fmt.Sprintf("Duration      %s", requestDuration(r, now)), "Started       "+formatTime(r.Started), "Response      "+formatTime(r.Responded), "Completed     "+formatTime(r.Completed))
			break
		}
	}
	tail = append(tail, "", "Tab [ routes/requests ]    j/k [ navigate ]    g/G [ first/last ]   q [ quit ]")
	available := max(0, height-len(lines)-len(tail))
	index := 0
	for i, r := range requests {
		if r.ID == selectedID {
			index = i
			break
		}
	}
	start := max(0, index-available/2)
	if start+available > len(requests) {
		start = max(0, len(requests)-available)
	}
	for _, r := range requests[start:min(len(requests), start+available)] {
		prefix := "  "
		if r.ID == selectedID {
			prefix = "> "
		}
		lines = append(lines, trim(fmt.Sprintf("%s#%-6d %-7s %-22s %-24s %s", prefix, r.ID, r.Method, r.Host, r.Path, requestState(r)), width))
	}
	lines = append(lines, tail...)
	return fit(lines, width, height)
}
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("15:04:05.000")
}
func requestState(r Request) string {
	if r.Cancelled {
		return "CANCELLED"
	}
	if !r.Completed.IsZero() {
		return fmt.Sprintf("%d · %.1fms", r.Status, r.DurationMS)
	}
	if !r.Responded.IsZero() {
		return fmt.Sprintf("%d · ACTIVE", r.Status)
	}
	return "WAITING"
}

// TTFB measures time from request arrival to the first response headers.
// It does not prove that the upstream streamed its response body.
func requestTTFB(r Request) string {
	if r.Started.IsZero() || r.Responded.IsZero() {
		return "—"
	}
	return fmt.Sprintf("%.1fms", r.TTFBMS)
}

func requestDuration(r Request, now time.Time) string {
	if !r.Completed.IsZero() {
		return fmt.Sprintf("%.1fms", r.DurationMS)
	}
	if r.Started.IsZero() {
		return "—"
	}
	elapsed := now.Sub(r.Started)
	if elapsed < 0 {
		elapsed = 0
	}
	return fmt.Sprintf("%.1fms (ongoing)", float64(elapsed)/float64(time.Millisecond))
}
