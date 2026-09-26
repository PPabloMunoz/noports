/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/pki"
	"github.com/spf13/cobra"
)

var trustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Adds the noports certificate authority to your system trust store. Required once for HTTPS with auto-generated certs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := pki.EnsureCA(); err != nil {
			return err
		}
		client.Info("CA Installed\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(trustCmd)
}
