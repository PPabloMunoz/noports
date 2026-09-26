/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/registry"
	"github.com/spf13/cobra"
)

// aliasCmd represents the alias command
var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Point a .localhost hostname at a local port",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		hostname, err := registry.NormalizeHostname(args[0])
		if err != nil {
			return err
		}
		port := -1

		if len(args) == 2 {
			p, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("port is not a valid int: %w", err)
			}
			port = p
		}

		if err := client.EnsureProxy(); err != nil {
			return err
		}

		conn, err := ipc.Dial()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if cmd.Flags().Changed("remove") {
			req := &ipc.Request{Command: ipc.CmdAliasRemove, Hostname: hostname}
			if err := client.Send(encoder, req); err != nil {
				return err
			}

			var res ipc.Response
			if err := client.Receive(decoder, &res); err != nil {
				return err
			}

			client.Info("Alias removed\n")
			return nil
		}

		if port <= 0 {
			return fmt.Errorf("invalid port")
		}

		req := &ipc.Request{Command: ipc.CmdAliasAdd, Hostname: hostname, LocalPort: port, PID: -1, WrapperPID: -1}
		if err := client.Send(encoder, req); err != nil {
			return err
		}

		var res ipc.Response
		if err := client.Receive(decoder, &res); err != nil {
			return err
		}

		client.Success("Alias added: https://%s\n", hostname)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(aliasCmd)
	aliasCmd.PersistentFlags().BoolP("remove", "r", false, "Remove a route")
}
