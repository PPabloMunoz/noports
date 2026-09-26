/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"fmt"
	"strconv"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/registry"
	"github.com/spf13/cobra"
)

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

		if cmd.Flags().Changed("remove") {
			if _, err := client.RoundTrip(&ipc.Request{Command: ipc.CmdAliasRemove, Hostname: hostname}); err != nil {
				return err
			}

			client.Info("Alias removed\n")
			return nil
		}

		if port <= 0 {
			return fmt.Errorf("invalid port")
		}

		if _, err := client.RoundTrip(&ipc.Request{Command: ipc.CmdAliasAdd, Hostname: hostname, LocalPort: port, PID: -1, WrapperPID: -1}); err != nil {
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
