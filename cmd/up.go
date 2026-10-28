package cmd

import (
	"github.com/cmdlinezero/clz-lhd/internal/config"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the proxy using a configuration file",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		configPath, _ := cmd.Flags().GetString("config")

		routes, err := config.LoadFile(configPath)
		if err != nil {
			return err
		}

		return serve(port, routes)
	},
}

func init() {
	upCmd.Flags().StringP("config", "c", "lhd.yaml", "path to the configuration file")
	rootCmd.AddCommand(upCmd)
}
