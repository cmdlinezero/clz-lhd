package doctor

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunReportsAvailablePortAndReachableUpstream(t *testing.T) {
	upstream := listen(t)
	defer upstream.Close()

	proxyProbe := listen(t)
	_, proxyPort, err := net.SplitHostPort(proxyProbe.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	proxyProbe.Close()

	_, upstreamPort, err := net.SplitHostPort(upstream.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	path := writeConfig(t, fmt.Sprintf("routes:\n  app.localhost: http://127.0.0.1:%s\n", upstreamPort))
	report, err := Run(path, proxyPort, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if problems := report.Problems(); problems != 0 {
		t.Fatalf("Problems() = %d, want 0; report = %#v", problems, report)
	}
}

func TestRunReportsUnavailablePortAndUpstream(t *testing.T) {
	occupied := listenAny(t)
	defer occupied.Close()
	_, occupiedPort, err := net.SplitHostPort(occupied.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	unreachable := listen(t)
	_, unreachablePort, err := net.SplitHostPort(unreachable.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	unreachable.Close()

	path := writeConfig(t, fmt.Sprintf("routes:\n  api.localhost: http://127.0.0.1:%s\n", unreachablePort))
	report, err := Run(path, occupiedPort, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if problems := report.Problems(); problems != 2 {
		t.Fatalf("Problems() = %d, want 2; report = %#v", problems, report)
	}
}

func TestRunReturnsConfigError(t *testing.T) {
	_, err := Run(filepath.Join(t.TempDir(), "missing.yaml"), "8080", time.Second)
	if err == nil {
		t.Fatal("Run() error = nil, want config error")
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listener
}

func listenAny(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	return listener
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lhd.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTargetAddressUsesDefaultSchemePorts(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"http://localhost", net.JoinHostPort("localhost", "80")},
		{"https://localhost", net.JoinHostPort("localhost", "443")},
	}

	for _, test := range tests {
		t.Run(test.target, func(t *testing.T) {
			target, err := url.Parse(test.target)
			if err != nil {
				t.Fatal(err)
			}
			got, err := targetAddress(target)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("targetAddress() = %q, want %q", got, test.want)
			}
		})
	}
}
