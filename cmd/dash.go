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
	var requests []dashboard.Request
	var requestsErr error
	requestSelection := dashboard.RequestSelection{}
	showRequests := false

	ticker := time.NewTicker(dashboardRefresh)
	defer ticker.Stop()

	redraw := func() {
		width, height := screen.Size()
		if showRequests {
			err := fetchErr
			if err == nil {
				err = requestsErr
			}
			drawDashboard(screen, dashboard.RequestLines(snapshot, requests, err, requestSelection.ID, width, height, time.Now()))
		} else {
			drawDashboard(screen, dashboard.Lines(snapshot, fetchErr, selection.Host, width, height, time.Now()))
		}
	}
	refresh := func() {
		fetchCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
		snapshot, fetchErr = client.Snapshot(fetchCtx)
		if fetchErr == nil {
			selection.Normalize(snapshot.Routes)
			var result statushttp.RequestsResponse
			result, requestsErr = client.Requests(fetchCtx)
			if requestsErr == nil {
				requests = dashboard.GroupRequests(result.Events)
				requestSelection.Normalize(requests)
			}
		}
		cancel()
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
				if event.Key() == tcell.KeyTab {
					showRequests = !showRequests
					redraw()
					continue
				}
				switch event.Str() {
				case "q":
					return nil
				case "j":
					if showRequests {
						requestSelection.Move(requests, 1)
					} else {
						selection.Move(snapshot.Routes, 1)
					}
					redraw()
				case "k":
					if showRequests {
						requestSelection.Move(requests, -1)
					} else {
						selection.Move(snapshot.Routes, -1)
					}
					redraw()
				case "g":
					if showRequests {
						requestSelection.First(requests)
					} else {
						selection.First(snapshot.Routes)
					}
					redraw()
				case "G":
					if showRequests {
						requestSelection.Last(requests)
					} else {
						selection.Last(snapshot.Routes)
					}
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
