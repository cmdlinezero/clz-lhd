package proxy

import (
	"sync"
	"time"
)

// LifecyclePhase identifies a request milestone. A response can start well
// before completion when the upstream streams its body.
type LifecyclePhase string

const (
	LifecycleRequestStarted   LifecyclePhase = "request_started"
	LifecycleResponseStarted  LifecyclePhase = "response_started"
	LifecycleRequestCompleted LifecyclePhase = "request_completed"
)

// LifecycleEvent contains metadata only. Path excludes query parameters and
// fragments; headers and bodies are never retained.
type LifecycleEvent struct {
	ID        uint64
	Phase     LifecyclePhase
	At        time.Time
	Host      string
	Method    string
	Path      string
	Status    int
	Duration  time.Duration
	Cancelled bool
}

// LifecycleRecorder retains the newest events in a fixed-size ring buffer.
// Snapshot returns an independent chronological copy.
type LifecycleRecorder struct {
	mu     sync.Mutex
	events []LifecycleEvent
	next   int
	count  int
	nextID uint64
}

const defaultLifecycleCapacity = 256

func NewLifecycleRecorder(capacity int) *LifecycleRecorder {
	if capacity < 1 {
		capacity = defaultLifecycleCapacity
	}
	return &LifecycleRecorder{events: make([]LifecycleEvent, capacity)}
}

func (r *LifecycleRecorder) begin(host, method, path string) (uint64, time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	now := time.Now()
	r.appendLocked(LifecycleEvent{ID: r.nextID, Phase: LifecycleRequestStarted, At: now, Host: host, Method: method, Path: path})
	return r.nextID, now
}

func (r *LifecycleRecorder) record(event LifecycleEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendLocked(event)
}

func (r *LifecycleRecorder) appendLocked(event LifecycleEvent) {
	r.events[r.next] = event
	r.next = (r.next + 1) % len(r.events)
	if r.count < len(r.events) {
		r.count++
	}
}

func (r *LifecycleRecorder) Snapshot() []LifecycleEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]LifecycleEvent, r.count)
	start := (r.next - r.count + len(r.events)) % len(r.events)
	for i := range result {
		result[i] = r.events[(start+i)%len(r.events)]
	}
	return result
}
