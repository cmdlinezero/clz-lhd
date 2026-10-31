package statusclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/status" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requests":42,"in_flight":3}`))
	}))
	defer server.Close()

	client := &Client{URL: server.URL + "/v1/status", HTTPClient: server.Client()}
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Requests != 42 || snapshot.InFlight != 3 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestClientRejectsNonOKResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &Client{URL: server.URL, HTTPClient: server.Client()}
	if _, err := client.Snapshot(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
