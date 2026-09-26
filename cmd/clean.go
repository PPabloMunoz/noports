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

		if err := client.StopDaemon(); err != nil {
			return err
		}
		client.Info("proxy stopped\n")

		if err := pki.UninstallCA(); err != nil {
			return err
		}
		client.Info("CA uninstalled\n")

		certsDir, err := paths.CertsDir()
		if err != nil {
			return fmt.Errorf("failed to get certificates dir path: %w", err)
		}
		if err := removeAllPath(certsDir); err != nil {
			return err
		}
		client.Info("certificates removed\n")

		routesPath, err := paths.RoutesFile()
		if err != nil {
			return fmt.Errorf("failed to get routes file path: %w", err)
		}
		if err := removePath(routesPath); err != nil {
			return err
		}
		client.Info("routes deleted\n")

		logFilePath, err := paths.LogFile()
		if err != nil {
			return fmt.Errorf("failed to get daemon log file path: %w", err)
		}
		if err := removePath(logFilePath); err != nil {
			return err
		}

		pidPath, err := paths.PIDFile()
		if err != nil {
			return fmt.Errorf("failed to get pid file path: %w", err)
		}
		if err := removePath(pidPath); err != nil {
			return err
		}

		if err := removePath(paths.Socket()); err != nil {
			return err
		}

		client.Success("clean completed\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}

// removePath removes a single file, ignoring missing files. It wraps failures with the path for CLI reporting.
func removePath(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}
	return nil
}

// removeAllPath removes a directory tree, succeeding when the path is already missing. It wraps failures with the path for CLI reporting.
func removeAllPath(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}
	return nil
}
