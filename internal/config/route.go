package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Route maps a normalized request host to an upstream target.
type Route struct {
	Host   string
	Target *url.URL
}

// ParseRoutes parses host=target route specifications and rejects duplicate
// hosts after normalization.
func ParseRoutes(specs []string) ([]Route, error) {
	routes := make([]Route, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))

	for _, spec := range specs {
		route, err := ParseRoute(spec)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[route.Host]; ok {
			return nil, fmt.Errorf("duplicate route for host %q", route.Host)
		}
		seen[route.Host] = struct{}{}
		routes = append(routes, route)
	}

	return routes, nil
}

// ParseRoute parses a host=target route specification. A numeric target is
// shorthand for http://localhost:<port>; explicit http and https URLs are also
// accepted.
func ParseRoute(spec string) (Route, error) {
	host, target, ok := strings.Cut(spec, "=")
	if !ok {
		return Route{}, fmt.Errorf("invalid route %q: expected host=target", spec)
	}

	host = normalizeHost(host)
	if err := validateHost(host); err != nil {
		return Route{}, fmt.Errorf("invalid route %q: %w", spec, err)
	}

	targetURL, err := parseTarget(strings.TrimSpace(target))
	if err != nil {
		return Route{}, fmt.Errorf("invalid route %q: %w", spec, err)
	}

	return Route{Host: host, Target: targetURL}, nil
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func validateHost(host string) error {
	if host == "" {
		return fmt.Errorf("host is empty")
	}
	if len(host) > 253 {
		return fmt.Errorf("host %q is too long", host)
	}

	for _, label := range strings.Split(host, ".") {
		if label == "" {
			return fmt.Errorf("host %q contains an empty label", host)
		}
		if len(label) > 63 {
			return fmt.Errorf("host %q contains a label longer than 63 characters", host)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("host %q contains a label beginning or ending with '-'", host)
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return fmt.Errorf("host %q contains invalid character %q", host, r)
		}
	}
	return nil
}

func parseTarget(target string) (*url.URL, error) {
	if target == "" {
		return nil, fmt.Errorf("target is empty")
	}

	if port, err := strconv.Atoi(target); err == nil {
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port %d is outside the range 1-65535", port)
		}
		return url.Parse("http://localhost:" + strconv.Itoa(port))
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target %q: %w", target, err)
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		return nil, fmt.Errorf("target %q must use http or https", target)
	}
	if targetURL.Host == "" {
		return nil, fmt.Errorf("target %q has no host", target)
	}

	return targetURL, nil
}
