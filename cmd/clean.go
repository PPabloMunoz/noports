/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/paths"
	"github.com/ppablomunoz/noports/internal/pki"
	"github.com/spf13/cobra"
)

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean files and certificates from the device",
	RunE: func(cmd *cobra.Command, args []string) error {
		var answer string
		client.Info("This will remove all the routes registered and remove and uninstall all certs created.\n")
		client.Warning("Do you want to continue [y/N]: ")
		_, _ = fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			client.Info("Canceling cleaning\n")
			return nil
		}

		if err := client.StopProxy(); err != nil {
			return err
		}
		client.Info("proxy stopped\n")

		if err := pki.UninstallCA(); err != nil {
			return err
		}
		client.Info("CA uninstalled\n")

		certsDir, err := paths.GetCertsDirPath()
		if err != nil {
			return fmt.Errorf("failed to get certificates dir path: %w", err)
		}
		if err := os.RemoveAll(certsDir); err != nil {
			return fmt.Errorf("failed to remove %s: %w", certsDir, err)
		}
		client.Info("certificates removed\n")

		routesPath, err := paths.GetRoutesFilePath()
		if err != nil {
			return fmt.Errorf("failed to get routes file path: %w", err)
		}
		if err := os.Remove(routesPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove %s: %w", routesPath, err)
		}
		client.Info("routes deleted\n")

		// Delete log file
		logFilePath, err := paths.GetLogFilePath()
		if err != nil {
			return fmt.Errorf("failed to get daemon log file path: %w", err)
		}
		if err := os.Remove(logFilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove %s: %w", logFilePath, err)
		}

		// Delete .pid
		pidPath, err := paths.GetPIDFilePath()
		if err != nil {
			return fmt.Errorf("failed to get pid file path: %w", err)
		}
		if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove %s: %w", pidPath, err)
		}

		// Delete socket file
		socketPath := paths.GetSocketPath()
		if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove %s: %w", socketPath, err)
		}

		client.Success("clean completed\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
