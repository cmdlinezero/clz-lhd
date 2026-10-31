package dashboard

import (
	"fmt"
	"net/url"
	"strings"
	"time"

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
		lines = append(lines, "", "DISCONNECTED", trim("Status API: "+fetchErr.Error(), width), "", "q quit")
		return fit(lines, width, height)
	}

	lines[0] = trim(fmt.Sprintf("CLZ LHD%*sLIVE", max(1, width-11), ""), width)
	lines = append(lines, "", trim(fmt.Sprintf("HTTP %s    HTTPS %s    In flight: %d", listener(snapshot.HTTP), listener(snapshot.HTTPS), snapshot.InFlight), width))
	lines = append(lines, "", "TRAFFIC — requests/sec · last 60s")
	lines = append(lines, graph(snapshot.Traffic, width)...)
	lines = append(lines, "", "ROUTES")
	for _, route := range snapshot.Routes {
		lines = append(lines, trim(routeLine(route), width))
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
		return "—"
	}
	return l.Address + " ●"
}

func routeLine(route statushttp.Route) string {
	target := route.Target
	if parsed, err := url.Parse(route.Target); err == nil && parsed.Host != "" {
		target = parsed.Host
	}
	last := "—"
	if route.LastStatus != 0 {
		last = fmt.Sprintf("%d %.0fms", route.LastStatus, route.LastDurationMS)
	}
	return fmt.Sprintf("%-22s → %-22s %-10s %6d req  %s", route.Host, target, strings.ToUpper(string(route.Reachability)), route.Requests, last)
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
		threshold := uint64((int(peak)*row + graphHeight - 1) / graphHeight)
		var b strings.Builder
		fmt.Fprintf(&b, "%3d ┤", threshold)
		for _, value := range values {
			if value >= threshold {
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
