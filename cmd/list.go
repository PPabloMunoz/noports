/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"encoding/json"
	"os"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/registry"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all routes",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := client.EnsureProxy(); err != nil {
			return err
		}

		routes, err := client.ListRoutes()
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			if routes == nil {
				routes = []registry.Route{}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(routes)
		}

		client.PrintRoutesTable(routes)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().Bool("json", false, "Output routes as JSON")
}
