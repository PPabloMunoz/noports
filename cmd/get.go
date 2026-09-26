/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/registry"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <name>",
	Args:  cobra.ExactArgs(1),
	Short: "Show details of a route",
	RunE: func(cmd *cobra.Command, args []string) error {
		hostname, err := registry.NormalizeHostname(args[0])
		if err != nil {
			return err
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

		req := &ipc.Request{Command: ipc.CmdGet, Hostname: hostname}
		if err := client.Send(encoder, req); err != nil {
			return err
		}

		var res ipc.Response
		if err := client.Receive(decoder, &res); err != nil {
			return err
		}

		var data ipc.DataResponseGet
		if err := client.DecodeGetResponse(&res.Data, &data); err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(data.Route)
		}

		fmt.Printf("https://%s\n", data.Route.Hostname)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().Bool("json", false, "Output route as JSON")
}
