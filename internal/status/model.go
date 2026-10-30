// Package status holds the bounded live runtime state used by interactive
// clients such as the dashboard. It deliberately does not retain historical
// data; Prometheus remains the interface for long-term metrics.
package status

import (
	"sort"
	"sync"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

const trafficWindow = 60

// Reachability describes the latest known state of a route's upstream.
type Reachability string

const (
	ReachabilityUnknown   Reachability = "unknown"
	ReachabilityConnected Reachability = "connected"
	ReachabilityRefused   Reachability = "refused"
	ReachabilityTimeout   Reachability = "timeout"
)

// Listener describes one incoming LHD listener.
type Listener struct {
	Address   string
	Listening bool
}

// Route is the current live state for one configured route.
type Route struct {
	Host         string
	Target       string
	Reachability Reachability
	LastChange   time.Time
	Requests     uint64
	LastStatus   int
	LastDuration time.Duration
	LastRequest  time.Time
}

// TrafficBucket is one second of aggregate proxy traffic.
type TrafficBucket struct {
	At       time.Time
	Requests uint64
}

// Snapshot is an immutable copy of the current runtime state.
type Snapshot struct {
	At       time.Time
	HTTP     Listener
	HTTPS    Listener
	Routes   []Route
	Traffic  []TrafficBucket
	InFlight int64
	Requests uint64
}

// Model stores the small amount of live state needed by an interactive
// dashboard. All methods are safe for concurrent use.
type Model struct {
	mu sync.RWMutex

	now func() time.Time

	http     Listener
	https    Listener
	routes   map[string]Route
	traffic  [trafficWindow]TrafficBucket
	inFlight int64
	requests uint64
}

// New creates live status for the configured routes.
func New(routes []config.Route) *Model {
	m := &Model{
		now:    time.Now,
		routes: make(map[string]Route, len(routes)),
	}
	for _, route := range routes {
		m.routes[route.Host] = Route{
			Host:         route.Host,
			Target:       route.Target.String(),
			Reachability: ReachabilityUnknown,
		}
	}
	return m
}

// SetListener records whether an incoming listener is currently accepting
// connections. The protocol must be "http" or "https"; other values are
// ignored.
func (m *Model) SetListener(protocol, address string, listening bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	listener := Listener{Address: address, Listening: listening}
	switch protocol {
	case "http":
		m.http = listener
	case "https":
		m.https = listener
	}
}

// SetReachability records the latest upstream reachability state for host.
// Unknown hosts are ignored because the model represents configured routes.
func (m *Model) SetReachability(host string, state Reachability) {
	m.mu.Lock()
	defer m.mu.Unlock()

	route, ok := m.routes[host]
	if !ok || route.Reachability == state {
		return
	}
	route.Reachability = state
	route.LastChange = m.now()
	m.routes[host] = route
}

// RequestStarted increments the number of currently active proxy requests.
func (m *Model) RequestStarted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inFlight++
}

// RequestCompleted records a completed proxy request and adds it to the
// aggregate one-second traffic window. Unknown hosts still contribute to the
// aggregate traffic count, but do not create route state.
func (m *Model) RequestCompleted(host string, statusCode int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	m.requests++
	if m.inFlight > 0 {
		m.inFlight--
	}
	m.recordTraffic(now)

	route, ok := m.routes[host]
	if !ok {
		return
	}
	route.Requests++
	route.LastStatus = statusCode
	route.LastDuration = duration
	route.LastRequest = now
	m.routes[host] = route
}

func (m *Model) recordTraffic(now time.Time) {
	second := now.Truncate(time.Second)
	index := int(second.Unix() % trafficWindow)
	bucket := &m.traffic[index]
	if !bucket.At.Equal(second) {
		*bucket = TrafficBucket{At: second}
	}
	bucket.Requests++
}

// Snapshot returns a deterministic, immutable view of the live state. Traffic
// always contains the last 60 one-second buckets, oldest first, including zero
// buckets so a terminal graph can render without filling gaps itself.
func (m *Model) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := m.now()
	routes := make([]Route, 0, len(m.routes))
	for _, route := range m.routes {
		routes = append(routes, route)
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Host < routes[j].Host })

	traffic := make([]TrafficBucket, trafficWindow)
	end := now.Truncate(time.Second)
	start := end.Add(-(trafficWindow - 1) * time.Second)
	for i := range traffic {
		at := start.Add(time.Duration(i) * time.Second)
		traffic[i] = TrafficBucket{At: at}
		index := int(at.Unix() % trafficWindow)
		bucket := m.traffic[index]
		if bucket.At.Equal(at) {
			traffic[i].Requests = bucket.Requests
		}
	}

	return Snapshot{
		At:       now,
		HTTP:     m.http,
		HTTPS:    m.https,
		Routes:   routes,
		Traffic:  traffic,
		InFlight: m.inFlight,
		Requests: m.requests,
	}
}
