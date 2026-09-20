/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
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
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run an app",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameFlag := cmd.Flag("name")
		name := nameFlag.Value.String()
		portFlag := cmd.Flag("port")
		port, err := strconv.Atoi(portFlag.Value.String())
		if err != nil {
			return fmt.Errorf("invalid port")
		}

		if name == "" {
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working dir: %w", err)
			}
			name = filepath.Base(wd)
		}

		if err := client.EnsureProxy(); err != nil {
			return err
		}

		if !portFlag.Changed {
			port, err = client.GetRandomPort()
			if err != nil {
				return err
			}
		}

		// Run command
		args = append(args, "--port", strconv.Itoa(port))
		command := exec.Command(args[0], args[1:]...)
		command.Env = os.Environ()
		command.Env = append(command.Env, fmt.Sprintf("PORT=%d", port))
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.Stdin = os.Stdin

		if err := command.Start(); err != nil {
			return fmt.Errorf("failed to start command: %w", err)
		}

		// Connect to socket
		conn, err := client.ConnectToSocket()
		if err != nil {
			_ = command.Process.Signal(os.Interrupt)
			return err
		}
		defer func() { _ = conn.Close() }()

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		req := &ipc.Request{Command: ipc.CmdAliasAdd, Hostname: name, LocalPort: port, PID: command.Process.Pid}
		if err := client.SendRequest(encoder, req); err != nil {
			_ = command.Process.Signal(os.Interrupt)
			return err
		}

		var res ipc.Response
		if err := client.GetResponse(decoder, &res); err != nil {
			_ = command.Process.Signal(os.Interrupt)
			return err
		}

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- command.Wait()
		}()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		select {
		case err := <-waitCh:
			// Error in child command
			signal.Stop(sigCh)
			if err := client.CleanUpCommand(command, name, &res); err != nil {
				return fmt.Errorf("failed to clean up command: %w", err)
			}
			if err != nil {
				return fmt.Errorf("command exit with error: %w", err)
			}
			return nil
		case <-sigCh:
			if err := client.CleanUpCommand(command, name, &res); err != nil {
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
	runCmd.PersistentFlags().String("name", "", "Name the route")
	runCmd.PersistentFlags().Int("port", -1, "Port to run the app")
}
