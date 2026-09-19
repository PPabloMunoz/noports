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
		fmt.Println("This will remove all the routes registered and remove and uninstall all certs created.")
		fmt.Print("Do you want to continue [y/N]:")
		_, _ = fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Canceling cleaning")
			return nil
		}

		if err := pki.UninstallCA(); err != nil {
			return err
		}
		fmt.Println("CA uninstalled")

		certsDir, err := paths.GetCertsDirPath()
		if err != nil {
			return fmt.Errorf("failed to get certificates dir: %w", err)
		}
		if err := os.RemoveAll(certsDir); err != nil {
			return fmt.Errorf("failed to delete certificates dir: %w", err)
		}
		fmt.Println("All certificates removed")

		routesPath, err := paths.GetRoutesFilePath()
		if err != nil {
			return fmt.Errorf("failed to get routes file path: %w", err)
		}
		if err := os.Remove(routesPath); err != nil {
			return fmt.Errorf("failed to delete %s: %w", routesPath, err)
		}
		fmt.Println("Routes deleted")

		fmt.Println("CLEAN COMPLETED")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
