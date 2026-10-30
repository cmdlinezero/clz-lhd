package cmd

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

func TestStartupSummary(t *testing.T) {
	routes, err := config.ParseRoutes([]string{
		"app.localhost=3000",
		"api.localhost=http://127.0.0.1:4000",
	})
	if err != nil {
		t.Fatalf("ParseRoutes() error = %v", err)
	}

	got := startupSummary("8088", routes, nil)
	want := "Routes:\n" +
		"  app.localhost -> http://localhost:3000\n" +
		"  api.localhost -> http://127.0.0.1:4000\n" +
		"\n" +
		"HTTP:    http://localhost:8088\n" +
		"Metrics: http://localhost:8088/metrics"

	if got != want {
		t.Fatalf("startupSummary() = %q, want %q", got, want)
	}
}

func TestStartupSummaryWithTLS(t *testing.T) {
	routes, err := config.ParseRoutes([]string{"app.localhost=3000"})
	if err != nil {
		t.Fatalf("ParseRoutes() error = %v", err)
	}

	got := startupSummary("8080", routes, &tlsListenerConfig{Port: "8443"})
	want := "Routes:\n" +
		"  app.localhost -> http://localhost:3000\n" +
		"\n" +
		"HTTP:    http://localhost:8080\n" +
		"HTTPS:   https://localhost:8443\n" +
		"Metrics: http://localhost:8080/metrics"
	if got != want {
		t.Fatalf("startupSummary() = %q, want %q", got, want)
	}
}

func TestServeContextShutsDownWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := serveContext(ctx, "0", nil, nil); err != nil {
		t.Fatalf("serveContext() error = %v, want nil", err)
	}
}

func TestServeContextWithTLSShutsDownWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	incomingTLS := &tlsListenerConfig{Port: "0", Certificate: tls.Certificate{}}
	if err := serveContext(ctx, "0", nil, incomingTLS); err != nil {
		t.Fatalf("serveContext() error = %v, want nil", err)
	}
}

func TestServeContextReportsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	if err := serveContext(context.Background(), port, nil, nil); err == nil {
		t.Fatal("serveContext() error = nil, want listen error")
	}
}

func TestServeContextReportsTLSListenError(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	incomingTLS := &tlsListenerConfig{Port: port, Certificate: tls.Certificate{}}
	if err := serveContext(context.Background(), "0", nil, incomingTLS); err == nil {
		t.Fatal("serveContext() error = nil, want HTTPS listen error")
	}
}

func TestMetricsOnlyAvailableOnHTTPHandler(t *testing.T) {
	httpRecorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodGet, "http://localhost/metrics", nil)
	newHTTPHandler(nil).ServeHTTP(httpRecorder, httpRequest)
	if got, want := httpRecorder.Code, http.StatusOK; got != want {
		t.Fatalf("HTTP /metrics status = %d, want %d", got, want)
	}

	httpsRecorder := httptest.NewRecorder()
	httpsRequest := httptest.NewRequest(http.MethodGet, "https://localhost/metrics", nil)
	newHTTPSHandler(nil).ServeHTTP(httpsRecorder, httpsRequest)
	if got, want := httpsRecorder.Code, http.StatusNotFound; got != want {
		t.Fatalf("HTTPS /metrics status = %d, want %d", got, want)
	}
}
