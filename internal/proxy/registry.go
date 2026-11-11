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

// RouteOwner identifies the component responsible for an ephemeral route.
// Ownership is an integrity boundary within the management API, not a separate
// authentication identity: all callers currently share one management token.
type RouteOwner string

const (
	OwnerManual   RouteOwner = "manual"
	OwnerWorktree RouteOwner = "worktree"
	OwnerAgent    RouteOwner = "agent"
)

var ErrRouteOwner = errors.New("runtime route owned by another component")
var ErrInvalidOwner = errors.New("invalid runtime route owner")

func validOwner(owner RouteOwner) bool {
	return owner == OwnerManual || owner == OwnerWorktree || owner == OwnerAgent
}

type RuntimeRoute struct {
	Route config.Route
	Owner RouteOwner
}

// RouteRegistry holds configured routes and ephemeral runtime routes.
// All lookups and mutations are concurrency-safe. Returned routes have their
// target URLs copied, so callers cannot mutate registry state indirectly.
type RouteRegistry struct {
	mu         sync.RWMutex
	configured map[string]config.Route
	runtime    map[string]RuntimeRoute
}

// NewRouteRegistry initializes the immutable configured-route namespace.
// Callers must supply routes that have already passed configuration validation.
func NewRouteRegistry(routes []config.Route) *RouteRegistry {
	r := &RouteRegistry{configured: make(map[string]config.Route, len(routes)), runtime: make(map[string]RuntimeRoute)}
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
	return copyRoute(route.Route), ok
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
		routes = append(routes, copyRoute(route.Route))
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Host < routes[j].Host })
	return routes
}

// Create registers a runtime route. Configured and existing runtime routes
// cannot be overwritten. The specification uses config.ParseRoute validation.
func (r *RouteRegistry) Create(spec string) error { return r.CreateOwned(spec, OwnerManual) }

func (r *RouteRegistry) CreateOwned(spec string, owner RouteOwner) error {
	if !validOwner(owner) {
		return ErrInvalidOwner
	}
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
	r.runtime[route.Host] = RuntimeRoute{Route: copyRoute(route), Owner: owner}
	return nil
}

// Update replaces an existing runtime route's target atomically.
func (r *RouteRegistry) Update(spec string) error { return r.UpdateOwned(spec, OwnerManual) }

func (r *RouteRegistry) UpdateOwned(spec string, owner RouteOwner) error {
	if !validOwner(owner) {
		return ErrInvalidOwner
	}
	route, err := config.ParseRoute(spec)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.configured[route.Host]; ok {
		return fmt.Errorf("%s: %w", route.Host, ErrConfiguredRoute)
	}
	if existing, ok := r.runtime[route.Host]; !ok {
		return fmt.Errorf("%s: %w", route.Host, ErrRouteNotFound)
	} else if existing.Owner != owner {
		return fmt.Errorf("%s: %w", route.Host, ErrRouteOwner)
	}
	r.runtime[route.Host] = RuntimeRoute{Route: copyRoute(route), Owner: owner}
	return nil
}

// Remove deletes an ephemeral route; configured routes cannot be deleted.
func (r *RouteRegistry) Remove(host string) error { return r.RemoveOwned(host, OwnerManual) }

func (r *RouteRegistry) RemoveOwned(host string, owner RouteOwner) error {
	if !validOwner(owner) {
		return ErrInvalidOwner
	}
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
	if existing, ok := r.runtime[route.Host]; !ok {
		return fmt.Errorf("%s: %w", route.Host, ErrRouteNotFound)
	} else if existing.Owner != owner {
		return fmt.Errorf("%s: %w", route.Host, ErrRouteOwner)
	}
	delete(r.runtime, route.Host)
	return nil
}

// IsConfigured reports whether a route is owned by the startup configuration.
func (r *RouteRegistry) IsConfigured(host string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.configured[host]
	return ok
}

// SnapshotOwned returns a consistent host-sorted snapshot with ownership metadata.
// Configured routes have no runtime owner.
func (r *RouteRegistry) SnapshotOwned() []RuntimeRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]RuntimeRoute, 0, len(r.configured)+len(r.runtime))
	for _, route := range r.configured {
		result = append(result, RuntimeRoute{Route: copyRoute(route)})
	}
	for _, entry := range r.runtime {
		result = append(result, RuntimeRoute{Route: copyRoute(entry.Route), Owner: entry.Owner})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Route.Host < result[j].Route.Host })
	return result
}
