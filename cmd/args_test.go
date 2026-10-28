package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandsRejectPositionalArguments(t *testing.T) {
	for _, command := range []*cobra.Command{upCmd, routeCmd, routesCmd, doctorCmd} {
		t.Run(command.Name(), func(t *testing.T) {
			if err := command.Args(command, []string{"unexpected"}); err == nil {
				t.Fatal("Args() error = nil, want positional-argument error")
			}
		})
	}
}
