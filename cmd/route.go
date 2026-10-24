package cmd

import (
	"github.com/cmdlinezero/clz-lhd/internal/proxy"
	"log"
	"net/http"

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

		mux.Handle("/", proxy.NewRouter(routes))

		log.Printf("Listening on :%s", port)
		log.Fatal(http.ListenAndServe(":"+port, mux))
	},
}

func init() {
	routeCmd.Flags().StringToStringP("route", "r", nil, "host=port mappings")
	rootCmd.AddCommand(routeCmd)
}
