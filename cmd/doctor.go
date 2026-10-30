package cmd

import (
	"fmt"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check local proxy configuration and connectivity",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		configPath, _ := cmd.Flags().GetString("config")
		tlsPort, _ := cmd.Flags().GetString("tls-port")
		verbose, _ := cmd.Flags().GetBool("verbose")

		report, err := doctor.Run(configPath, port, tlsPort, time.Second)
		if err != nil {
			return err
		}

		fmt.Fprint(cmd.OutOrStdout(), doctorSummary(report, verbose))
		if problems := report.Problems(); problems > 0 {
			return fmt.Errorf("%d %s found", problems, pluralize("problem", problems))
		}
		fmt.Fprintln(cmd.OutOrStdout(), "0 problems found")
		return nil
	},
}

func doctorSummary(report doctor.Report, verbose bool) string {
	summary := "CLZ LHD diagnostics\n\n"
	for _, check := range report.Checks {
		mark := "✓"
		if !check.OK {
			mark = "✗"
		}
		summary += fmt.Sprintf("%s %s\n", mark, check.Label)
		detail := check.Detail
		if verbose && check.VerboseDetail != "" {
			detail = check.VerboseDetail
		}
		if detail != "" {
			summary += fmt.Sprintf("  %s\n", detail)
		}
	}

	return summary
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

func init() {
	doctorCmd.Flags().StringP("config", "c", "lhd.yaml", "path to the configuration file")
	doctorCmd.Flags().String("tls-port", "8443", "HTTPS port when TLS is configured")
	rootCmd.AddCommand(doctorCmd)
}
