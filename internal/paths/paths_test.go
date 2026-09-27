package paths_test

import (
	"testing"

	"github.com/ppablomunoz/noports/internal/paths"
)

func TestSocket(t *testing.T) {
	t.Setenv("NOPORTS_SOCKET", "/tmp/test-noports.sock")
	if got := paths.Socket(); got != "/tmp/test-noports.sock" {
		t.Fatalf("Socket() = %q, want override value", got)
	}
}

func TestSocketDefault(t *testing.T) {
	t.Setenv("NOPORTS_SOCKET", "")
	if got := paths.Socket(); got != paths.DefaultSocketPath {
		t.Fatalf("Socket() = %q, want default %q", got, paths.DefaultSocketPath)
	}
}
