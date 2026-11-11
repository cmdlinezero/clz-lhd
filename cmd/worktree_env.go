package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cmdlinezero/clz-lhd/internal/worktree"
	"github.com/spf13/cobra"
)

type managedRoute struct {
	Host   string `json:"host"`
	Target string `json:"target"`
	Source string `json:"source"`
}

func listManagedRoutes(client routeAPIClient) ([]managedRoute, error) {
	data, err := client.request(http.MethodGet, "/v1/routes", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Routes []managedRoute `json:"routes"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode management routes: %w", err)
	}
	return result.Routes, nil
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func worktreeExports(host string, port int) string {
	values := [][2]string{{"PORT", strconv.Itoa(port)}, {"LHD_PORT", strconv.Itoa(port)}, {"LHD_HOST", host}, {"LHD_URL", "http://" + host}}
	var b strings.Builder
	for _, v := range values {
		fmt.Fprintf(&b, "export %s=%s\n", v[0], shellQuote(v[1]))
	}
	return b.String()
}

func prepareWorktreeEnv(client routeAPIClient, info worktree.Info, available func(int) bool) (string, error) {
	if client.token == "" {
		return "", fmt.Errorf("LHD_MANAGEMENT_TOKEN is required to register worktree routes")
	}
	routes, err := listManagedRoutes(client)
	if err != nil {
		return "", err
	}
	used := make(map[int]bool)
	for _, r := range routes {
		port, ok := worktree.LoopbackPort(r.Target)
		if ok {
			used[port] = true
		}
		if r.Host != info.Host {
			continue
		}
		if r.Source != "runtime" {
			return "", fmt.Errorf("worktree hostname %s is owned by configuration", info.Host)
		}
		if !ok {
			return "", fmt.Errorf("existing worktree route %s is not a loopback HTTP target with an explicit port", info.Host)
		}
		// Reuse existing registration even if the application is currently listening.
		return worktreeExports(info.Host, port), nil
	}
	port, err := worktree.CandidatePorts(info.Root, used, available)
	if err != nil {
		return "", err
	}
	target := (&url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(port)}).String()
	payload := struct {
		Host   string `json:"host"`
		Target string `json:"target"`
	}{info.Host, target}
	if _, err = client.request(http.MethodPost, "/v1/routes", payload); err != nil {
		return "", fmt.Errorf("register worktree route: %w (no environment exported)", err)
	}
	return worktreeExports(info.Host, port), nil
}

func init() {
	worktreeCmd.AddCommand(&cobra.Command{Use: "env", Short: "Register the current worktree and print POSIX shell exports", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		info, err := worktree.DiscoverCurrent()
		if err != nil {
			return err
		}
		output, err := prepareWorktreeEnv(newRouteAPIClient(cmd), info, worktree.PortAvailable)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), output)
		return err
	}})
}
