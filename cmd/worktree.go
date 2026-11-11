package cmd

import (
	"fmt"

	"github.com/cmdlinezero/clz-lhd/internal/worktree"
	"github.com/spf13/cobra"
)

var worktreeCmd = &cobra.Command{Use: "worktree", Short: "Inspect Git worktree routing identity"}

func init() {
	worktreeCmd.AddCommand(&cobra.Command{
		Use: "info", Short: "Show the proposed hostname for the current Git worktree",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, err := worktree.DiscoverCurrent()
			if err != nil {
				return err
			}
			branch := info.Branch
			if info.Detached {
				branch = "(detached HEAD)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Repository: %s\nWorktree:   %s\nBranch:     %s\nMain:       %t\nHostname:   %s\n", info.Repository, info.Root, branch, info.Main, info.Host)
			return nil
		},
	})
	rootCmd.AddCommand(worktreeCmd)
}
