package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdlinezero/clz-lhd/internal/dashboard"
	"github.com/cmdlinezero/clz-lhd/internal/statusclient"
	"github.com/cmdlinezero/clz-lhd/internal/statushttp"
	"github.com/gdamore/tcell/v3"
	"github.com/spf13/cobra"
)

const dashboardRefresh = time.Second

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Show the live local LHD dashboard",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		statusPort, err := cmd.Flags().GetString("status-port")
		if err != nil {
			return err
		}
		return runDashboard(cmd.Context(), statusclient.NewClient(statusPort))
	},
}

func init() {
	rootCmd.AddCommand(dashCmd)
}

func runDashboard(ctx context.Context, client *statusclient.Client) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("create terminal screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("initialize terminal screen: %w", err)
	}
	defer screen.Fini()
	screen.HideCursor()

	selection := dashboard.Selection{}
	var snapshot statushttp.Response
	var fetchErr error

	ticker := time.NewTicker(dashboardRefresh)
	defer ticker.Stop()

	redraw := func() {
		width, height := screen.Size()
		drawDashboard(screen, dashboard.Lines(snapshot, fetchErr, selection.Host, width, height, time.Now()))
	}
	refresh := func() {
		fetchCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
		snapshot, fetchErr = client.Snapshot(fetchCtx)
		cancel()
		if fetchErr == nil {
			selection.Normalize(snapshot.Routes)
		}
		redraw()
	}
	refresh()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			refresh()
		case event, ok := <-screen.EventQ():
			if !ok {
				return nil
			}
			switch event := event.(type) {
			case *tcell.EventResize:
				screen.Sync()
				redraw()
			case *tcell.EventKey:
				switch event.Str() {
				case "q":
					return nil
				case "j":
					selection.Move(snapshot.Routes, 1)
					redraw()
				case "k":
					selection.Move(snapshot.Routes, -1)
					redraw()
				case "g":
					selection.First(snapshot.Routes)
					redraw()
				case "G":
					selection.Last(snapshot.Routes)
					redraw()
				}
				if event.Key() == tcell.KeyCtrlC {
					return nil
				}
			}
		}
	}
}

func drawDashboard(screen tcell.Screen, lines []string) {
	screen.Clear()
	for y, line := range lines {
		screen.PutStr(0, y, line)
	}
	screen.Show()
}
