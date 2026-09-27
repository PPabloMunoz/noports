package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	_ = w.Close()
	os.Stdout = old
	out := make([]byte, 0, 4096)
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			break
		}
	}
	_ = r.Close()
	return string(out)
}

func TestPrintLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.log")
	content := "one\ntwo\nthree\nfour\nfive\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp log: %v", err)
	}

	got := captureStdout(t, func() {
		if err := printLastLines(path, 3); err != nil {
			t.Errorf("printLastLines: %v", err)
		}
	})
	if got != "three\nfour\nfive\n" {
		t.Fatalf("printLastLines tail = %q, want last 3 lines", got)
	}

	got = captureStdout(t, func() {
		if err := printLastLines(path, 0); err != nil {
			t.Errorf("printLastLines: %v", err)
		}
	})
	if got != content {
		t.Fatalf("printLastLines all = %q, want full content", got)
	}
}
