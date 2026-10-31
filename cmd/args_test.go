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

func TestStatusPortIsGlobalFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("status-port")
	if flag == nil {
		t.Fatal("status-port persistent flag not registered")
	}
	if flag.DefValue != "9099" {
		t.Fatalf("status-port default = %q, want %q", flag.DefValue, "9099")
	}
}

func TestVerboseIsGlobalFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("verbose")
	if flag == nil {
		t.Fatal("verbose persistent flag not registered")
	}
	if flag.Shorthand != "v" {
		t.Fatalf("verbose shorthand = %q, want %q", flag.Shorthand, "v")
	}
}
