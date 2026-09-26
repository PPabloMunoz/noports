package ipc

import (
	"fmt"
	"net"
	"os"

	"github.com/ppablomunoz/noports/internal/paths"
)

// Listen creates the unix socket listener, removing a stale socket file.
// It dials first to refuse starting a second daemon.
func Listen() (net.Listener, error) {
	socketPath := paths.Socket()

	conn, err := net.Dial("unix", socketPath)
	if err == nil {
		_ = conn.Close()
		return nil, fmt.Errorf("daemon already running at %s", socketPath)
	}

	// Connection failed — the socket file is either absent or stale.
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove stale socket at %s: %w", socketPath, err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on socket: %w", err)
	}
	return listener, nil
}

// Dial connects to the running daemon.
func Dial() (net.Conn, error) {
	return net.Dial("unix", paths.Socket())
}
