package statushttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
