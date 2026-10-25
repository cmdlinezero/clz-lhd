package doctor

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
)

// Check is the result of a single diagnostic check.
type Check struct {
	OK     bool
	Label  string
	Detail string
}

// Report contains the diagnostics for one configuration.
type Report struct {
	Checks []Check
}

// Problems returns the number of failed checks in the report.
func (r Report) Problems() int {
	problems := 0
	for _, check := range r.Checks {
		if !check.OK {
			problems++
		}
	}
	return problems
}

// Run loads path and checks whether the proxy port is available and each
// configured upstream accepts TCP connections.
func Run(path, port string, timeout time.Duration) (Report, error) {
	routes, err := config.LoadFile(path)
	if err != nil {
		return Report{}, err
	}

	report := Report{Checks: []Check{
		{OK: true, Label: fmt.Sprintf("%s loaded", path)},
		{OK: true, Label: fmt.Sprintf("%d routes configured", len(routes))},
	}}

	report.Checks = append(report.Checks, checkPort(port))
	for _, route := range routes {
		report.Checks = append(report.Checks, checkUpstream(route, timeout))
	}

	return report, nil
}

func checkPort(port string) Check {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return Check{
			Label:  fmt.Sprintf("port %s available", port),
			Detail: err.Error(),
		}
	}
	listener.Close()
	return Check{OK: true, Label: fmt.Sprintf("port %s available", port)}
}

func checkUpstream(route config.Route, timeout time.Duration) Check {
	address, err := targetAddress(route.Target)
	label := fmt.Sprintf("%s -> %s reachable", route.Host, route.Target.Host)
	if err != nil {
		return Check{Label: label, Detail: err.Error()}
	}

	connection, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return Check{Label: label, Detail: err.Error()}
	}
	connection.Close()
	return Check{OK: true, Label: label}
}

func targetAddress(target *url.URL) (string, error) {
	port := target.Port()
	if port == "" {
		switch target.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", fmt.Errorf("unsupported target scheme %q", target.Scheme)
		}
	}
	return net.JoinHostPort(target.Hostname(), port), nil
}
