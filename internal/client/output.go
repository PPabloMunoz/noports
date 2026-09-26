package client

import (
	"fmt"
	"os"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

// Success prints a green SUCCESS line to stdout. It owns the trailing newline, so callers pass messages without one.
func Success(format string, v ...any) {
	fmt.Printf("%s %s", paint(colorGreen, "[SUCCESS]"), line(fmt.Sprintf(format, v...)))
}

// Info prints a blue INFO line to stdout. It owns the trailing newline, so callers pass messages without one.
func Info(format string, v ...any) {
	fmt.Printf("%s %s", paint(colorBlue, "[INFO]"), line(fmt.Sprintf(format, v...)))
}

// Warning prints a yellow WARN line to stdout. It owns the trailing newline, so callers pass messages without one.
func Warning(format string, v ...any) {
	fmt.Printf("%s %s", paint(colorYellow, "[WARN]"), line(fmt.Sprintf(format, v...)))
}

// Error prints a red ERROR line to stderr. It owns the trailing newline, so callers pass messages without one.
func Error(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "%s %s", paint(colorRed, "[ERROR]"), line(fmt.Sprintf(format, v...)))
}

// line returns s with exactly one trailing newline. It lets helpers own line discipline instead of every caller.
func line(s string) string {
	return strings.TrimSuffix(s, "\n") + "\n"
}

// paint wraps s in ANSI color codes, or returns it plain when colors are disabled.
func paint(color, s string) string {
	if !colorEnabled() {
		return s
	}
	return color + s + colorReset
}

// colorEnabled reports whether ANSI colors may be emitted. It requires a TTY stdout, a non-dumb TERM, and an unset NO_COLOR.
func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	st, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
