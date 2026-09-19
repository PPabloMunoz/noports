/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/spf13/cobra"
)

// aliasCmd represents the alias command
var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "A brief description of your command",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
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

		conn, err := client.ConnectToSocket()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if cmd.Flags().Changed("remove") {
			req := &ipc.Request{Command: ipc.CmdAliasRemove, Hostname: name}
			if err := client.SendRequest(encoder, req); err != nil {
				return err
			}

			var res ipc.Response
			if err := client.GetResponse(decoder, &res); err != nil {
				return err
			}

			fmt.Println("Alias removed")
			return nil
		}

		if port <= 0 {
			return fmt.Errorf("invalid port")
		}

		req := &ipc.Request{Command: ipc.CmdAliasAdd, Hostname: name, LocalPort: port, PID: -1}
		if err := client.SendRequest(encoder, req); err != nil {
			return err
		}

		var res ipc.Response
		if err := client.GetResponse(decoder, &res); err != nil {
			return err
		}

		fmt.Println("Alias added")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(aliasCmd)
	aliasCmd.PersistentFlags().BoolP("remove", "r", false, "Remove a route")
}
