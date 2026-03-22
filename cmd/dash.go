package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/spf13/cobra"
)

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Start proxy with Web UI dashboard",
	Run: func(cmd *cobra.Command, args []string) {
		routes, _ := cmd.Flags().GetStringToString("route")
		
		// 1. Run Proxy in background
		go func() {
			fmt.Println("Proxy engine starting...")
			// (Re-use logic from route.go or move to internal/proxy)
		}()

		// 2. Admin API
		http.HandleFunc("/api/routes", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(routes)
		})

		fmt.Println("Admin Dashboard: http://localhost:4000")
		http.ListenAndServe(":4000", nil)
	},
}

func init() {
	dashCmd.Flags().StringToStringP("route", "r", nil, "host=port mappings")
	rootCmd.AddCommand(dashCmd)
}
