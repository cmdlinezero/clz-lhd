package proxy

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewReverseProxyRejectsInvalidTarget(t *testing.T) {
	t.Parallel()

	proxy, err := NewReverseProxy("://invalid")
	if err == nil {
		t.Fatal("NewReverseProxy() error = nil, want an error")
	}
	if proxy != nil {
		t.Fatalf("NewReverseProxy() proxy = %v, want nil", proxy)
	}
}

func TestNewReverseProxyForwardsRequest(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/hello"; got != want {
			t.Errorf("upstream path = %q, want %q", got, want)
		}
		if got, want := r.URL.RawQuery, "name=lhd"; got != want {
			t.Errorf("upstream query = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Test"), "proxy"; got != want {
			t.Errorf("upstream X-Test header = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Forwarded-Host"), "app.localhost"; got != want {
			t.Errorf("upstream X-Forwarded-Host = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Forwarded-Proto"), "http"; got != want {
			t.Errorf("upstream X-Forwarded-Proto = %q, want %q", got, want)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading upstream request body: %v", err)
		}
		if got, want := string(body), "request body"; got != want {
			t.Errorf("upstream body = %q, want %q", got, want)
		}

		w.Header().Set("X-Upstream", "ok")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	proxy, err := NewReverseProxy(upstream.URL)
	if err != nil {
		t.Fatalf("NewReverseProxy() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://app.localhost/hello?name=lhd", strings.NewReader("request body"))
	req.Header.Set("X-Test", "proxy")
	recorder := httptest.NewRecorder()

	proxy.ServeHTTP(recorder, req)

	res := recorder.Result()
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusCreated; got != want {
		t.Errorf("response status = %d, want %d", got, want)
	}
	if got, want := res.Header.Get("X-Upstream"), "ok"; got != want {
		t.Errorf("response X-Upstream header = %q, want %q", got, want)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("reading proxy response body: %v", err)
	}
	if got, want := string(body), "upstream response"; got != want {
		t.Errorf("response body = %q, want %q", got, want)
	}
}

func TestNewReverseProxyUsesLocalDevelopmentTransport(t *testing.T) {
	t.Parallel()

	proxy, err := NewReverseProxy("http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("NewReverseProxy() error = %v", err)
	}

	transport, ok := proxy.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("proxy.Transport type = %T, want *http.Transport", proxy.Transport)
	}
	if got, want := transport.MaxIdleConnsPerHost, 100; got != want {
		t.Errorf("MaxIdleConnsPerHost = %d, want %d", got, want)
	}
}

func TestNewReverseProxyReturnsBadGatewayForUnavailableUpstream(t *testing.T) {
	t.Parallel()

	listener := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	target := listener.URL
	listener.Close()

	proxy, err := NewReverseProxy(target)
	if err != nil {
		t.Fatalf("NewReverseProxy() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://app.localhost/", nil)
	proxy.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusBadGateway; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if got, want := recorder.Body.String(), "Proxy: upstream unavailable\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestNewReverseProxyForwardsHTTPSProtocol(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("X-Forwarded-Proto"), "https"; got != want {
			t.Errorf("upstream X-Forwarded-Proto = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	proxy, err := NewReverseProxy(upstream.URL)
	if err != nil {
		t.Fatalf("NewReverseProxy() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://app.localhost/", nil)
	req.TLS = &tls.ConnectionState{}
	recorder := httptest.NewRecorder()

	proxy.ServeHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Errorf("response status = %d, want %d", got, want)
	}
}
