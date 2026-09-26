/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/registry"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [flags] -- <command> [args...]",
	Short: "Run an app",
	Example: `  noports run --name web -- python3 -m http.server
  noports run --name web --port-arg -- astro dev`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		port, err := cmd.Flags().GetInt("port")
		if err != nil {
			return fmt.Errorf("invalid port")
		}
		portArg, err := cmd.Flags().GetBool("port-arg")
		if err != nil {
			return err
		}

		if name == "" {
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working dir: %w", err)
			}
			name = filepath.Base(wd)
		}

		hostname, err := registry.NormalizeHostname(name)
		if err != nil {
			return err
		}
		name = registry.BareName(hostname)

		if err := client.EnsureProxy(); err != nil {
			return err
		}

		if cmd.Flags().Changed("port") {
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid port %d: must be between 1 and 65535", port)
			}
		} else {
			port, err = client.FreeLoopbackPort()
			if err != nil {
				return err
			}
		}

		// Run command. Only some frameworks (e.g. astro dev) need a
		// --port CLI flag; the rest pick up PORT from the environment.
		childArgs := args[1:]
		if portArg {
			childArgs = append(append([]string{}, childArgs...), "--port", strconv.Itoa(port))
		}
		command := exec.Command(args[0], childArgs...)
		env := os.Environ()
		env = client.SetEnvVar(env, "PORT", strconv.Itoa(port))
		env = client.SetEnvVar(env, "NOPORTS_PORT", strconv.Itoa(port))
		env = client.SetEnvVar(env, "NOPORTS_NAME", name)
		env = client.SetEnvVar(env, "NOPORTS_URL", fmt.Sprintf("https://%s", hostname))
		command.Env = env
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.Stdin = os.Stdin

		if err := command.Start(); err != nil {
			return fmt.Errorf("failed to start command: %w", err)
		}

		// Connect to socket
		conn, err := ipc.Dial()
		if err != nil {
			_ = command.Process.Signal(os.Interrupt)
			return err
		}
		defer func() { _ = conn.Close() }()

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		wrapperPID := os.Getpid()
		// Record both start times immediately so the daemon's orphan sweep
		// can tell PID reuse apart from the original processes. A lookup
		// failure just stores 0 ("unknown"); the daemon backfills on receipt
		// and the sweep falls back to a liveness-only check.
		childStart, _ := registry.ProcessStartTime(command.Process.Pid)
		wrapperStart, _ := registry.ProcessStartTime(wrapperPID)
		req := &ipc.Request{Command: ipc.CmdAliasAdd, Hostname: hostname, LocalPort: port, PID: command.Process.Pid, WrapperPID: wrapperPID, ChildStartTime: childStart, WrapperStartTime: wrapperStart}
		if err := client.Send(encoder, req); err != nil {
			_ = command.Process.Signal(os.Interrupt)
			return err
		}

		var res ipc.Response
		if err := client.Receive(decoder, &res); err != nil {
			_ = command.Process.Signal(os.Interrupt)
			return err
		}

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- command.Wait()
		}()

		if waitDur, _ := cmd.Flags().GetDuration("wait"); waitDur > 0 {
			client.Info("Waiting up to %s for backend on port %d...\n", waitDur, port)
			tcpReady := make(chan error, 1)
			go func() { tcpReady <- client.WaitForTCP(port, waitDur) }()
			select {
			case childErr := <-waitCh:
				// Child exited before the backend became ready.
				if cerr := client.RemoveRunRoute(command, name, &res); cerr != nil {
					return fmt.Errorf("failed to clean up command: %w", cerr)
				}
				if childErr != nil {
					return fmt.Errorf("command exited before backend became ready: %w", childErr)
				}
				return fmt.Errorf("command exited before backend became ready")
			case tcpErr := <-tcpReady:
				if tcpErr != nil {
					_ = client.RemoveRunRoute(command, name, &res)
					_ = command.Process.Signal(os.Interrupt)
					return tcpErr
				}
			}
		}

		client.Success("Serving at https://%s\n", hostname)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		select {
		case err := <-waitCh:
			// Error in child command
			signal.Stop(sigCh)
			if err := client.RemoveRunRoute(command, name, &res); err != nil {
				return fmt.Errorf("failed to clean up command: %w", err)
			}
			if err != nil {
				return fmt.Errorf("command exit with error: %w", err)
			}
			return nil
		case <-sigCh:
			if err := client.RemoveRunRoute(command, name, &res); err != nil {
				return fmt.Errorf("failed to clean up command: %w", err)
			}

			if err := command.Process.Signal(os.Interrupt); err != nil {
				return fmt.Errorf("failed to signal child process: %w", err)
			}

			select {
			case <-waitCh:
				fmt.Println("child exited cleanly")
			case <-time.After(5 * time.Second):
				fmt.Println("timeout waiting for child, killing it")
				_ = command.Process.Kill()
				<-waitCh
			}

		}

		return nil
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
