package cmd

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func serve(port string, routes []config.Route) error {
	log.Print(startupSummary(port, routes))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", proxy.NewRouter(routes))

	return http.ListenAndServe(":"+port, mux)
}

func startupSummary(port string, routes []config.Route) string {
	var summary strings.Builder

	summary.WriteString("Routes:\n")
	for _, route := range routes {
		fmt.Fprintf(&summary, "  http://%s -> %s\n", route.Host, route.Target)
	}
	fmt.Fprintf(&summary, "\nListening on :%s\n", port)
	fmt.Fprintf(&summary, "Metrics: http://localhost:%s/metrics", port)

	return summary.String()
}
