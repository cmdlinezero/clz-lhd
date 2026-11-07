package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

// The loopback upstream stands in for an HTTP service on another LAN machine:
// routing depends on its URL, not on whether the address is localhost.
func TestRouterProxiesExplicitNetworkTarget(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("upstream path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"models":[]}`)
	}))
	defer upstream.Close()

	route, err := config.ParseRoute("ollama.localhost=" + upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewRouter([]config.Route{route}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/tags", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "ollama.localhost"
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("content type = %q", got)
	}
	scanner := bufio.NewScanner(res.Body)
	if !scanner.Scan() || scanner.Text() != `{"models":[]}` {
		t.Fatalf("response = %q", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestRouterStreamsNDJSONBeforeUpstreamCompletes(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"response":"first","done":false}`)
		w.(http.Flusher).Flush()
		select {
		case <-release:
			fmt.Fprintln(w, `{"response":"second","done":true}`)
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()

	server := httptest.NewServer(NewRouter([]config.Route{testRoute(t, "ollama.localhost", upstream.URL)}))
	defer server.Close()
	defer unblock()
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/generate", strings.NewReader(`{"model":"example","stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "ollama.localhost"
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("first streamed headers not received before upstream completed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	scanner := bufio.NewScanner(res.Body)
	if !scanner.Scan() {
		t.Fatalf("first chunk not received: %v", scanner.Err())
	}
	if got := scanner.Text(); got != `{"response":"first","done":false}` {
		t.Fatalf("first chunk = %q", got)
	}
	// The upstream is still blocked on release at this point. The first line
	// therefore proves that the proxy did not buffer until request completion.
	unblock()
	if !scanner.Scan() {
		t.Fatalf("final chunk not received: %v", scanner.Err())
	}
	if got := scanner.Text(); got != `{"response":"second","done":true}` {
		t.Fatalf("final chunk = %q", got)
	}
	if scanner.Scan() {
		t.Fatalf("unexpected extra chunk %q", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestRouterPropagatesClientCancellation(t *testing.T) {
	cancelled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"response":"started","done":false}`)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(cancelled)
	}))
	defer upstream.Close()

	server := httptest.NewServer(NewRouter([]config.Route{testRoute(t, "ollama.localhost", upstream.URL)}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/chat", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "ollama.localhost"
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(res.Body)
	if !scanner.Scan() {
		t.Fatalf("first chunk not received: %v", scanner.Err())
	}
	cancel()
	res.Body.Close()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream request context not cancelled when client disconnected")
	}
}
