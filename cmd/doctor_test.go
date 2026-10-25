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

	got := doctorSummary(report)
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
