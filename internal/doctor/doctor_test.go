package doctor

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	report, err := Run(path, proxyPort, "8443", time.Second)
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
	report, err := Run(path, occupiedPort, "8443", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if problems := report.Problems(); problems != 2 {
		t.Fatalf("Problems() = %d, want 2; report = %#v", problems, report)
	}
}

func TestRunReturnsConfigError(t *testing.T) {
	_, err := Run(filepath.Join(t.TempDir(), "missing.yaml"), "8080", "8443", time.Second)
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

func TestRunReportsTLSCertificateDetailsAndCoverage(t *testing.T) {
	upstream := listen(t)
	defer upstream.Close()
	_, upstreamPort, err := net.SplitHostPort(upstream.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	httpProbe := listenAny(t)
	_, httpPort, err := net.SplitHostPort(httpProbe.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	httpProbe.Close()
	httpsProbe := listenAny(t)
	_, httpsPort, err := net.SplitHostPort(httpsProbe.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	httpsProbe.Close()

	certFile, keyFile := writeDoctorCertificatePair(t, []string{"app.localhost"}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	path := writeConfig(t, fmt.Sprintf("routes:\n  app.localhost: http://127.0.0.1:%s\ntls:\n  cert: %s\n  key: %s\n", upstreamPort, certFile, keyFile))
	report, err := Run(path, httpPort, httpsPort, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if problems := report.Problems(); problems != 0 {
		t.Fatalf("Problems() = %d, want 0; report = %#v", problems, report)
	}
	assertCheck(t, report, "TLS certificate details", true, "DNS names: app.localhost")
	assertCheck(t, report, "TLS certificate covers configured routes", true, "")
	assertCheck(t, report, "HTTPS port "+httpsPort+" available", true, "")
}

func TestRunReportsTLSHostnameAndValidityProblems(t *testing.T) {
	certFile, keyFile := writeDoctorCertificatePair(t, []string{"other.localhost"}, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	path := writeConfig(t, fmt.Sprintf("routes:\n  app.localhost: 65534\ntls:\n  cert: %s\n  key: %s\n", certFile, keyFile))
	report, err := Run(path, "0", "0", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	assertCheck(t, report, "TLS certificate currently valid", false, "expired")
	assertCheck(t, report, "TLS certificate covers configured routes", false, "app.localhost")
}

func assertCheck(t *testing.T, report Report, label string, ok bool, detail string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Label != label {
			continue
		}
		if check.OK != ok {
			t.Fatalf("check %q OK = %v, want %v", label, check.OK, ok)
		}
		if detail != "" && !strings.Contains(check.Detail, detail) {
			t.Fatalf("check %q detail = %q, want containing %q", label, check.Detail, detail)
		}
		return
	}
	t.Fatalf("check %q not found in %#v", label, report.Checks)
}

func writeDoctorCertificatePair(t *testing.T, dnsNames []string, notBefore, notAfter time.Time) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "LHD doctor test"},
		Issuer:       pkix.Name{CommonName: "LHD test CA"},
		DNSNames:     dnsNames,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}
