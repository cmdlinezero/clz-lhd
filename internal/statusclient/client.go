package statusclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cmdlinezero/clz-lhd/internal/statushttp"
)

// Client reads the loopback runtime status API.
type Client struct {
	URL        string
	HTTPClient *http.Client
}

func NewClient(port string) *Client {
	return &Client{
		URL:        "http://127.0.0.1:" + port + "/v1/status",
		HTTPClient: http.DefaultClient,
	}
}

func (c *Client) Snapshot(ctx context.Context) (statushttp.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return statushttp.Response{}, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return statushttp.Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return statushttp.Response{}, fmt.Errorf("status API returned %s", resp.Status)
	}

	var snapshot statushttp.Response
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return statushttp.Response{}, fmt.Errorf("decode status API: %w", err)
	}
	return snapshot, nil
}

// Requests retrieves the bounded, newest-first lifecycle event snapshot.
func (c *Client) Requests(ctx context.Context) (statushttp.RequestsResponse, error) {
	url := strings.TrimSuffix(c.URL, "/v1/status") + "/v1/requests"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return statushttp.RequestsResponse{}, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return statushttp.RequestsResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statushttp.RequestsResponse{}, fmt.Errorf("requests API returned %s", resp.Status)
	}
	var result statushttp.RequestsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return statushttp.RequestsResponse{}, fmt.Errorf("decode requests API: %w", err)
	}
	return result, nil
}
