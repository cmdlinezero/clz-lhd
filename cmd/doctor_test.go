package cmd

import (
	"strings"
	"testing"

	"github.com/cmdlinezero/clz-lhd/internal/doctor"
)

func TestDoctorSummary(t *testing.T) {
	report := doctor.Report{Checks: []doctor.Check{
		{OK: true, Label: "lhd.yaml loaded"},
		{OK: true, Label: "2 routes configured"},
		{Label: "api.localhost -> localhost:4000 reachable", Detail: "connection refused"},
	}}

	got := doctorSummary(report, false)
	for _, want := range []string{
		"CLZ LHD diagnostics",
		"✓ lhd.yaml loaded",
		"✓ 2 routes configured",
		"✗ api.localhost -> localhost:4000 reachable",
		"  connection refused",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctorSummary() missing %q:\n%s", want, got)
		}
	}
}

func TestDoctorSummaryUsesVerboseDetail(t *testing.T) {
	report := doctor.Report{Checks: []doctor.Check{{
		OK:            true,
		Label:         "TLS certificate details",
		Detail:        "issuer: LHD test CA; expires: 2027-01-01; DNS names: app.localhost",
		VerboseDetail: "subject: CN=app.localhost; issuer: CN=LHD test CA; serial: 42",
	}}}

	concise := doctorSummary(report, false)
	if !strings.Contains(concise, "issuer: LHD test CA") || strings.Contains(concise, "serial: 42") {
		t.Fatalf("concise doctorSummary() = %q", concise)
	}

	verbose := doctorSummary(report, true)
	if !strings.Contains(verbose, "serial: 42") || strings.Contains(verbose, "expires: 2027-01-01") {
		t.Fatalf("verbose doctorSummary() = %q", verbose)
	}
}
