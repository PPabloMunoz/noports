package client

import (
	"fmt"
	"os"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

// Success prints a green SUCCESS message to stdout. It formats the message before printing and respects color settings.
func Success(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s %s", paint(colorGreen, "[SUCCESS]"), msg)
}

// Info prints a blue INFO message to stdout. It formats the message before printing and respects color settings.
func Info(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s %s", paint(colorBlue, "[INFO]"), msg)
}

// Warning prints a yellow WARN message to stdout. It formats the message before printing and respects color settings.
func Warning(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s %s", paint(colorYellow, "[WARN]"), msg)
}

// Error prints a red ERROR message to stdout. It formats the message before printing and respects color settings.
func Error(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s %s", paint(colorRed, "[ERROR]"), msg)
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
