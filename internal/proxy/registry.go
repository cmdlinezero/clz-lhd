package proxy

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

// ErrConfiguredRoute indicates an attempt to mutate a route owned by configuration.
var ErrConfiguredRoute = errors.New("route is managed by configuration")

// ErrRouteExists indicates that a runtime route already exists.
var ErrRouteExists = errors.New("route already exists")

// ErrRouteNotFound indicates that no runtime route exists for the host.
var ErrRouteNotFound = errors.New("runtime route not found")

// RouteRegistry holds configured routes and ephemeral runtime routes.
// All lookups and mutations are concurrency-safe. Returned routes have their
// target URLs copied, so callers cannot mutate registry state indirectly.
type RouteRegistry struct {
	mu         sync.RWMutex
	configured map[string]config.Route
	runtime    map[string]config.Route
}

// NewRouteRegistry initializes the immutable configured-route namespace.
// Callers must supply routes that have already passed configuration validation.
func NewRouteRegistry(routes []config.Route) *RouteRegistry {
	r := &RouteRegistry{configured: make(map[string]config.Route, len(routes)), runtime: make(map[string]config.Route)}
	for _, route := range routes {
		r.configured[route.Host] = copyRoute(route)
	}
	return r
}

func copyRoute(route config.Route) config.Route {
	if route.Target != nil {
		target := *route.Target
		route.Target = &target
	}
	return route
}

// Lookup returns the route active for host at the instant of lookup.
func (r *RouteRegistry) Lookup(host string) (config.Route, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if route, ok := r.configured[host]; ok {
		return copyRoute(route), true
	}
	route, ok := r.runtime[host]
	return copyRoute(route), ok
}

// Snapshot returns a host-sorted copy of all routes.
func (r *RouteRegistry) Snapshot() []config.Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	routes := make([]config.Route, 0, len(r.configured)+len(r.runtime))
	for _, route := range r.configured {
		routes = append(routes, copyRoute(route))
	}
	for _, route := range r.runtime {
		routes = append(routes, copyRoute(route))
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Host < routes[j].Host })
	return routes
}

// Create registers a runtime route. Configured and existing runtime routes
// cannot be overwritten. The specification uses config.ParseRoute validation.
func (r *RouteRegistry) Create(spec string) error {
	route, err := config.ParseRoute(spec)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.configured[route.Host]; ok {
		return fmt.Errorf("%s: %w", route.Host, ErrConfiguredRoute)
	}
	if _, ok := r.runtime[route.Host]; ok {
		return fmt.Errorf("%s: %w", route.Host, ErrRouteExists)
	}
	r.runtime[route.Host] = copyRoute(route)
	return nil
}

// Update replaces an existing runtime route's target atomically.
func (r *RouteRegistry) Update(spec string) error {
	route, err := config.ParseRoute(spec)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.configured[route.Host]; ok {
		return fmt.Errorf("%s: %w", route.Host, ErrConfiguredRoute)
	}
	if _, ok := r.runtime[route.Host]; !ok {
		return fmt.Errorf("%s: %w", route.Host, ErrRouteNotFound)
	}
	r.runtime[route.Host] = copyRoute(route)
	return nil
}

// Remove deletes an ephemeral route; configured routes cannot be deleted.
func (r *RouteRegistry) Remove(host string) error {
	// Reuse config validation and normalization, without allowing an arbitrary
	// Host header to bypass configured-route protection.
	route, err := config.ParseRoute(host + "=http://localhost")
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.configured[route.Host]; ok {
		return fmt.Errorf("%s: %w", route.Host, ErrConfiguredRoute)
	}
	if _, ok := r.runtime[route.Host]; !ok {
		return fmt.Errorf("%s: %w", route.Host, ErrRouteNotFound)
	}
	delete(r.runtime, route.Host)
	return nil
}
