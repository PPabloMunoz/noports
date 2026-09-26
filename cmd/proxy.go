/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"fmt"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/spf13/cobra"
)

// proxyCmd represents the proxy command
var proxyCmd = &cobra.Command{
	Use:       "proxy <start|stop>",
	Short:     "Start and stop proxy daemon",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"start", "stop"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "start":
			if err := client.EnsureProxy(); err != nil {
				return err
			}
			client.Info("Proxy started\n")
		case "stop":
			if err := client.StopDaemon(); err != nil {
				return err
			}
			client.Info("Proxy stopped\n")
		default:
			return fmt.Errorf("arg is not valid")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(proxyCmd)
}
