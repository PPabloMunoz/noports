/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/ppablomunoz/noports/internal/pki"
	"github.com/spf13/cobra"
)

// trustCmd represents the trust command
var trustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Adds the noports certificate authority to your system trust store. Required once for HTTPS with auto-generated certs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := pki.EnsureCA(); err != nil {
			return err
		}
		fmt.Println("CA Installed")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(trustCmd)
}
