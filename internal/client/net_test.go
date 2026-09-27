package client

import (
	"net"
	"testing"
	"time"
)

func TestWaitForTCPReady(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := WaitForTCP(port, 2*time.Second); err != nil {
		t.Fatalf("WaitForTCP(ready port): %v", err)
	}
}

func TestWaitForTCPTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	if err := WaitForTCP(port, 300*time.Millisecond); err == nil {
		t.Fatal("WaitForTCP(closed port) = nil error, want timeout error")
	}
}
