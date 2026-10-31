package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/cmdlinezero/clz-lhd/internal/status"
	"github.com/cmdlinezero/clz-lhd/internal/statushttp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const shutdownTimeout = 5 * time.Second

type tlsListenerConfig struct {
	Port        string
	Certificate tls.Certificate
}

func serve(port, statusPort string, routes []config.Route) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return serveContext(ctx, port, statusPort, routes, nil)
}

func serveTLS(port, statusPort string, routes []config.Route, tlsConfig tlsListenerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return serveContext(ctx, port, statusPort, routes, &tlsConfig)
}

func serveContext(ctx context.Context, port, statusPort string, routes []config.Route, incomingTLS *tlsListenerConfig) error {
	liveStatus := status.New(routes)
	httpHandler := newHTTPHandlerWithStatus(routes, liveStatus)
	httpsHandler := newHTTPSHandlerWithStatus(routes, liveStatus)

	httpListener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	defer httpListener.Close()
	liveStatus.SetListener("http", httpListener.Addr().String(), true)
	defer liveStatus.SetListener("http", httpListener.Addr().String(), false)

	var httpsListener net.Listener
	if incomingTLS != nil {
		httpsListener, err = net.Listen("tcp", ":"+incomingTLS.Port)
		if err != nil {
			return fmt.Errorf("listen HTTPS on :%s: %w", incomingTLS.Port, err)
		}
		defer httpsListener.Close()
		liveStatus.SetListener("https", httpsListener.Addr().String(), true)
		defer liveStatus.SetListener("https", httpsListener.Addr().String(), false)
		httpsListener = tls.NewListener(httpsListener, &tls.Config{
			Certificates: []tls.Certificate{incomingTLS.Certificate},
			MinVersion:   tls.VersionTLS12,
		})
	}

	statusListener, err := net.Listen("tcp", "127.0.0.1:"+statusPort)
	if err != nil {
		return fmt.Errorf("listen status on 127.0.0.1:%s: %w", statusPort, err)
	}
	defer statusListener.Close()

	httpServer := &http.Server{Handler: httpHandler}
	statusServer := &http.Server{Handler: statushttp.NewHandler(liveStatus)}
	servers := []*http.Server{httpServer, statusServer}
	serveErr := make(chan error, 3)
	go func() { serveErr <- httpServer.Serve(httpListener) }()
	go func() { serveErr <- statusServer.Serve(statusListener) }()

	if httpsListener != nil {
		httpsServer := &http.Server{Handler: httpsHandler}
		servers = append(servers, httpsServer)
		go func() { serveErr <- httpsServer.Serve(httpsListener) }()
	}

	log.Print(startupSummary(port, statusPort, routes, incomingTLS))

	select {
	case err := <-serveErr:
		shutdownServers(servers)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		if err := shutdownServers(servers); err != nil {
			return err
		}
		return nil
	}
}

func newHTTPHandler(routes []config.Route) http.Handler {
	return newHTTPHandlerWithStatus(routes, nil)
}

func newHTTPHandlerWithStatus(routes []config.Route, liveStatus *status.Model) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", newProxyRouter(routes, liveStatus))
	return mux
}

func newHTTPSHandler(routes []config.Route) http.Handler {
	return newHTTPSHandlerWithStatus(routes, nil)
}

func newHTTPSHandlerWithStatus(routes []config.Route, liveStatus *status.Model) http.Handler {
	return newProxyRouter(routes, liveStatus)
}

func newProxyRouter(routes []config.Route, liveStatus *status.Model) http.Handler {
	if liveStatus == nil {
		return proxy.NewRouter(routes)
	}
	return proxy.NewRouterWithStatus(routes, liveStatus)
}

func shutdownServers(servers []*http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	for _, server := range servers {
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
	}
	return nil
}

func startupSummary(port, statusPort string, routes []config.Route, incomingTLS *tlsListenerConfig) string {
	var summary strings.Builder

	summary.WriteString("Routes:\n")
	for _, route := range routes {
		fmt.Fprintf(&summary, "  %s -> %s\n", route.Host, route.Target)
	}
	fmt.Fprintf(&summary, "\nHTTP:    http://localhost:%s\n", port)
	if incomingTLS != nil {
		fmt.Fprintf(&summary, "HTTPS:   https://localhost:%s\n", incomingTLS.Port)
	}
	fmt.Fprintf(&summary, "Metrics: http://localhost:%s/metrics\n", port)
	fmt.Fprintf(&summary, "Status:  http://127.0.0.1:%s/v1/status", statusPort)

	return summary.String()
}
