package dashboard

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/status"
	"github.com/cmdlinezero/clz-lhd/internal/statushttp"
)

const graphHeight = 6

// Lines renders a terminal-sized snapshot without depending on a terminal library.
func Lines(snapshot statushttp.Response, fetchErr error, selectedHost string, width, height int, now time.Time) []string {
	if width < 1 || height < 1 {
		return nil
	}

	lines := []string{"CLZ LHD"}
	if fetchErr != nil {
		lines = append(lines, "", "○ DISCONNECTED", trim("Status API: "+fetchErr.Error(), width), "", "j/k [ navigate ]    g/G [ first/last ]   q [ quit ]")
		return fit(lines, width, height)
	}

	lines[0] = trim(fmt.Sprintf("CLZ LHD%*s● LIVE", max(1, width-13), ""), width)
	lines = append(lines, "", trim(fmt.Sprintf("HTTP %s    HTTPS %s    In flight: %d", listener(snapshot.HTTP), listener(snapshot.HTTPS), snapshot.InFlight), width))

	// The traffic graph is useful context, but route navigation and detail take
	// priority on short terminals. Omitting the graph below this height leaves
	// enough room for the selected route, its detail panel, and the controls.
	if height >= 23 {
		lines = append(lines, "", "TRAFFIC — requests/sec · last 60s")
		lines = append(lines, graph(snapshot.Traffic, width)...)
	}
	lines = append(lines, "", "ROUTES")

	tail := []string{}
	if route, ok := selectedRoute(snapshot.Routes, selectedHost); ok {
		tail = append(tail, "", "ROUTE DETAIL")
		tail = append(tail, routeDetail(route, now, width)...)
	}
	tail = append(tail, "")
	age := now.Sub(snapshot.At).Round(time.Second)
	if age < 0 {
		age = 0
	}
	tail = append(tail, trim(fmt.Sprintf("j/k [ navigate ]    g/G [ first/last ]   q [ quit ]    updated %s ago", age), width))

	availableRoutes := max(0, height-len(lines)-len(tail))
	for _, route := range visibleRoutes(snapshot.Routes, selectedHost, availableRoutes) {
		lines = append(lines, trim(routeLine(route, route.Host == selectedHost, width), width))
	}
	lines = append(lines, tail...)
	return fit(lines, width, height)
}

func visibleRoutes(routes []statushttp.Route, selectedHost string, limit int) []statushttp.Route {
	if limit <= 0 || len(routes) == 0 {
		return nil
	}
	if len(routes) <= limit {
		return routes
	}

	selected := selectedIndex(routes, selectedHost)
	start := selected - limit/2
	if start < 0 {
		start = 0
	}
	if start+limit > len(routes) {
		start = len(routes) - limit
	}
	return routes[start : start+limit]
}

func listener(l statushttp.Listener) string {
	if !l.Listening {
		return "○ —"
	}
	return "● " + l.Address
}

func routeLine(route statushttp.Route, selected bool, width int) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}
	target := route.Target
	if parsed, err := url.Parse(route.Target); err == nil && parsed.Host != "" {
		target = parsed.Host
	}
	state := routeState(route.Reachability)
	last := "—"
	if route.LastStatus != 0 {
		last = fmt.Sprintf("%d %.0fms", route.LastStatus, route.LastDurationMS)
	}

	// Preserve the state and recent result on ordinary terminals, while using a
	// compact layout before truncation becomes necessary on narrow terminals.
	if width < 72 {
		return fmt.Sprintf("%s%s %-18s → %-18s %s", prefix, state, route.Host, target, last)
	}
	return fmt.Sprintf("%s%s %-22s → %-22s %6d req  %s", prefix, state, route.Host, target, route.Requests, last)
}

func selectedRoute(routes []statushttp.Route, host string) (statushttp.Route, bool) {
	for _, route := range routes {
		if route.Host == host {
			return route, true
		}
	}
	return statushttp.Route{}, false
}

func routeDetail(route statushttp.Route, now time.Time, width int) []string {
	last := "—"
	if route.LastStatus != 0 {
		age := now.Sub(route.LastRequest).Round(time.Second)
		if age < 0 {
			age = 0
		}
		last = fmt.Sprintf("%d · %.0fms · %s ago", route.LastStatus, route.LastDurationMS, age)
	}
	return []string{
		trim(fmt.Sprintf("Host          %s", route.Host), width),
		trim(fmt.Sprintf("Target        %s", route.Target), width),
		trim(fmt.Sprintf("Reachability  %s", routeState(route.Reachability)), width),
		trim(fmt.Sprintf("Requests      %d", route.Requests), width),
		trim(fmt.Sprintf("Last request  %s", last), width),
	}
}

func routeState(reachability status.Reachability) string {
	switch reachability {
	case status.ReachabilityConnected:
		return "● CONNECTED"
	case status.ReachabilityRefused:
		return "× REFUSED"
	case status.ReachabilityTimeout:
		return "! TIMEOUT"
	default:
		return "○ UNKNOWN"
	}
}

func graph(buckets []statushttp.TrafficBucket, width int) []string {
	plotWidth := min(60, max(10, width-7))
	values := make([]uint64, plotWidth)
	start := max(0, len(buckets)-plotWidth)
	for i, bucket := range buckets[start:] {
		values[plotWidth-len(buckets[start:])+i] = bucket.Requests
	}

	var peak uint64
	for _, value := range values {
		if value > peak {
			peak = value
		}
	}
	if peak == 0 {
		peak = 1
	}

	lines := make([]string, 0, graphHeight+1)
	for row := graphHeight; row >= 1; row-- {
		label := "   "
		if row == graphHeight {
			label = fmt.Sprintf("%3d", peak)
		} else if peak > 1 && row == graphHeight/2 {
			label = fmt.Sprintf("%3d", max(1, int((peak+1)/2)))
		}

		var b strings.Builder
		fmt.Fprintf(&b, "%s ┤", label)
		for _, value := range values {
			filledRows := int((value*graphHeight + peak - 1) / peak)
			if filledRows >= row {
				b.WriteRune('█')
			} else {
				b.WriteRune(' ')
			}
		}
		lines = append(lines, b.String())
	}
	lines = append(lines, fmt.Sprintf("  0 ┼%s", strings.Repeat("─", plotWidth)))
	return lines
}

func fit(lines []string, width, height int) []string {
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = trim(lines[i], width)
	}
	return lines
}

func trim(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}
