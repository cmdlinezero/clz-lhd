package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// File describes the on-disk lhd.yaml configuration.
type File struct {
	Routes map[string]string `yaml:"routes"`
}

// LoadFile reads and validates routes from a YAML configuration file.
func LoadFile(path string) ([]Route, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	routes, err := DecodeFile(file)
	if err != nil {
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}
	return routes, nil
}

// DecodeFile decodes and validates YAML configuration from r.
func DecodeFile(r io.Reader) ([]Route, error) {
	var file File
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode YAML: %w", err)
	}
	if len(file.Routes) == 0 {
		return nil, fmt.Errorf("no routes configured")
	}

	specs := make([]string, 0, len(file.Routes))
	for host, target := range file.Routes {
		specs = append(specs, host+"="+target)
	}

	routes, err := ParseRoutes(specs)
	if err != nil {
		return nil, fmt.Errorf("validate routes: %w", err)
	}
	return routes, nil
}
