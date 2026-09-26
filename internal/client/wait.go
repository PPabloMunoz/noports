package client

import (
	"fmt"
	"net"
	"time"
)

// WaitForTCP polls 127.0.0.1:port until something accepts TCP or timeout
// elapses. Used by `run --wait` so the URL is only reported once the
// backend is actually reachable instead of serving instant 502s.
func WaitForTCP(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for {
		conn, err := net.DialTimeout("tcp", addr, min(time.Second, time.Until(deadline)))
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no TCP listener on %s after %s", addr, timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
