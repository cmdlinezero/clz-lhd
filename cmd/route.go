package cmd

import (
	"log"
	"net/http"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Start the proxy with host mappings",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		routeSpecs, _ := cmd.Flags().GetStringArray("route")
		routes, err := config.ParseRoutes(routeSpecs)
		if err != nil {
			return err
		}

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.Handle("/", proxy.NewRouter(routes))

		log.Printf("Listening on :%s", port)
		return http.ListenAndServe(":"+port, mux)
	},
}

func init() {
	routeCmd.Flags().StringArrayP("route", "r", nil, "host=target mappings")
	rootCmd.AddCommand(routeCmd)
}
