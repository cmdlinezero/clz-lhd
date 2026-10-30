package doctor

import (
	"crypto/x509/pkix"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/tlsconfig"
)

// Check is the result of a single diagnostic check.
type Check struct {
	OK            bool
	Label         string
	Detail        string
	VerboseDetail string
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

// Run loads path and checks listener ports, TLS material when configured, and
// whether each configured upstream accepts TCP connections.
func Run(path, port, tlsPort string, timeout time.Duration) (Report, error) {
	project, err := config.LoadProject(path)
	if err != nil {
		return Report{}, err
	}

	report := Report{Checks: []Check{
		{OK: true, Label: fmt.Sprintf("%s loaded", path)},
		{OK: true, Label: fmt.Sprintf("%d routes configured", len(project.Routes))},
	}}

	report.Checks = append(report.Checks, checkPort("HTTP", port))
	if project.TLS != nil {
		report.Checks = append(report.Checks, checkTLS(project, tlsPort, time.Now())...)
	}
	for _, route := range project.Routes {
		report.Checks = append(report.Checks, checkUpstream(route, timeout))
	}

	return report, nil
}

func checkPort(protocol, port string) Check {
	label := fmt.Sprintf("%s port %s available", protocol, port)
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return Check{Label: label, Detail: err.Error()}
	}
	listener.Close()
	return Check{OK: true, Label: label}
}

func checkTLS(project config.Project, tlsPort string, now time.Time) []Check {
	certificate, err := tlsconfig.Load(project.TLS.Cert, project.TLS.Key)
	if err != nil {
		return []Check{
			{Label: "TLS certificate and key load", Detail: err.Error()},
			checkPort("HTTPS", tlsPort),
		}
	}

	checks := []Check{{
		OK:     true,
		Label:  "TLS certificate and key load",
		Detail: fmt.Sprintf("certificate: %s; key: %s", certificate.CertFile, certificate.KeyFile),
	}}

	validity := Check{
		OK:     certificate.ValidAt(now),
		Label:  "TLS certificate currently valid",
		Detail: fmt.Sprintf("valid from %s; expires %s", formatTime(certificate.Leaf.NotBefore), formatTime(certificate.Leaf.NotAfter)),
	}
	if !validity.OK {
		if now.Before(certificate.Leaf.NotBefore) {
			validity.Detail = "not valid until " + formatTime(certificate.Leaf.NotBefore)
		} else {
			validity.Detail = "expired " + formatTime(certificate.Leaf.NotAfter)
		}
	}
	checks = append(checks, validity)

	dnsNames := append([]string(nil), certificate.Leaf.DNSNames...)
	sort.Strings(dnsNames)
	issuer := friendlyName(certificate.Leaf.Issuer)
	checks = append(checks, Check{
		OK:    true,
		Label: "TLS certificate details",
		Detail: fmt.Sprintf(
			"issuer: %s; expires: %s; DNS names: %s",
			issuer, certificate.Leaf.NotAfter.Format(time.DateOnly), strings.Join(dnsNames, ", "),
		),
		VerboseDetail: fmt.Sprintf(
			"subject: %s; issuer: %s; serial: %s; valid from: %s; expires: %s; DNS names: %s; signature algorithm: %s",
			nameOrUnknown(certificate.Leaf.Subject.String()),
			nameOrUnknown(certificate.Leaf.Issuer.String()),
			certificate.Leaf.SerialNumber.String(),
			formatTime(certificate.Leaf.NotBefore),
			formatTime(certificate.Leaf.NotAfter),
			strings.Join(dnsNames, ", "),
			certificate.Leaf.SignatureAlgorithm.String(),
		),
	})

	var uncovered []string
	for _, route := range project.Routes {
		if err := certificate.Leaf.VerifyHostname(route.Host); err != nil {
			uncovered = append(uncovered, route.Host)
		}
	}
	coverage := Check{OK: len(uncovered) == 0, Label: "TLS certificate covers configured routes"}
	if len(uncovered) > 0 {
		coverage.Detail = "not covered: " + strings.Join(uncovered, ", ")
	}
	checks = append(checks, coverage, checkPort("HTTPS", tlsPort))
	return checks
}

func formatTime(value time.Time) string {
	return value.Format(time.RFC3339)
}

func friendlyName(name pkix.Name) string {
	if len(name.Organization) > 0 {
		return strings.Join(name.Organization, ", ")
	}
	if name.CommonName != "" {
		return name.CommonName
	}
	return nameOrUnknown(name.String())
}

func nameOrUnknown(name string) string {
	if name == "" {
		return "unknown"
	}
	return name
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
