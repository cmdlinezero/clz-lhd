package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"

	"github.com/cmdlinezero/clz-lhd/internal/worktree"
	"github.com/spf13/cobra"
)

// Worktree route identity is inferred from the hostname format produced by
// worktree.Hostname. The API does not yet store worktree ownership metadata.
var worktreeHostPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?-[0-9a-f]{10}\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.localhost$`)

func worktreeRoutes(routes []managedRoute) []managedRoute {
	result := make([]managedRoute, 0)
	for _, r := range routes {
		if r.Source != "runtime" || !worktreeHostPattern.MatchString(r.Host) {
			continue
		}
		if _, ok := worktree.LoopbackPort(r.Target); !ok {
			continue
		}
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Host < result[j].Host })
	return result
}

func removeWorktreeRoute(client routeAPIClient, info worktree.Info) (bool, error) {
	if client.token == "" {
		return false, fmt.Errorf("LHD_MANAGEMENT_TOKEN is required to remove worktree routes")
	}
	routes, err := listManagedRoutes(client)
	if err != nil {
		return false, err
	}
	for _, r := range routes {
		if r.Host != info.Host {
			continue
		}
		if r.Source != "runtime" {
			return false, fmt.Errorf("worktree hostname %s is owned by configuration; refusing removal", info.Host)
		}
		if !worktreeHostPattern.MatchString(r.Host) {
			return false, fmt.Errorf("route %s is not a worktree-style hostname", info.Host)
		}
		if _, ok := worktree.LoopbackPort(r.Target); !ok {
			return false, fmt.Errorf("route %s is not a loopback HTTP(S) worktree target; refusing removal", info.Host)
		}
		_, err = client.request(http.MethodDelete, "/v1/routes/"+url.PathEscape(info.Host), nil)
		if err != nil {
			return false, fmt.Errorf("remove worktree route: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func init() {
	worktreeCmd.AddCommand(&cobra.Command{Use: "list", Short: "List runtime routes matching the worktree hostname convention", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		routes, err := listManagedRoutes(newRouteAPIClient(cmd))
		if err != nil {
			return err
		}
		matches := worktreeRoutes(routes)
		if len(matches) == 0 {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "No registered worktree-style routes.")
			return err
		}
		for _, r := range matches {
			port, _ := worktree.LoopbackPort(r.Target)
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\t%s\n", r.Host, port, r.Target); err != nil {
				return err
			}
		}
		return nil
	}})
	worktreeCmd.AddCommand(&cobra.Command{Use: "remove", Short: "Unregister the current worktree route (does not delete files or stop services)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		info, err := worktree.DiscoverCurrent()
		if err != nil {
			return err
		}
		removed, err := removeWorktreeRoute(newRouteAPIClient(cmd), info)
		if err != nil {
			return err
		}
		if removed {
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree route %s\n", info.Host)
		} else {
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "No worktree route registered for %s\n", info.Host)
		}
		return err
	}})
}
