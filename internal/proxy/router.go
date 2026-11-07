package proxy

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/status"
)

// Router proxies requests according to their Host header.
type Router struct {
	routes    map[string]config.Route
	observer  statusObserver
	lifecycle *LifecycleRecorder
}

type statusObserver interface {
	RequestStarted()
	RequestCompleted(host string, statusCode int, duration time.Duration)
	SetReachability(host string, state status.Reachability)
}

// NewRouter creates a router from validated routes.
func NewRouter(routes []config.Route) *Router {
	return NewRouterWithStatus(routes, nil)
}

// NewRouterWithStatus creates a router that also records live runtime status.
func NewRouterWithStatus(routes []config.Route, observer statusObserver) *Router {
	byHost := make(map[string]config.Route, len(routes))
	for _, route := range routes {
		byHost[route.Host] = route
	}
	return &Router{routes: byHost, observer: observer, lifecycle: NewLifecycleRecorder(defaultLifecycleCapacity)}
}

// LifecycleSnapshot returns a copy of recent request lifecycle events.
// It is intentionally not exposed by the public status API in this release.
func (router *Router) LifecycleSnapshot() []LifecycleEvent {
	return router.lifecycle.Snapshot()
}

func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := requestHost(r.Host)
	start := time.Now()
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	id, _ := router.lifecycle.begin(host, r.Method, path)
	if router.observer != nil {
		router.observer.RequestStarted()
	}

	inFlight := HTTPRequestsInFlight.WithLabelValues(host)
	inFlight.Inc()
	defer inFlight.Dec()

	statusWriter := &statusResponseWriter{ResponseWriter: w}
	statusWriter.onStart = func(code int) {
		router.lifecycle.record(LifecycleEvent{ID: id, Phase: LifecycleResponseStarted, At: time.Now(), Host: host, Method: r.Method, Path: path, Status: code, Duration: time.Since(start)})
	}
	defer func() {
		router.lifecycle.record(LifecycleEvent{ID: id, Phase: LifecycleRequestCompleted, At: time.Now(), Host: host, Method: r.Method, Path: path, Status: statusWriter.statusCodeInt(), Duration: time.Since(start), Cancelled: r.Context().Err() != nil})
		if router.observer != nil {
			router.observer.RequestCompleted(host, statusWriter.statusCodeInt(), time.Since(start))
		}
	}()

	route, ok := router.routes[host]
	if !ok {
		http.Error(statusWriter, "Proxy: Host not configured", http.StatusNotFound)
		return
	}

	upstreamFailed := false
	p, err := newReverseProxy(route.Target.String(), func(err error) {
		upstreamFailed = true
		if router.observer != nil {
			router.observer.SetReachability(route.Host, reachabilityForError(err))
		}
	})
	if err != nil {
		http.Error(statusWriter, "Proxy: Invalid target", http.StatusBadGateway)
		return
	}

	p.ServeHTTP(statusWriter, r)
	duration := time.Since(start)

	if router.observer != nil && !upstreamFailed {
		router.observer.SetReachability(route.Host, status.ReachabilityConnected)
	}
	HTTPRequestDuration.WithLabelValues(route.Host).Observe(duration.Seconds())
	HTTPRequestsTotal.WithLabelValues(route.Host, statusWriter.statusCode()).Inc()
}

func reachabilityForError(err error) status.Reachability {
	if errors.Is(err, syscall.ECONNREFUSED) {
		return status.ReachabilityRefused
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return status.ReachabilityTimeout
	}
	return status.ReachabilityUnknown
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
	status  int
	onStart func(int)
}

func (w *statusResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	if w.onStart != nil {
		w.onStart(status)
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusResponseWriter) statusCode() string {
	return strconv.Itoa(w.statusCodeInt())
}

func (w *statusResponseWriter) statusCodeInt() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// Unwrap allows http.ResponseController to access optional interfaces provided
// by the underlying ResponseWriter (for example flushing and hijacking).
func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
