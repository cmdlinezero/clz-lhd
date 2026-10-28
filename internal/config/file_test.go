package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeFile(t *testing.T) {
	routes, err := DecodeFile(strings.NewReader(`
routes:
  app.localhost: "3000"
  api.localhost: "http://127.0.0.1:8080"
`))
	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("len(routes) = %d, want 2", len(routes))
	}

	got := make(map[string]string, len(routes))
	for _, route := range routes {
		got[route.Host] = route.Target.String()
	}
	if got["app.localhost"] != "http://localhost:3000" {
		t.Errorf("app.localhost target = %q", got["app.localhost"])
	}
	if got["api.localhost"] != "http://127.0.0.1:8080" {
		t.Errorf("api.localhost target = %q", got["api.localhost"])
	}
}

func TestDecodeFileAcceptsNumericPort(t *testing.T) {
	routes, err := DecodeFile(strings.NewReader("routes:\n  app.localhost: 3000\n"))
	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	if got, want := routes[0].Target.String(), "http://localhost:3000"; got != want {
		t.Errorf("Target = %q, want %q", got, want)
	}
}

func TestDecodeFileRejectsInvalidConfiguration(t *testing.T) {
	tests := map[string]string{
		"empty":          "{}\n",
		"unknown field":  "routes: {}\nother: true\n",
		"invalid route":  "routes:\n  bad_host.localhost: 3000\n",
		"invalid target": "routes:\n  app.localhost: ftp://localhost:21\n",
		"malformed YAML": "routes: [\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFile(strings.NewReader(input)); err == nil {
				t.Fatal("DecodeFile() error = nil, want error")
			}
		})
	}
}

func TestDecodeFileRejectsDuplicateYAMLKeys(t *testing.T) {
	_, err := DecodeFile(strings.NewReader(`
routes:
  app.localhost: "3000"
  app.localhost: "4000"
`))
	if err == nil {
		t.Fatal("DecodeFile() error = nil, want duplicate-key error")
	}
}

func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lhd.yaml")
	if err := os.WriteFile(path, []byte("routes:\n  app.localhost: 3000\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	routes, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if got, want := routes[0].Host, "app.localhost"; got != want {
		t.Errorf("Host = %q, want %q", got, want)
	}
}

func TestLoadFileReportsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("LoadFile() error = nil, want error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not contain path %q", err, path)
	}
}

func TestDecodeFileSortsRoutesByNormalizedHost(t *testing.T) {
	routes, err := DecodeFile(strings.NewReader(`
routes:
  ZETA.LOCALHOST: "3000"
  api.localhost: "4000"
  App.localhost.: "5000"
`))
	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}

	want := []string{"api.localhost", "app.localhost", "zeta.localhost"}
	for i, route := range routes {
		if route.Host != want[i] {
			t.Fatalf("routes[%d].Host = %q, want %q", i, route.Host, want[i])
		}
	}
}
