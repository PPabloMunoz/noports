package ipc

import (
	"fmt"
	"net"
	"os"

	"github.com/ppablomunoz/noports/internal/paths"
)

// Listen creates the control socket listener, removing a stale socket file. It refuses to start when another daemon is already serving.
func Listen() (net.Listener, error) {
	socketPath := paths.Socket()

	if IsServing() {
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

// Dial connects to the running daemon over the control socket. Callers must close the returned connection when done.
func Dial() (net.Conn, error) {
	return net.Dial("unix", paths.Socket())
}

// IsServing reports whether a daemon accepts control-socket connections. It dials and closes immediately without sending a request; false means a new daemon is free to start.
func IsServing() bool {
	conn, err := Dial()
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
