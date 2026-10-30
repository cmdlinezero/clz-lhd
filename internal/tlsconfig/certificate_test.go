package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadCertificate(t *testing.T) {
	now := time.Now()
	certFile, keyFile := writeCertificatePair(t, []string{"app.localhost", "api.localhost"}, now.Add(-time.Hour), now.Add(time.Hour))

	cert, err := Load(certFile, keyFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cert.Leaf.DNSNames, []string{"app.localhost", "api.localhost"}; !equalStrings(got, want) {
		t.Fatalf("DNSNames = %v, want %v", got, want)
	}
	if !cert.ValidAt(now) {
		t.Fatal("ValidAt(now) = false, want true")
	}
	if cert.CertFile != certFile || cert.KeyFile != keyFile {
		t.Fatalf("paths = (%q, %q), want (%q, %q)", cert.CertFile, cert.KeyFile, certFile, keyFile)
	}
}

func TestLoadRequiresCertificateAndKey(t *testing.T) {
	tests := []struct {
		name     string
		certFile string
		keyFile  string
		want     string
	}{
		{name: "neither", want: "certificate and key files are required"},
		{name: "certificate only", certFile: "cert.pem", want: "key file is required"},
		{name: "key only", keyFile: "key.pem", want: "certificate file is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.certFile, tt.keyFile)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadRejectsMismatchedKey(t *testing.T) {
	now := time.Now()
	certFile, _ := writeCertificatePair(t, []string{"app.localhost"}, now.Add(-time.Hour), now.Add(time.Hour))
	_, otherKey := writeCertificatePair(t, []string{"other.localhost"}, now.Add(-time.Hour), now.Add(time.Hour))

	_, err := Load(certFile, otherKey)
	if err == nil || !strings.Contains(err.Error(), "private key does not match public key") {
		t.Fatalf("Load() error = %v, want key mismatch", err)
	}
}

func TestLoadRejectsMalformedCertificate(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(certFile, keyFile); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestValidAtRejectsExpiredCertificate(t *testing.T) {
	now := time.Now()
	certFile, keyFile := writeCertificatePair(t, []string{"app.localhost"}, now.Add(-2*time.Hour), now.Add(-time.Hour))
	cert, err := Load(certFile, keyFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cert.ValidAt(now) {
		t.Fatal("ValidAt(now) = true, want false")
	}
}

func writeCertificatePair(t *testing.T, dnsNames []string, notBefore, notAfter time.Time) (string, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsNames[0]},
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
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
