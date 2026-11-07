package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

// A response-start milestone must be observable before the response finishes.
func TestRCStreamingLifecycleWhileInFlight(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"chunk":1}`)
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	router := NewRouter([]config.Route{testRoute(t, "stream.localhost", upstream.URL)})
	server := httptest.NewServer(router)
	defer server.Close()
	defer close(release)
	req, err := http.NewRequest(http.MethodGet, server.URL+"/generate?token=private", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "stream.localhost"
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if !bufio.NewScanner(res.Body).Scan() {
		t.Fatal("first response chunk not received")
	}
	events := router.LifecycleSnapshot()
	if len(events) != 2 {
		t.Fatalf("while streaming: got %d milestones, want 2: %+v", len(events), events)
	}
	if events[0].Phase != LifecycleRequestStarted || events[1].Phase != LifecycleResponseStarted {
		t.Fatalf("phases: %+v", events)
	}
	if events[0].ID != events[1].ID || events[0].Path != "/generate" || events[1].Path != "/generate" {
		t.Fatalf("correlation or redaction: %+v", events)
	}
}

func TestRCCancelledRequestLifecycle(t *testing.T) {
	upstreamReady := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(upstreamReady)
		<-r.Context().Done()
	}))
	defer upstream.Close()
	router := NewRouter([]config.Route{testRoute(t, "cancel.localhost", upstream.URL)})
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/wait", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "cancel.localhost"
	done := make(chan struct{})
	go func() {
		defer close(done)
		res, err := server.Client().Do(req)
		if err == nil {
			res.Body.Close()
		}
	}()
	select {
	case <-upstreamReady:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream never received request")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("client did not cancel")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		events := router.LifecycleSnapshot()
		if len(events) >= 3 {
			last := events[len(events)-1]
			if last.Phase != LifecycleRequestCompleted || !last.Cancelled {
				t.Fatalf("completion: %+v", last)
			}
			if last.ID != events[0].ID {
				t.Fatalf("request ID changed: %+v", events)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("completion missing: %+v", router.LifecycleSnapshot())
}

func TestRCUnknownHostLifecycle(t *testing.T) {
	router := NewRouter(nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://missing.localhost/a?secret=value", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: %d", w.Code)
	}
	events := router.LifecycleSnapshot()
	if len(events) != 3 {
		t.Fatalf("milestones: %+v", events)
	}
	for _, e := range events {
		if e.Path != "/a" || e.Host != "missing.localhost" {
			t.Fatalf("redaction: %+v", e)
		}
	}
	if events[1].Status != http.StatusNotFound || events[2].Status != http.StatusNotFound {
		t.Fatalf("statuses: %+v", events)
	}
}
