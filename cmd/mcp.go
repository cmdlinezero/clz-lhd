package cmd

import (
	"context"
	"fmt"
	"github.com/cmdlinezero/clz-lhd/internal/mcp"
	"github.com/spf13/cobra"
	"os"
)

func init() {
	mcpCmd := &cobra.Command{
		Use: "mcp", Short: "Run an MCP server over stdio (read-only by default)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			port, err := cmd.Flags().GetString("status-port")
			if err != nil {
				return err
			}
			enabled, err := cmd.Flags().GetBool("enable-agent-writes")
			if err != nil {
				return err
			}
			token := ""
			if enabled {
				token = os.Getenv("LHD_AGENT_TOKEN")
				if token == "" {
					return fmt.Errorf("--enable-agent-writes requires LHD_AGENT_TOKEN")
				}
				if token == os.Getenv("LHD_MANAGEMENT_TOKEN") {
					return fmt.Errorf("agent and management tokens must differ")
				}
			}
			return (mcp.Server{Port: port, AgentToken: token}).Serve(context.Background(), os.Stdin, os.Stdout)
		},
	}
	mcpCmd.Flags().Bool("enable-agent-writes", false, "Expose restricted agent route mutation tools (requires LHD_AGENT_TOKEN)")
	rootCmd.AddCommand(mcpCmd)
}
