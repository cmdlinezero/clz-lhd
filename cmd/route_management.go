package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// routeAPIClient communicates only with the local status listener.
type routeAPIClient struct {
	port       string
	token      string
	httpClient *http.Client
}

func newRouteAPIClient(cmd *cobra.Command) routeAPIClient {
	port, _ := cmd.Flags().GetString("status-port")
	return routeAPIClient{port: port, token: os.Getenv("LHD_MANAGEMENT_TOKEN"), httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func (c routeAPIClient) request(method, path string, payload any) ([]byte, error) {
	if method != http.MethodGet && c.token == "" {
		return nil, errors.New("LHD_MANAGEMENT_TOKEN is required for route mutations")
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	// Port is a CLI flag, not an arbitrary URL. Reject anything other than a TCP port.
	if c.port == "" || strings.Trim(c.port, "0123456789") != "" {
		return nil, fmt.Errorf("invalid status port %q", c.port)
	}
	endpoint := "http://127.0.0.1:" + c.port + path
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("management API unavailable: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8192))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("management API returned %s: %s", response.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func runRouteMutation(cmd *cobra.Command, method string, args []string) error {
	client := newRouteAPIClient(cmd)
	var payload any
	path := "/v1/routes"
	if method == http.MethodDelete {
		path += "/" + url.PathEscape(args[0])
	} else {
		payload = struct {
			Host   string `json:"host"`
			Target string `json:"target"`
		}{Host: args[0], Target: args[1]}
	}
	if _, err := client.request(method, path, payload); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Route %s: %s\n", map[string]string{http.MethodPost: "added", http.MethodPut: "updated", http.MethodDelete: "removed"}[method], args[0])
	return nil
}

func init() {
	for _, action := range []struct {
		name, description, method string
		count                     int
	}{
		{"add", "Create a temporary route in the running LHD server", http.MethodPost, 2},
		{"update", "Update a temporary route in the running LHD server", http.MethodPut, 2},
		{"remove", "Remove a temporary route from the running LHD server", http.MethodDelete, 1},
	} {
		action := action
		usage := action.name + " <host> <target>"
		if action.count == 1 {
			usage = action.name + " <host>"
		}
		routeCmd.AddCommand(&cobra.Command{
			Use: usage, Short: action.description, Args: cobra.ExactArgs(action.count),
			RunE: func(cmd *cobra.Command, args []string) error { return runRouteMutation(cmd, action.method, args) },
		})
	}
}
