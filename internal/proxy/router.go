package proxy

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Router proxies requests according to their Host header.
type Router struct {
	routes map[string]string
}

// NewRouter creates a router for host=target mappings. Targets without an HTTP
// scheme are treated as local ports.
func NewRouter(routes map[string]string) *Router {
	return &Router{routes: routes}
}

func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := strings.Split(r.Host, ":")[0]

	target, ok := router.routes[host]
	if !ok {
		http.Error(w, "Proxy: Host not configured", http.StatusNotFound)
		return
	}

	if !strings.HasPrefix(target, "http") {
		target = "http://localhost:" + target
	}

	p, err := NewReverseProxy(target)
	if err != nil {
		http.Error(w, "Proxy: Invalid target", http.StatusBadGateway)
		return
	}

	start := time.Now()
	statusWriter := &statusResponseWriter{ResponseWriter: w}
	p.ServeHTTP(statusWriter, r)

	HttpRequestDuration.WithLabelValues(host).Observe(time.Since(start).Seconds())
	HttpRequestsTotal.WithLabelValues(host, statusWriter.statusCode()).Inc()
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
