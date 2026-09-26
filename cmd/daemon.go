/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"context"
	"html/template"

	"github.com/ppablomunoz/noports/internal/app"
	"github.com/spf13/cobra"
)

// dashboardTemplate renders the "localhost" host. Set by main via SetDashboardTemplate
// (main owns the //go:embed of web/, since embed patterns cannot escape cmd/).
var dashboardTemplate *template.Template

// SetDashboardTemplate installs the landing-page template (embedded web/index.html.tmpl in main).
func SetDashboardTemplate(t *template.Template) {
	dashboardTemplate = t
}

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the noports daemon (HTTPS proxy, redirect and control socket)",
	Long: `Starts the background daemon: ensures directories and local CA,
loads persisted routes, serves the HTTP->HTTPS redirect, the HTTPS
reverse proxy, and the unix control socket.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return app.Run(context.Background(), app.Config{DashboardTemplate: dashboardTemplate})
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
