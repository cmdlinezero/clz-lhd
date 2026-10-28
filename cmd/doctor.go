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

		report, err := doctor.Run(configPath, port, time.Second)
		if err != nil {
			return err
		}

		fmt.Fprint(cmd.OutOrStdout(), doctorSummary(report))
		if problems := report.Problems(); problems > 0 {
			return fmt.Errorf("%d %s found", problems, pluralize("problem", problems))
		}
		fmt.Fprintln(cmd.OutOrStdout(), "0 problems found")
		return nil
	},
}

func doctorSummary(report doctor.Report) string {
	summary := "CLZ LHD diagnostics\n\n"
	for _, check := range report.Checks {
		mark := "✓"
		if !check.OK {
			mark = "✗"
		}
		summary += fmt.Sprintf("%s %s\n", mark, check.Label)
		if check.Detail != "" {
			summary += fmt.Sprintf("  %s\n", check.Detail)
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
	rootCmd.AddCommand(doctorCmd)
}
