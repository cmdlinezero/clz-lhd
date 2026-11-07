package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

func TestLifecycleRecorderBoundedConcurrent(t *testing.T) {
	recorder := NewLifecycleRecorder(9)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, _ := recorder.begin("a.localhost", "GET", "/safe")
			recorder.record(LifecycleEvent{ID: id, Phase: LifecycleRequestCompleted})
		}()
	}
	wg.Wait()
	got := recorder.Snapshot()
	if len(got) != 9 {
		t.Fatalf("len = %d", len(got))
	}
	for _, event := range got {
		if event.ID == 0 {
			t.Fatal("missing event ID")
		}
	}
	got[0].Host = "changed"
	if recorder.Snapshot()[0].Host == "changed" {
		t.Fatal("snapshot aliases recorder")
	}
}

func TestLifecycleMilestonesAndQueryRedaction(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"partial":true}`)
		w.(http.Flusher).Flush()
		time.Sleep(30 * time.Millisecond)
		fmt.Fprintln(w, `{"done":true}`)
	}))
	defer upstream.Close()
	route, err := config.ParseRoute("api.localhost=" + upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter([]config.Route{route})
	server := httptest.NewServer(router)
	defer server.Close()
	req, err := http.NewRequest("GET", server.URL+"/stream?token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "api.localhost"
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	// Drain the response before checking completion.
	var body [1024]byte
	for {
		_, err = res.Body.Read(body[:])
		if err != nil {
			break
		}
	}
	// The upstream handler may return just after the client drains the body.
	var events []LifecycleEvent
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		events = router.LifecycleSnapshot()
		if len(events) == 3 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	phases := []LifecyclePhase{LifecycleRequestStarted, LifecycleResponseStarted, LifecycleRequestCompleted}
	for i, e := range events {
		if e.Phase != phases[i] || e.Path != "/stream" || e.Host != "api.localhost" || e.Method != "GET" {
			t.Fatalf("event %d = %+v", i, e)
		}
		if e.ID != events[0].ID {
			t.Fatal("different request IDs")
		}
		if i > 0 && e.At.Before(events[i-1].At) {
			t.Fatal("out of order timestamps")
		}
	}
	if events[1].Status != 200 || events[2].Status != 200 {
		t.Fatalf("statuses = %+v", events)
	}
	if events[2].Duration < events[1].Duration {
		t.Fatal("completion precedes response")
	}
}
