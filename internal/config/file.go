package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// TLS describes incoming HTTPS certificate files.
type TLS struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

// File describes the on-disk lhd.yaml configuration.
type File struct {
	Routes map[string]string `yaml:"routes"`
	TLS    *TLS              `yaml:"tls,omitempty"`
}

// Project is the validated project configuration used by the CLI.
type Project struct {
	Routes []Route
	TLS    *TLS
}

// LoadProject reads and validates project configuration from a YAML file.
// Relative TLS paths are resolved from the directory containing the config file.
func LoadProject(path string) (Project, error) {
	file, err := os.Open(path)
	if err != nil {
		return Project{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	project, err := DecodeProject(file)
	if err != nil {
		return Project{}, fmt.Errorf("load config %q: %w", path, err)
	}
	if project.TLS != nil {
		base := filepath.Dir(path)
		project.TLS.Cert = resolvePath(base, project.TLS.Cert)
		project.TLS.Key = resolvePath(base, project.TLS.Key)
	}
	return project, nil
}

// LoadFile reads and validates routes from a YAML configuration file.
func LoadFile(path string) ([]Route, error) {
	project, err := LoadProject(path)
	if err != nil {
		return nil, err
	}
	return project.Routes, nil
}

// DecodeProject decodes and validates YAML project configuration from r.
func DecodeProject(r io.Reader) (Project, error) {
	var file File
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return Project{}, fmt.Errorf("decode YAML: %w", err)
	}
	if len(file.Routes) == 0 {
		return Project{}, fmt.Errorf("no routes configured")
	}
	if file.TLS != nil {
		if file.TLS.Cert == "" || file.TLS.Key == "" {
			return Project{}, fmt.Errorf("tls.cert and tls.key must be configured together")
		}
	}

	specs := make([]string, 0, len(file.Routes))
	for host, target := range file.Routes {
		specs = append(specs, host+"="+target)
	}

	routes, err := ParseRoutes(specs)
	if err != nil {
		return Project{}, fmt.Errorf("validate routes: %w", err)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Host < routes[j].Host
	})
	return Project{Routes: routes, TLS: file.TLS}, nil
}

// DecodeFile decodes and validates YAML configuration from r.
func DecodeFile(r io.Reader) ([]Route, error) {
	project, err := DecodeProject(r)
	if err != nil {
		return nil, err
	}
	return project.Routes, nil
}

func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}
