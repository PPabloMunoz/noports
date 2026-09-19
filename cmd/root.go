/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "noports",
	Short: "A lightweight reverse proxy CLI that maps local development ports to clean, persistent .localhost domains with automatic HTTPS.",
	Long: `noports is a lightweight, zero-configuration local reverse proxy built in Go that simplifies local multi-service development.

Instead of managing conflicting port allocations (e.g., :3000, :8080, :5173), it assigns deterministic, named .localhost subdomains to running
processes (e.g., [https://api.localhost](https://api.localhost), [https://web.localhost](https://web.localhost)). Running entirely as a self-contained binary,
it combines an internal HTTP/WebSocket reverse proxy with automated local certificate management via pure Go tooling. It intercepts loopback traffic,
routes incoming requests dynamically to their assigned backend ports, and provides instant, valid TLS termination out of the box—eliminating port collisions,
CORS quirks, and credential sync issues across concurrent projects.
	`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := ipc.Dial()
		if err != nil {
			return fmt.Errorf("failed to connect to socket: %v", err)
		}
		defer func() { _ = conn.Close() }()

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		req := ipc.Request{
			Command:   ipc.CmdAliasAdd,
			Hostname:  "app.localhost",
			LocalPort: 4321,
			PID:       -1,
		}
		if err := encoder.Encode(req); err != nil {
			return fmt.Errorf("failed to encode request: %w", err)
		}

		// wait for response
		var res ipc.Response
		if err := decoder.Decode(&res); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		fmt.Printf("OK: %v -- Error: '%s' -- Data: '%v'\n", res.OK, res.Error, res.Data)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Flags
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
