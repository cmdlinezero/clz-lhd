package cmd

import (
	"log"
	"net/http"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func serve(port string, routes []config.Route) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", proxy.NewRouter(routes))

	log.Printf("Listening on :%s", port)
	return http.ListenAndServe(":"+port, mux)
}
