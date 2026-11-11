package statushttp

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/cmdlinezero/clz-lhd/internal/status"
)

// NewManagementHandler extends the read-only status API with authenticated
// runtime route mutations. An empty token disables all mutations.
func NewManagementHandler(model *status.Model, recorder interface{ Snapshot() []proxy.LifecycleEvent }, registry *proxy.RouteRegistry, token string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", NewHandler(model, recorder))
	mux.HandleFunc("GET /v1/routes", func(w http.ResponseWriter, r *http.Request) {
		if !localAuthority(r.Host) {
			http.Error(w, "invalid management host", http.StatusForbidden)
			return
		}
		type item struct {
			Host   string `json:"host"`
			Target string `json:"target"`
			Source string `json:"source"`
			Owner  string `json:"owner,omitempty"`
		}
		routes := registry.SnapshotOwned()
		result := make([]item, 0, len(routes))
		for _, entry := range routes {
			source := "runtime"
			if entry.Owner == "" {
				source = "config"
			}
			result = append(result, item{Host: entry.Route.Host, Target: entry.Route.Target.String(), Source: source, Owner: string(entry.Owner)})
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Routes []item `json:"routes"`
		}{result})
	})
	mutate := func(w http.ResponseWriter, r *http.Request, action string) {
		w.Header().Set("Cache-Control", "no-store")
		if !localAuthority(r.Host) || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
			http.Error(w, "management request rejected", http.StatusForbidden)
			return
		}
		if token == "" {
			http.Error(w, "route mutations disabled: set LHD_MANAGEMENT_TOKEN", http.StatusForbidden)
			return
		}
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authorization, "Bearer ")), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var err error
		switch action {
		case "create", "update":
			if r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			var input struct {
				Host   string `json:"host"`
				Target string `json:"target"`
				Owner  string `json:"owner,omitempty"`
			}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if decoder.Decode(&input) != nil || input.Host == "" || input.Target == "" {
				http.Error(w, "invalid route JSON", http.StatusBadRequest)
				return
			}
			owner := proxy.OwnerManual
			if input.Owner != "" {
				owner = proxy.RouteOwner(input.Owner)
			}
			spec := input.Host + "=" + input.Target
			if action == "create" {
				err = registry.CreateOwned(spec, owner)
			} else {
				err = registry.UpdateOwned(spec, owner)
			}
			if err == nil {
				route, ok := registry.Lookup(input.Host)
				if ok {
					model.AddRoute(route)
				}
			}
		case "delete":
			host := r.PathValue("host")
			owner := proxy.OwnerManual
			if requested := r.URL.Query().Get("owner"); requested != "" {
				owner = proxy.RouteOwner(requested)
			}
			err = registry.RemoveOwned(host, owner)
			if err == nil {
				model.RemoveRoute(host)
			}
		}
		if err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, proxy.ErrConfiguredRoute) || errors.Is(err, proxy.ErrRouteExists) || errors.Is(err, proxy.ErrRouteOwner) {
				code = http.StatusConflict
			}
			if errors.Is(err, proxy.ErrRouteNotFound) {
				code = http.StatusNotFound
			}
			http.Error(w, err.Error(), code)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	mux.HandleFunc("POST /v1/routes", func(w http.ResponseWriter, r *http.Request) { mutate(w, r, "create") })
	mux.HandleFunc("PUT /v1/routes", func(w http.ResponseWriter, r *http.Request) { mutate(w, r, "update") })
	mux.HandleFunc("DELETE /v1/routes/{host}", func(w http.ResponseWriter, r *http.Request) { mutate(w, r, "delete") })
	return mux
}

func localAuthority(authority string) bool {
	host := authority
	if h, _, err := net.SplitHostPort(authority); err == nil {
		host = h
	}
	return host == "127.0.0.1" || host == "localhost" || host == "[::1]" || host == "::1"
}
