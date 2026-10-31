package cmd

import (
	"fmt"

	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/cmdlinezero/clz-lhd/internal/tlsconfig"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the proxy using a configuration file",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		statusPort, _ := cmd.Flags().GetString("status-port")
		tlsPort, _ := cmd.Flags().GetString("tls-port")
		configPath, _ := cmd.Flags().GetString("config")

		project, err := config.LoadProject(configPath)
		if err != nil {
			return err
		}
		if project.TLS == nil {
			return serve(port, statusPort, project.Routes)
		}

		certificate, err := tlsconfig.Load(project.TLS.Cert, project.TLS.Key)
		if err != nil {
			return fmt.Errorf("load TLS certificate: %w", err)
		}
		return serveTLS(port, statusPort, project.Routes, tlsListenerConfig{
			Port:        tlsPort,
			Certificate: certificate.TLS,
		})
	},
}

func init() {
	upCmd.Flags().StringP("config", "c", "lhd.yaml", "path to the configuration file")
	upCmd.Flags().String("tls-port", "8443", "HTTPS port when TLS is configured")
	rootCmd.AddCommand(upCmd)
}
