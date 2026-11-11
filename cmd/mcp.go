package cmd

import (
 "context"
 "os"
 "github.com/cmdlinezero/clz-lhd/internal/mcp"
 "github.com/spf13/cobra"
)

func init() {
 rootCmd.AddCommand(&cobra.Command{
  Use:"mcp", Short:"Run a read-only MCP server over stdio",Args:cobra.NoArgs,
  RunE:func(cmd *cobra.Command,_ []string)error {
   port,err:=cmd.Flags().GetString("status-port");if err!=nil{return err}
   return (mcp.Server{Port:port}).Serve(context.Background(),os.Stdin,os.Stdout)
  },
 })
}
