/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
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
		isRunning, err := client.IsDaemonRunning()
		if err != nil {
			return err
		}

		switch args[0] {
		case "start":
			if isRunning {
				client.Info("Already running\n")
				return nil
			}
			if err := client.EnsureProxy(); err != nil {
				return err
			}
		case "stop":
			if !isRunning {
				client.Info("Already stopped\n")
				return nil
			}
			if err := client.StopProxy(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("arg is not valid")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(proxyCmd)
}
