package cmd

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRouteManagementRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/routes" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing bearer token")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing content type")
		}
		var input struct{ Host, Target string }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if input.Host != "app.localhost" || input.Target != "http://127.0.0.1:5173" {
			t.Errorf("unexpected payload: %+v", input)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	// Use the test server's ephemeral port, while keeping the client restricted to loopback.
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	client := routeAPIClient{port: port, token: "secret", httpClient: &http.Client{Timeout: time.Second}}
	_, err = client.request(http.MethodPost, "/v1/routes", struct {
		Host   string `json:"host"`
		Target string `json:"target"`
	}{"app.localhost", "http://127.0.0.1:5173"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRouteManagementMissingToken(t *testing.T) {
	client := routeAPIClient{port: "9099", httpClient: &http.Client{Timeout: time.Second}}
	if _, err := client.request(http.MethodDelete, "/v1/routes/app.localhost", nil); err == nil || !strings.Contains(err.Error(), "LHD_MANAGEMENT_TOKEN") {
		t.Fatalf("error = %v", err)
	}
}

func TestRouteManagementUnavailableAndInvalidPort(t *testing.T) {
	client := routeAPIClient{port: "bad", token: "secret", httpClient: &http.Client{Timeout: time.Second}}
	if _, err := client.request(http.MethodGet, "/v1/routes", nil); err == nil {
		t.Fatal("expected invalid port error")
	}
	client.port = "1"
	if _, err := client.request(http.MethodGet, "/v1/routes", nil); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestRouteManagementAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "route conflict", http.StatusConflict) }))
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	client := routeAPIClient{port: port, token: "secret", httpClient: &http.Client{Timeout: time.Second}}
	_, err := client.request(http.MethodDelete, "/v1/routes/app.localhost", nil)
	if err == nil || !strings.Contains(err.Error(), "409") || !strings.Contains(err.Error(), "route conflict") {
		t.Fatalf("error = %v", err)
	}
}
