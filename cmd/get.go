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

		res, err := client.RoundTrip(&ipc.Request{Command: ipc.CmdGet, Hostname: hostname})
		if err != nil {
			return err
		}

		var data ipc.DataResponseGet
		if err := client.DecodeData(&res.Data, &data); err != nil {
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
