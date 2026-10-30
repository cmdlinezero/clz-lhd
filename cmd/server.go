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
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const shutdownTimeout = 5 * time.Second

type tlsListenerConfig struct {
	Port        string
	Certificate tls.Certificate
}

func serve(port string, routes []config.Route) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return serveContext(ctx, port, routes, nil)
}

func serveTLS(port string, routes []config.Route, tlsConfig tlsListenerConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return serveContext(ctx, port, routes, &tlsConfig)
}

func serveContext(ctx context.Context, port string, routes []config.Route, incomingTLS *tlsListenerConfig) error {
	httpHandler := newHTTPHandler(routes)
	httpsHandler := newHTTPSHandler(routes)

	httpListener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	defer httpListener.Close()

	var httpsListener net.Listener
	if incomingTLS != nil {
		httpsListener, err = net.Listen("tcp", ":"+incomingTLS.Port)
		if err != nil {
			return fmt.Errorf("listen HTTPS on :%s: %w", incomingTLS.Port, err)
		}
		defer httpsListener.Close()
		httpsListener = tls.NewListener(httpsListener, &tls.Config{
			Certificates: []tls.Certificate{incomingTLS.Certificate},
			MinVersion:   tls.VersionTLS12,
		})
	}

	httpServer := &http.Server{Handler: httpHandler}
	servers := []*http.Server{httpServer}
	serveErr := make(chan error, 2)
	go func() { serveErr <- httpServer.Serve(httpListener) }()

	if httpsListener != nil {
		httpsServer := &http.Server{Handler: httpsHandler}
		servers = append(servers, httpsServer)
		go func() { serveErr <- httpsServer.Serve(httpsListener) }()
	}

	log.Print(startupSummary(port, routes, incomingTLS))

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
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", proxy.NewRouter(routes))
	return mux
}

func newHTTPSHandler(routes []config.Route) http.Handler {
	return proxy.NewRouter(routes)
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

func startupSummary(port string, routes []config.Route, incomingTLS *tlsListenerConfig) string {
	var summary strings.Builder

	summary.WriteString("Routes:\n")
	for _, route := range routes {
		fmt.Fprintf(&summary, "  %s -> %s\n", route.Host, route.Target)
	}
	fmt.Fprintf(&summary, "\nHTTP:    http://localhost:%s\n", port)
	if incomingTLS != nil {
		fmt.Fprintf(&summary, "HTTPS:   https://localhost:%s\n", incomingTLS.Port)
	}
	fmt.Fprintf(&summary, "Metrics: http://localhost:%s/metrics", port)

	return summary.String()
}
