/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/paths"
	"github.com/ppablomunoz/noports/internal/registry"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		running, err := client.IsDaemonRunning()
		if err != nil {
			return err
		}
		if !running {
			client.Info("Daemon not running\n")
			return nil
		}

		pidInfo := "unknown"
		if pidFilePath, err := paths.PIDFile(); err == nil {
			if b, err := os.ReadFile(pidFilePath); err == nil {
				if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
					state := "dead"
					if registry.ProcessAlive(pid) {
						state = "alive"
					}
					pidInfo = fmt.Sprintf("%d (%s)", pid, state)
				}
			}
		}

		routes := "unknown"
		if list, err := client.ListRoutes(); err == nil {
			routes = strconv.Itoa(len(list))
		}

		logPath, _ := paths.LogFile()
		client.Success("Daemon running (pid %s)\n", pidInfo)
		client.Info("Socket: %s\n", paths.Socket())
		client.Info("Routes: %s\n", routes)
		client.Info("Log: %s\n", logPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
