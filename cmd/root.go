package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "clz-lhd",
	Short: "A local host development (LHD) port proxy with host-based routing and telemetry",
	Long: `clz-lhd is a CLI tool designed to simplify local host development (LHD) by 
mapping custom local domains (like api.localhost) to specific internal ports, 
complete with built-in Prometheus metrics and an admin dashboard.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Persistent flags are available to the root command and every subcommand
	rootCmd.PersistentFlags().StringP("port", "p", "80", "The port the proxy listens on")
}
