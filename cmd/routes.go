package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/spf13/cobra"
)

var routesCmd = &cobra.Command{
	Use:   "routes",
	Short: "Show configured routes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		live, _ := cmd.Flags().GetBool("live")
		if live {
			data, err := newRouteAPIClient(cmd).request("GET", "/v1/routes", nil)
			if err != nil {
				return err
			}
			var result struct {
				Routes []struct{ Host, Target, Source string } `json:"routes"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("decode live routes: %w", err)
			}
			sort.Slice(result.Routes, func(i, j int) bool { return result.Routes[i].Host < result.Routes[j].Host })
			fmt.Fprintln(cmd.OutOrStdout(), "HOST\tTARGET\tSOURCE")
			for _, route := range result.Routes {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", route.Host, route.Target, route.Source)
			}
			return nil
		}
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
	routesCmd.Flags().Bool("live", false, "List routes from the running local LHD server")
	routesCmd.Flags().StringP("config", "c", "lhd.yaml", "path to the configuration file")
	rootCmd.AddCommand(routesCmd)
}
