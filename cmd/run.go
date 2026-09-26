/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"fmt"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [flags] -- <command> [args...]",
	Short: "Run an app",
	Example: `  noports run --name web -- python3 -m http.server
  noports run --name web --port-arg -- astro dev`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, hostname, err := resolveRouteName(cmd)
		if err != nil {
			return err
		}
		portArg, err := cmd.Flags().GetBool("port-arg")
		if err != nil {
			return err
		}
		flagPort, err := cmd.Flags().GetInt("port")
		if err != nil {
			return fmt.Errorf("invalid port")
		}
		waitDur, _ := cmd.Flags().GetDuration("wait")

		if err := client.EnsureProxy(); err != nil {
			return err
		}

		port, err := resolvePort(cmd, flagPort)
		if err != nil {
			return err
		}

		command, err := launchChild(args[0], args[1:], name, hostname, port, portArg)
		if err != nil {
			return err
		}

		conn, res, err := registerRoute(command, hostname, port)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- command.Wait()
		}()

		if err := waitForBackend(command, name, port, waitDur, waitCh, res); err != nil {
			return err
		}

		return superviseChild(command, name, hostname, waitCh, res)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	// Stop flag parsing at the first positional arg so child args that
	// look like flags (e.g. `astro dev --port 4321`) are passed through
	// verbatim. noports flags must therefore come before the command.
	// `--` is still accepted as an explicit separator but is optional.
	runCmd.Flags().SetInterspersed(false)
	runCmd.Flags().String("name", "", "Name the route")
	runCmd.Flags().Int("port", 0, "Port to run the app (default: random free port)")
	runCmd.Flags().Bool("port-arg", false, "Append --port <port> to the child command (needed by some frameworks, e.g. astro)")
	runCmd.Flags().Duration("wait", 0, "Wait up to this long for the backend to accept TCP before serving (e.g. --wait 10s)")
}
