package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"
)

// Certificate contains a usable TLS certificate together with the parsed leaf
// certificate metadata used by diagnostics.
type Certificate struct {
	TLS      tls.Certificate
	Leaf     *x509.Certificate
	CertFile string
	KeyFile  string
}

// Load reads a PEM certificate and private key, verifies that they form a
// usable pair, and parses the leaf certificate for later diagnostics.
func Load(certFile, keyFile string) (*Certificate, error) {
	if certFile == "" && keyFile == "" {
		return nil, fmt.Errorf("certificate and key files are required")
	}
	if certFile == "" {
		return nil, fmt.Errorf("certificate file is required when a key file is configured")
	}
	if keyFile == "" {
		return nil, fmt.Errorf("key file is required when a certificate file is configured")
	}

	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("read certificate %q: %w", certFile, err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("read private key %q: %w", keyFile, err)
	}

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load certificate pair: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return nil, fmt.Errorf("load certificate pair: certificate chain is empty")
	}

	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse leaf certificate: %w", err)
	}
	pair.Leaf = leaf

	return &Certificate{
		TLS:      pair,
		Leaf:     leaf,
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}

// ValidAt reports whether the certificate is valid at the supplied time. It
// intentionally does not check hostname coverage or trust; those are separate
// diagnostic concerns.
func (c *Certificate) ValidAt(at time.Time) bool {
	return !at.Before(c.Leaf.NotBefore) && !at.After(c.Leaf.NotAfter)
}
