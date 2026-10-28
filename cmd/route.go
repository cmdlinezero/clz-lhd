package cmd

import (
	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/spf13/cobra"
)

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Start the proxy with host mappings",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		routeSpecs, _ := cmd.Flags().GetStringArray("route")
		routes, err := config.ParseRoutes(routeSpecs)
		if err != nil {
			return err
		}

		return serve(port, routes)
	},
}

func init() {
	routeCmd.Flags().StringArrayP("route", "r", nil, "host=target mappings")
	rootCmd.AddCommand(routeCmd)
}
