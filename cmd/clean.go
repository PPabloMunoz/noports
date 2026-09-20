/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
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
		if err := client.StopProxy(); err != nil {
			return err
		}

		var answer string
		client.Info("This will remove all the routes registered and remove and uninstall all certs created.\n")
		client.Warning("Do you want to continue [y/N]: ")
		_, _ = fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			client.Info("Canceling cleaning\n")
			return nil
		}

		if err := pki.UninstallCA(); err != nil {
			return err
		}
		client.Info("CA uninstalled\n")

		certsDir, err := paths.GetCertsDirPath()
		if err != nil {
			return fmt.Errorf("failed to get certificates dir: %w", err)
		}
		if err := os.RemoveAll(certsDir); err != nil {
			return fmt.Errorf("failed to delete certificates dir: %w", err)
		}
		client.Info("leaf certificates removed\n")

		routesPath, err := paths.GetRoutesFilePath()
		if err != nil {
			return fmt.Errorf("failed to get routes file path: %w", err)
		}
		if err := os.Remove(routesPath); err != nil {
			return fmt.Errorf("failed to delete %s: %w", routesPath, err)
		}
		client.Info("routes deleted\n")

		client.Success("clean completed\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
