package proxy

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

// Router proxies requests according to their Host header.
type Router struct {
	routes map[string]config.Route
}

// NewRouter creates a router from validated routes.
func NewRouter(routes []config.Route) *Router {
	byHost := make(map[string]config.Route, len(routes))
	for _, route := range routes {
		byHost[route.Host] = route
	}
	return &Router{routes: byHost}
}

func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := requestHost(r.Host)
	inFlight := HTTPRequestsInFlight.WithLabelValues(host)
	inFlight.Inc()
	defer inFlight.Dec()

	route, ok := router.routes[host]
	if !ok {
		http.Error(w, "Proxy: Host not configured", http.StatusNotFound)
		return
	}

	p, err := NewReverseProxy(route.Target.String())
	if err != nil {
		http.Error(w, "Proxy: Invalid target", http.StatusBadGateway)
		return
	}

	start := time.Now()
	statusWriter := &statusResponseWriter{ResponseWriter: w}
	p.ServeHTTP(statusWriter, r)

	HTTPRequestDuration.WithLabelValues(route.Host).Observe(time.Since(start).Seconds())
	HTTPRequestsTotal.WithLabelValues(route.Host, statusWriter.statusCode()).Inc()
}

func requestHost(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusResponseWriter) statusCode() string {
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	return strconv.Itoa(status)
}

// Unwrap allows http.ResponseController to access optional interfaces provided
// by the underlying ResponseWriter (for example flushing and hijacking).
func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
