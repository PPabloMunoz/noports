package registry

import (
	"strings"
	"testing"
)

func TestNormalizeHostname(t *testing.T) {
	valid63 := strings.Repeat("a", 63)
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"bare name", "api", "api.localhost", false},
		{"fully qualified", "api.localhost", "api.localhost", false},
		{"uppercase bare", "API", "api.localhost", false},
		{"uppercase qualified", "API.LOCALHOST", "api.localhost", false},
		{"surrounding spaces", "  api  ", "api.localhost", false},
		{"surrounding dots", ".api.", "api.localhost", false},
		{"hyphens inside", "my-api-1", "my-api-1.localhost", false},
		{"digits", "api123", "api123.localhost", false},
		{"max length label", valid63, valid63 + ".localhost", false},
		{"empty", "", "", true},
		{"blank", "   ", "", true},
		{"dots only", "...", "", true},
		{"reserved localhost", "localhost", "", true},
		{"reserved qualified", "localhost.localhost", "", true},
		{"multi label", "foo.bar", "", true},
		{"multi label qualified", "foo.bar.localhost", "", true},
		{"too long", strings.Repeat("a", 64), "", true},
		{"leading hyphen", "-api", "", true},
		{"trailing hyphen", "api-", "", true},
		{"underscore", "my_api", "", true},
		{"space inside", "my api", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeHostname(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeHostname(%q) = %q, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeHostname(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeHostname(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBareName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"api.localhost", "api"},
		{"API.LOCALHOST", "api"},
		{"  api.localhost  ", "api"},
		{"api", "api"},
		{"localhost", "localhost"},
	}
	for _, tt := range tests {
		if got := BareName(tt.input); got != tt.want {
			t.Errorf("BareName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
