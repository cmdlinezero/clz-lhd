package config

import "testing"

func TestParseRoutePortShorthand(t *testing.T) {
	route, err := ParseRoute("app.localhost=3000")
	if err != nil {
		t.Fatalf("ParseRoute() error = %v", err)
	}
	if got, want := route.Host, "app.localhost"; got != want {
		t.Errorf("Host = %q, want %q", got, want)
	}
	if got, want := route.Target.String(), "http://localhost:3000"; got != want {
		t.Errorf("Target = %q, want %q", got, want)
	}
}

func TestParseRouteExplicitTargets(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1:8080",
		"https://127.0.0.1:8443",
		"http://[::1]:8080",
	} {
		t.Run(target, func(t *testing.T) {
			route, err := ParseRoute("api.localhost=" + target)
			if err != nil {
				t.Fatalf("ParseRoute() error = %v", err)
			}
			if got := route.Target.String(); got != target {
				t.Errorf("Target = %q, want %q", got, target)
			}
		})
	}
}

func TestParseRouteNormalizesHost(t *testing.T) {
	route, err := ParseRoute(" APP.LOCALHOST. =3000")
	if err != nil {
		t.Fatalf("ParseRoute() error = %v", err)
	}
	if got, want := route.Host, "app.localhost"; got != want {
		t.Errorf("Host = %q, want %q", got, want)
	}
}

func TestParseRouteRejectsInvalidSpecifications(t *testing.T) {
	for _, spec := range []string{
		"app.localhost",
		"=3000",
		"bad_host.localhost=3000",
		"-app.localhost=3000",
		"app..localhost=3000",
		"app.localhost=",
		"app.localhost=0",
		"app.localhost=65536",
		"app.localhost=not-a-port",
		"app.localhost=ftp://localhost:21",
		"app.localhost=http://",
	} {
		t.Run(spec, func(t *testing.T) {
			if _, err := ParseRoute(spec); err == nil {
				t.Fatalf("ParseRoute(%q) error = nil, want error", spec)
			}
		})
	}
}

func TestParseRoutesRejectsNormalizedDuplicateHosts(t *testing.T) {
	_, err := ParseRoutes([]string{
		"app.localhost=3000",
		"APP.LOCALHOST.=4000",
	})
	if err == nil {
		t.Fatal("ParseRoutes() error = nil, want duplicate-host error")
	}
}
