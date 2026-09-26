package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/paths"
	"github.com/ppablomunoz/noports/internal/registry"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
)

const (
	// stopGracePeriod must exceed the daemon's own shutdown budget
	// (10s in app.Run) so a slow but healthy shutdown still counts as stopped.
	stopGracePeriod = 30 * time.Second
	stopPollEvery   = 100 * time.Millisecond
)

func PrintRoutesTable(routes []registry.Route) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	_, _ = fmt.Fprintf(w, "NAME\tHOST\tPORT\tPID\n")
	for _, r := range routes {
		name := registry.BareName(r.Hostname)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", name, r.Hostname, r.Port, r.PID)
	}

	_ = w.Flush()
}

// IsDaemonRunning checks if a daemon process is running by checking
// if can connect to the socket
func IsDaemonRunning() (bool, error) {
	socketPath := paths.GetSocketPath()

	conn, err := net.Dial("unix", socketPath)
	if err == nil {
		_ = conn.Close()
		return true, nil
	}
	return false, nil
}

func StartProxy() error {
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	defer func() { _ = r.Close() }()

	cmd := exec.Command(os.Args[0], "daemon")
	cmd.ExtraFiles = []*os.File{w}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}
	_ = w.Close()

	// Add timeout to reader
	dataChan := make(chan byte, 1)
	errChan := make(chan error, 1)

	go func() {
		buf := make([]byte, 1)
		if _, err := r.Read(buf); err != nil {
			errChan <- fmt.Errorf("failed to read from pipe: %w", err)
		}
		dataChan <- '0'
	}()

	select {
	case err := <-errChan:
		return err
	case <-dataChan:
		break
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout exceed. Could not read pipe for daemon readiness")
	}

	return nil
}

// StopProxy signals the daemon and waits until it has released the pidfile
// and control socket. It uses the socket dial (same test as IsDaemonRunning
// and ipc.Listen) to decide liveness, and only the pidfile to find the
// process to signal.
func StopProxy() error {
	pidFilePath, err := paths.GetPIDFilePath()
	if err != nil {
		return err
	}
	socketPath := paths.GetSocketPath()

	pidBytes, readErr := os.ReadFile(pidFilePath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("failed to read %s: %w", pidFilePath, readErr)
	}
	pidExists := readErr == nil
	running := socketAccepting(socketPath)

	switch {
	case !running && !pidExists:
		// Already stopped and no leftovers to reap.
		return nil
	case !running:
		// Socket dead but leftovers remain (e.g. kill -9): nothing to
		// signal. Reap stale files so the next start finds clean state.
		// The socket was just probed as not accepting, so it is safe to
		// unlink (same rule waitForShutdown applies).
		if err := os.Remove(pidFilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	case !pidExists:
		// Daemon is accepting on the socket but there is no pidfile to
		// signal it with; do not guess a PID.
		return fmt.Errorf("daemon is running at %s but %s is missing; cannot signal it", socketPath, pidFilePath)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return fmt.Errorf("failed to convert daemon PID '%s' to int: %w", string(pidBytes), err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find proxy process: %w", err)
	}

	// A daemon that already exited (ErrProcessDone/ESRCH) is fine here:
	// waitForShutdown detects unclean death and reaps stale files.
	if err := p.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("failed to signal proxy process: %w", err)
	}

	return waitForShutdown(pid)
}

// waitForShutdown blocks until the daemon has released everything a new
// instance needs (pidfile + control socket), so a StartProxy right after
// StopProxy cannot race the old instance's graceful shutdown. pid detects an
// unclean death (e.g. kill -9): nothing will ever clean up then, so stale
// files are reaped here instead.
func waitForShutdown(pid int) error {
	pidFilePath, err := paths.GetPIDFilePath()
	if err != nil {
		return err
	}
	socketPath := paths.GetSocketPath()

	deadline := time.Now().Add(stopGracePeriod)
	for {
		if fileGone(pidFilePath) && !socketAccepting(socketPath) {
			return nil
		}

		if !pidAlive(pid) {
			// The daemon died without cleaning up: remove stale leftovers so
			// the next start finds a clean state.
			if err := os.Remove(pidFilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			// Re-probe and never unlink a socket another daemon is still
			// accepting on.
			if !socketAccepting(socketPath) {
				if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			return nil
		}

		if time.Now().After(deadline) {
			logPath, _ := paths.GetLogFilePath()
			return fmt.Errorf("timed out after %v waiting for daemon (pid %d) to stop; see %s", stopGracePeriod, pid, logPath)
		}
		time.Sleep(stopPollEvery)
	}
}

// socketAccepting is the same test ipc.Listen uses to refuse a second daemon:
// false means a new daemon is free to start (stale socket files included).
func socketAccepting(socketPath string) bool {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func fileGone(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

// pidAlive reports whether pid names a live process. Note signal 0 cannot
// distinguish PID reuse; callers treat file/socket state
// as the primary evidence and liveness only as a fallback.
func pidAlive(pid int) bool {
	return registry.ProcessAlive(pid)
}

func randomNum() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	min := 4000
	max := 4999
	return r.Intn(max-min+1) + min
}

func GetRandomPort() (int, error) {
	port := randomNum()

	start := time.Now()
	const maxTimeout = 10 * time.Second

	// Check if port in use
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	for err != nil {
		if time.Since(start) > maxTimeout {
			return 0, fmt.Errorf("could not find free port within %v", maxTimeout)
		}

		port = randomNum()
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	}
	_ = listener.Close()
	return port, nil
}

// SetEnv returns env with key set to value, replacing any existing entry
// instead of appending a duplicate.
func SetEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func CleanUpCommand(command *exec.Cmd, name string, res *ipc.Response) error {
	conn, err := ConnectToSocket()
	if err != nil {
		_ = command.Process.Signal(os.Interrupt)
		return err
	}
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	req := &ipc.Request{Command: ipc.CmdAliasRemove, Hostname: name}
	if err := SendRequest(encoder, req); err != nil {
		return err
	}
	if err := GetResponse(decoder, res); err != nil {
		return err
	}
	return nil
}

func Success(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s[SUCCESS]%s %s", ColorGreen, ColorReset, msg)
}

func Info(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s[INFO]%s %s", ColorBlue, ColorReset, msg)
}

func Warning(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s[WARN]%s %s", ColorYellow, ColorReset, msg)
}

func Error(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s[ERROR]%s %s", ColorRed, ColorReset, msg)
}
