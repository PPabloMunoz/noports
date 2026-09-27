package client

import (
	"testing"
)

func TestLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello\n"},
		{"hello\n", "hello\n"},
		{"hello\n\n", "hello\n"},
		{"", "\n"},
	}
	for _, tt := range tests {
		if got := line(tt.input); got != tt.want {
			t.Errorf("line(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPaintNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := paint(colorRed, "[ERROR]"); got != "[ERROR]" {
		t.Fatalf("paint with NO_COLOR = %q, want plain input", got)
	}
}
