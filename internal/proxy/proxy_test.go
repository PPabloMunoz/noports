package proxy

import (
	"testing"
)

func TestHostOnly(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Example.COM:443", "example.com"},
		{"example.com", "example.com"},
		{"  API.Localhost:8443  ", "api.localhost"},
		{"127.0.0.1:80", "127.0.0.1"},
		{"[::1]:443", "::1"},
		{"localhost", "localhost"},
	}
	for _, tt := range tests {
		if got := hostOnly(tt.input); got != tt.want {
			t.Errorf("hostOnly(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"[::1]", true},
		{"127.0.0.2", true},
		{"localhost", false},
		{"example.com", false},
		{"192.168.1.1", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isLoopback(tt.input); got != tt.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
