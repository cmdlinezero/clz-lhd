package cmd

import (
	"clz-lhd/internal/proxy"
	_ "fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Start the proxy with host mappings",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetString("port")
		routes, _ := cmd.Flags().GetStringToString("route")

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			host := strings.Split(r.Host, ":")[0]

			targetPort, ok := routes[host]
			if !ok {
				http.Error(w, "Proxy: Host not configured", 404)
				return
			}

			// Format: "8081" -> "http://localhost:8081"
			targetAddr := targetPort
			if !strings.HasPrefix(targetAddr, "http") {
				targetAddr = "http://localhost:" + targetPort
			}

			p, _ := proxy.NewReverseProxy(targetAddr)
			p.ServeHTTP(w, r)

			// Update Prometheus
			proxy.HttpRequestDuration.WithLabelValues(host).Observe(time.Since(start).Seconds())
			proxy.HttpRequestsTotal.WithLabelValues(host, "200").Inc()
		})

		log.Printf("Listening on :%s", port)
		log.Fatal(http.ListenAndServe(":"+port, mux))
	},
}

func init() {
	routeCmd.Flags().StringToStringP("route", "r", nil, "host=port mappings")
	rootCmd.AddCommand(routeCmd)
}
