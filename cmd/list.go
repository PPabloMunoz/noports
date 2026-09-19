/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/ipc"
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

		conn, err := client.ConnectToSocket()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		req := &ipc.Request{Command: ipc.CmdList}
		if err := client.SendRequest(encoder, req); err != nil {
			return err
		}

		var res ipc.Response
		if err := client.GetResponse(decoder, &res); err != nil {
			return err
		}

		var data ipc.DataResponseList
		if err := client.GetDataList(&res.Data, &data); err != nil {
			return err
		}

		client.PrintRoutesTable(data.Routes)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
