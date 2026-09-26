package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/registry"
	"github.com/spf13/cobra"
)

// resolveRouteName reads the --name flag and returns the bare name and normalized hostname. An empty flag defaults to the current directory basename.
func resolveRouteName(cmd *cobra.Command) (bare, hostname string, err error) {
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return "", "", err
	}
	if name == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("failed to get working dir: %w", err)
		}
		name = filepath.Base(wd)
	}
	hostname, err = registry.NormalizeHostname(name)
	if err != nil {
		return "", "", err
	}
	return registry.BareName(hostname), hostname, nil
}

// resolvePort validates the --port flag or picks a free loopback port. Explicit ports must be in range 1-65535.
func resolvePort(cmd *cobra.Command, flagPort int) (int, error) {
	if cmd.Flags().Changed("port") {
		if flagPort < 1 || flagPort > 65535 {
			return 0, fmt.Errorf("invalid port %d: must be between 1 and 65535", flagPort)
		}
		return flagPort, nil
	}
	return client.FreeLoopbackPort()
}

// launchChild starts the backend command with PORT-style env vars set. Only some frameworks need the --port CLI flag; the rest read PORT from the environment.
func launchChild(bin string, args []string, name, hostname string, port int, portArg bool) (*exec.Cmd, error) {
	childArgs := args
	if portArg {
		childArgs = append(append([]string{}, args...), "--port", strconv.Itoa(port))
	}
	command := exec.Command(bin, childArgs...)
	env := os.Environ()
	env = setEnvVar(env, "PORT", strconv.Itoa(port))
	env = setEnvVar(env, "NOPORTS_PORT", strconv.Itoa(port))
	env = setEnvVar(env, "NOPORTS_NAME", name)
	env = setEnvVar(env, "NOPORTS_URL", fmt.Sprintf("https://%s", hostname))
	command.Env = env
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}
	return command, nil
}

// registerRoute registers the child backend with the daemon. It records both start times so the orphan sweep can tell PID reuse apart, and interrupts the child on failure.
func registerRoute(command *exec.Cmd, hostname string, port int) (net.Conn, *ipc.Response, error) {
	conn, err := ipc.Dial()
	if err != nil {
		_ = command.Process.Signal(os.Interrupt)
		return nil, nil, err
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	wrapperPID := os.Getpid()
	childStart, _ := registry.ProcessStartTime(command.Process.Pid)
	wrapperStart, _ := registry.ProcessStartTime(wrapperPID)
	req := &ipc.Request{Command: ipc.CmdAliasAdd, Hostname: hostname, LocalPort: port, PID: command.Process.Pid, WrapperPID: wrapperPID, ChildStartTime: childStart, WrapperStartTime: wrapperStart}
	if err := client.Send(encoder, req); err != nil {
		_ = command.Process.Signal(os.Interrupt)
		_ = conn.Close()
		return nil, nil, err
	}

	var res ipc.Response
	if err := client.Receive(decoder, &res); err != nil {
		_ = command.Process.Signal(os.Interrupt)
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, &res, nil
}

// waitForBackend waits up to waitDur for the backend to accept TCP. A non-positive duration skips waiting; a child exit or timeout interrupts the child and unregisters the route.
func waitForBackend(command *exec.Cmd, name string, port int, waitDur time.Duration, waitCh chan error, res *ipc.Response) error {
	if waitDur <= 0 {
		return nil
	}
	client.Info("Waiting up to %s for backend on port %d...\n", waitDur, port)
	tcpReady := make(chan error, 1)
	go func() { tcpReady <- client.WaitForTCP(port, waitDur) }()
	select {
	case childErr := <-waitCh:
		// Child exited before the backend became ready.
		if cerr := client.RemoveRunRoute(command, name, res); cerr != nil {
			return fmt.Errorf("failed to clean up command: %w", cerr)
		}
		if childErr != nil {
			return fmt.Errorf("command exited before backend became ready: %w", childErr)
		}
		return fmt.Errorf("command exited before backend became ready")
	case tcpErr := <-tcpReady:
		if tcpErr != nil {
			_ = client.RemoveRunRoute(command, name, res)
			_ = command.Process.Signal(os.Interrupt)
			return tcpErr
		}
	}
	return nil
}

// superviseChild reports readiness and waits for the child or an interrupt. It unregisters the route on exit and gives the child 5s to stop gracefully before killing it.
func superviseChild(command *exec.Cmd, name, hostname string, waitCh chan error, res *ipc.Response) error {
	client.Success("Serving at https://%s\n", hostname)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-waitCh:
		// Error in child command
		signal.Stop(sigCh)
		if err := client.RemoveRunRoute(command, name, res); err != nil {
			return fmt.Errorf("failed to clean up command: %w", err)
		}
		if err != nil {
			return fmt.Errorf("command exit with error: %w", err)
		}
		return nil
	case <-sigCh:
		if err := client.RemoveRunRoute(command, name, res); err != nil {
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
}

// setEnvVar returns env with key set to value, replacing any existing entry instead of appending a duplicate.
func setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
