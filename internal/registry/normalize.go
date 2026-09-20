package registry

import (
	"fmt"
	"strings"
)

const localhostSuffix = ".localhost"

// NormalizeHostname accepts a bare name ("api", "API", " api ") or a
// fully-qualified hostname ("api.localhost", "API.LOCALHOST") and returns
// the canonical lowercase "name.localhost" form.
//
// It rejects empty input, the reserved "localhost" host, multi-label names
// ("foo.bar"), and anything outside RFC 1035 labels
// (lowercase alphanumerics and hyphens, 1-63 chars, no leading/trailing hyphen).
func NormalizeHostname(input string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.Trim(s, ".")
	if s == "" {
		return "", fmt.Errorf("name is required")
	}

	bare, _ := strings.CutSuffix(s, localhostSuffix)
	bare = strings.Trim(bare, ".")
	if bare == "" {
		return "", fmt.Errorf("invalid name %q: use letters, numbers and hyphens (e.g. api)", input)
	}
	if bare == "localhost" {
		return "", fmt.Errorf("name %q is reserved, choose another name", input)
	}
	if strings.Contains(bare, ".") {
		return "", fmt.Errorf("invalid name %q: use a single label without dots (e.g. api)", input)
	}
	if len(bare) > 63 {
		return "", fmt.Errorf("invalid name %q: must be 63 characters or fewer", input)
	}
	if !isValidLabel(bare) {
		return "", fmt.Errorf("invalid name %q: use lowercase letters, numbers and hyphens, starting and ending with a letter or number (e.g. api)", input)
	}

	return bare + localhostSuffix, nil
}

// BareName strips the ".localhost" suffix for display / env vars.
// It assumes input is already normalized; it lowercases and trims defensively.
func BareName(hostname string) string {
	s := strings.ToLower(strings.TrimSpace(hostname))
	return strings.TrimSuffix(s, localhostSuffix)
}

func isValidLabel(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		if !isLower && !isDigit && c != '-' {
			return false
		}
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	return true
}
