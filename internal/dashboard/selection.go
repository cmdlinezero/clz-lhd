package dashboard

import "github.com/cmdlinezero/clz-lhd/internal/statushttp"

// Selection tracks the selected route by hostname so it remains stable across
// refreshed status snapshots.
type Selection struct {
	Host string
}

func (s *Selection) Normalize(routes []statushttp.Route) {
	if len(routes) == 0 {
		s.Host = ""
		return
	}
	for _, route := range routes {
		if route.Host == s.Host {
			return
		}
	}
	s.Host = routes[0].Host
}

func (s *Selection) Move(routes []statushttp.Route, delta int) {
	s.Normalize(routes)
	if len(routes) == 0 {
		return
	}
	index := selectedIndex(routes, s.Host)
	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(routes) {
		index = len(routes) - 1
	}
	s.Host = routes[index].Host
}

func (s *Selection) First(routes []statushttp.Route) {
	if len(routes) > 0 {
		s.Host = routes[0].Host
	}
}

func (s *Selection) Last(routes []statushttp.Route) {
	if len(routes) > 0 {
		s.Host = routes[len(routes)-1].Host
	}
}

func selectedIndex(routes []statushttp.Route, host string) int {
	for i, route := range routes {
		if route.Host == host {
			return i
		}
	}
	return 0
}
