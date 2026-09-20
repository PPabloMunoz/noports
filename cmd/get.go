/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/registry"
	"github.com/spf13/cobra"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get <name>",
	Args:  cobra.ExactArgs(1),
	Short: "Get the data the route with name",
	RunE: func(cmd *cobra.Command, args []string) error {
		hostname, err := registry.NormalizeHostname(args[0])
		if err != nil {
			return err
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

		req := &ipc.Request{Command: ipc.CmdGet, Hostname: hostname}
		if err := client.SendRequest(encoder, req); err != nil {
			return err
		}

		var res ipc.Response
		if err := client.GetResponse(decoder, &res); err != nil {
			return err
		}

		var data ipc.DataResponseGet
		if err := client.GetDataGet(&res.Data, &data); err != nil {
			return err
		}

		fmt.Printf("https://%s\n", data.Route.Hostname)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	// getCmd.PersistentFlags().String("foo", "", "A help for foo")
}
