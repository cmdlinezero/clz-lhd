// Package statushttp exposes the local read-only runtime status API used by
// interactive clients such as clz-lhd dash.
package statushttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/status"
)

type snapshotter interface {
	Snapshot() status.Snapshot
}

// NewHandler returns the versioned read-only status API.
func NewHandler(model snapshotter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(NewResponse(model.Snapshot()))
	})
	return mux
}

type Response struct {
	At       time.Time       `json:"at"`
	HTTP     Listener        `json:"http"`
	HTTPS    Listener        `json:"https"`
	Routes   []Route         `json:"routes"`
	Traffic  []TrafficBucket `json:"traffic"`
	InFlight int64           `json:"in_flight"`
	Requests uint64          `json:"requests"`
}

type Listener struct {
	Address   string `json:"address"`
	Listening bool   `json:"listening"`
}

type Route struct {
	Host           string              `json:"host"`
	Target         string              `json:"target"`
	Reachability   status.Reachability `json:"reachability"`
	LastChange     time.Time           `json:"last_change,omitempty"`
	Requests       uint64              `json:"requests"`
	LastStatus     int                 `json:"last_status"`
	LastDurationMS float64             `json:"last_duration_ms"`
	LastRequest    time.Time           `json:"last_request,omitempty"`
}

type TrafficBucket struct {
	At       time.Time `json:"at"`
	Requests uint64    `json:"requests"`
}

func NewResponse(snapshot status.Snapshot) Response {
	routes := make([]Route, len(snapshot.Routes))
	for i, item := range snapshot.Routes {
		routes[i] = Route{
			Host:           item.Host,
			Target:         item.Target,
			Reachability:   item.Reachability,
			LastChange:     item.LastChange,
			Requests:       item.Requests,
			LastStatus:     item.LastStatus,
			LastDurationMS: float64(item.LastDuration) / float64(time.Millisecond),
			LastRequest:    item.LastRequest,
		}
	}

	traffic := make([]TrafficBucket, len(snapshot.Traffic))
	for i, item := range snapshot.Traffic {
		traffic[i] = TrafficBucket{At: item.At, Requests: item.Requests}
	}

	return Response{
		At:       snapshot.At,
		HTTP:     Listener(snapshot.HTTP),
		HTTPS:    Listener(snapshot.HTTPS),
		Routes:   routes,
		Traffic:  traffic,
		InFlight: snapshot.InFlight,
		Requests: snapshot.Requests,
	}
}
