package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/spf13/cobra"
)

var routesCmd = &cobra.Command{
	Use:   "routes",
	Short: "Show configured routes",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		routes, err := config.LoadFile(configPath)
		if err != nil {
			return err
		}

		fmt.Fprint(cmd.OutOrStdout(), routesSummary(routes))
		return nil
	},
}

func routesSummary(routes []config.Route) string {
	routes = append([]config.Route(nil), routes...)
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Host < routes[j].Host
	})

	var b strings.Builder
	fmt.Fprintln(&b, "HOST\tTARGET")
	for _, route := range routes {
		fmt.Fprintf(&b, "%s\t%s\n", route.Host, route.Target.String())
	}
	return b.String()
}

func init() {
	routesCmd.Flags().StringP("config", "c", "lhd.yaml", "path to the configuration file")
	rootCmd.AddCommand(routesCmd)
}
