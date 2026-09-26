/*
Copyright © 2026 Pablo Muñoz
*/
package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ppablomunoz/noports/internal/client"
	"github.com/ppablomunoz/noports/internal/paths"
	"github.com/spf13/cobra"
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show daemon logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		n, _ := cmd.Flags().GetInt("tail")
		follow, _ := cmd.Flags().GetBool("follow")

		logPath, err := paths.GetLogFilePath()
		if err != nil {
			return err
		}
		if !paths.Exists(logPath) {
			client.Info("No log file yet at %s (daemon never started?)\n", logPath)
			return nil
		}

		if err := printTail(logPath, n); err != nil {
			return err
		}
		if follow {
			return followFile(logPath)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().IntP("tail", "n", 50, "Show last N lines (0 = all)")
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
}

func printTail(logPath string, n int) error {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", logPath, err)
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, l := range lines {
		fmt.Println(l)
	}
	return nil
}

func followFile(logPath string) error {
	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", logPath, err)
	}
	defer func() { _ = f.Close() }()

	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek %s: %w", logPath, err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	buf := make([]byte, 4096)
	for {
		select {
		case <-sigCh:
			return nil
		case <-time.After(250 * time.Millisecond):
		}

		st, err := f.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", logPath, err)
		}
		if st.Size() < offset {
			// Truncated (or rotated): start over.
			offset = 0
		}
		if st.Size() == offset {
			continue
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek %s: %w", logPath, err)
		}
		nr, err := f.Read(buf)
		if err != nil && nr == 0 {
			continue
		}
		offset += int64(nr)
		fmt.Print(string(buf[:nr]))
	}
}
