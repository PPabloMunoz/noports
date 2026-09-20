package client

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
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

func PrintRoutesTable(routes []registry.Route) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	_, _ = fmt.Fprintf(w, "NAME\tHOST\tPORT\tPID\n")
	for _, r := range routes {
		name := strings.Split(r.Hostname, ".")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", name[0], r.Hostname, r.Port, r.PID)
	}

	_ = w.Flush()
}

func IsDaemonRunning() (bool, error) {
	pidFilePath, err := paths.GetPIDFilePath()
	if err != nil {
		return false, err
	}

	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil {
		return false, nil
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return false, nil
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}

	if err := p.Signal(syscall.Signal(0)); err != nil {
		return false, nil
	}
	return true, nil
}

func StartProxy() error {
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	cmd := exec.Command(os.Args[0], "daemon")
	cmd.ExtraFiles = []*os.File{w}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}
	_ = w.Close()

	buf := make([]byte, 1)
	if _, err := r.Read(buf); err != nil {
		return fmt.Errorf("failed to read from pipe: %w", err)
	}
	_ = r.Close()
	return nil
}

func StopProxy() error {
	running, err := IsDaemonRunning()
	if err != nil {
		return err
	}

	if !running {
		return nil
	}

	pidFilePath, err := paths.GetPIDFilePath()
	if err != nil {
		return err
	}
	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", pidFilePath, err)
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return fmt.Errorf("failed to convert daemon PID '%s' to int: %w", string(pidBytes), err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find proxy process: %w", err)
	}

	if err := p.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to kill proxy process: %w", err)
	}

	return nil
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
	fmt.Printf("%s%s%s", ColorGreen, msg, ColorReset)
}

func Info(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s%s%s", ColorBlue, msg, ColorReset)
}

func Warning(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s%s%s", ColorYellow, msg, ColorReset)
}

func Error(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s%s%s", ColorRed, msg, ColorReset)
}
