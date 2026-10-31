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
func Lines(snapshot statushttp.Response, fetchErr error, width, height int, now time.Time) []string {
	if width < 1 || height < 1 {
		return nil
	}

	lines := []string{"CLZ LHD"}
	if fetchErr != nil {
		lines = append(lines, "", "○ DISCONNECTED", trim("Status API: "+fetchErr.Error(), width), "", "q quit")
		return fit(lines, width, height)
	}

	lines[0] = trim(fmt.Sprintf("CLZ LHD%*s● LIVE", max(1, width-13), ""), width)
	lines = append(lines, "", trim(fmt.Sprintf("HTTP %s    HTTPS %s    In flight: %d", listener(snapshot.HTTP), listener(snapshot.HTTPS), snapshot.InFlight), width))
	lines = append(lines, "", "TRAFFIC — requests/sec · last 60s")
	lines = append(lines, graph(snapshot.Traffic, width)...)
	lines = append(lines, "", "ROUTES")
	for _, route := range snapshot.Routes {
		lines = append(lines, trim(routeLine(route, width), width))
	}
	lines = append(lines, "")
	age := now.Sub(snapshot.At).Round(time.Second)
	if age < 0 {
		age = 0
	}
	lines = append(lines, trim(fmt.Sprintf("q quit    updated %s ago", age), width))
	return fit(lines, width, height)
}

func listener(l statushttp.Listener) string {
	if !l.Listening {
		return "○ —"
	}
	return "● " + l.Address
}

func routeLine(route statushttp.Route, width int) string {
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
		return fmt.Sprintf("%s %-18s → %-18s %s", state, route.Host, target, last)
	}
	return fmt.Sprintf("%s %-22s → %-22s %6d req  %s", state, route.Host, target, route.Requests, last)
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
