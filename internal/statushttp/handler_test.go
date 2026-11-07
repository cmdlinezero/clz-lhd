package statushttp

import (
	"encoding/json"
	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/status"
)

type fakeSnapshotter struct{ snapshot status.Snapshot }

func (f fakeSnapshotter) Snapshot() status.Snapshot { return f.snapshot }

func TestStatusEndpointReturnsVersionedSnapshot(t *testing.T) {
	at := time.Date(2026, 10, 30, 20, 0, 0, 0, time.UTC)
	model := fakeSnapshotter{snapshot: status.Snapshot{
		At:       at,
		HTTP:     status.Listener{Address: ":80", Listening: true},
		HTTPS:    status.Listener{Address: ":8443", Listening: true},
		InFlight: 2,
		Requests: 9,
		Routes: []status.Route{{
			Host:         "app.localhost",
			Target:       "http://localhost:3000",
			Reachability: status.ReachabilityConnected,
			Requests:     4,
			LastStatus:   http.StatusOK,
			LastDuration: 1250 * time.Microsecond,
			LastRequest:  at,
		}},
		Traffic: []status.TrafficBucket{{At: at, Requests: 3}},
	}}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	NewHandler(model).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var got Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Requests != 9 || got.InFlight != 2 {
		t.Fatalf("aggregate status = %+v", got)
	}
	if len(got.Routes) != 1 || got.Routes[0].Host != "app.localhost" {
		t.Fatalf("routes = %+v", got.Routes)
	}
	if got.Routes[0].LastDurationMS != 1.25 {
		t.Fatalf("last_duration_ms = %v, want 1.25", got.Routes[0].LastDurationMS)
	}
	if len(got.Traffic) != 1 || got.Traffic[0].Requests != 3 {
		t.Fatalf("traffic = %+v", got.Traffic)
	}
}

func TestStatusAPIIsReadOnly(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/status", nil)
	NewHandler(fakeSnapshotter{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func TestRequestsEndpointNewestFirstAndRedacted(t *testing.T) {
	recorder := proxy.NewLifecycleRecorder(3)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()
	route, err := config.ParseRoute("api.localhost=" + upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	router := proxy.NewRouterWithRecorder([]config.Route{route}, nil, recorder)
	request := httptest.NewRequest(http.MethodGet, "http://api.localhost/private?token=supersecret", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("proxy status = %d", response.Code)
	}

	api := NewHandler(fakeSnapshotter{}, recorder)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/requests", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
	if strings.Contains(w.Body.String(), "supersecret") {
		t.Fatal("query string leaked")
	}
	var result RequestsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 3 {
		t.Fatalf("events = %+v", result.Events)
	}
	want := []proxy.LifecyclePhase{proxy.LifecycleRequestCompleted, proxy.LifecycleResponseStarted, proxy.LifecycleRequestStarted}
	for i, event := range result.Events {
		if event.Phase != want[i] || event.Path != "/private" || event.Host != "api.localhost" || event.ID == 0 {
			t.Fatalf("event %d = %+v", i, event)
		}
		if i > 0 && event.At.After(result.Events[i-1].At) {
			t.Fatal("not newest first")
		}
	}
	if result.Events[0].Status != http.StatusAccepted || result.Events[1].Status != http.StatusAccepted || result.Events[2].Status != 0 {
		t.Fatalf("statuses = %+v", result.Events)
	}
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		w := httptest.NewRecorder()
		api.ServeHTTP(w, httptest.NewRequest(method, "/v1/requests", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s status = %d", method, w.Code)
		}
	}
}

func TestRequestsEndpointEmptyAndBounded(t *testing.T) {
	recorder := proxy.NewLifecycleRecorder(2)
	api := NewHandler(fakeSnapshotter{}, recorder)
	check := func(want int) {
		t.Helper()
		w := httptest.NewRecorder()
		api.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/requests", nil))
		var result RequestsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Events) != want {
			t.Fatalf("len = %d, want %d", len(result.Events), want)
		}
	}
	check(0)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer upstream.Close()
	route, err := config.ParseRoute("api.localhost=" + upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	router := proxy.NewRouterWithRecorder([]config.Route{route}, nil, recorder)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://api.localhost/", nil))
		}()
	}
	wg.Wait()
	check(2)
}
