package registry

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// livenessSupported reports whether process-liveness probing is implemented
// on this OS. Only linux and darwin are supported — Windows is not supported
// by noports at all (see internal/pki/truststore_other.go). On any other OS
// every probe reports "unknown" so the orphan sweep never prunes.
func livenessSupported() bool {
	switch runtime.GOOS {
	case "linux", "darwin":
		return true
	default:
		return false
	}
}

// ProcessAlive reports whether pid names a live process (linux and darwin
// only; see livenessSupported).
//
// Note signal 0 cannot distinguish PID reuse: combine with ProcessStartTime
// (see SameProcess) wherever a recycled PID would cause harm.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		// Guard against 0 (current process group) and -1 (all caller-owned
		// processes): signalling those would broadcast, not probe.
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// ProcessStartTime returns the wall-clock start time of pid as Unix
// milliseconds, or an error if it cannot be determined (e.g. no such process).
//
// On Linux it is derived from /proc/<pid>/stat (starttime in clock ticks
// since boot) plus the boot time from /proc/stat. Elsewhere it falls back to
// `ps -o lstart=` (1-second granularity). Values are stable for the lifetime
// of a process: re-reading the same process later yields the same value, so
// exact equality detects PID reuse.
func ProcessStartTime(pid int) (int64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid %d", pid)
	}
	switch runtime.GOOS {
	case "linux":
		if ms, err := linuxStartTimeMs(pid); err == nil {
			return ms, nil
		}
		// /proc unavailable or unparsable; try ps as a fallback.
		return psStartTimeMs(pid)
	case "darwin":
		return psStartTimeMs(pid)
	default:
		return 0, fmt.Errorf("process start time lookup not supported on %s", runtime.GOOS)
	}
}

// SameProcess reports whether pid is alive AND is the same process instance
// that was recorded with start time recordedStartMs.
//
// A recorded start of 0 means "unknown" (routes persisted before start times
// were recorded): only liveness is checked. If the start time cannot be
// re-read for a live process, the process is assumed alive (fail open: a
// transient `ps` failure must not prune a live route; the next sweep retries).
func SameProcess(pid int, recordedStartMs int64) bool {
	if pid <= 0 {
		return false
	}
	if !livenessSupported() {
		return true // unsupported OS: unknown, fail open (never prune)
	}
	if !ProcessAlive(pid) {
		return false
	}
	if recordedStartMs == 0 {
		return true
	}
	cur, err := ProcessStartTime(pid)
	if err != nil {
		return true
	}
	return cur == recordedStartMs
}

// RouteAlive reports whether a run route's recorded processes are still the
// same live instances. Strict rule: the child backend AND the `run` wrapper
// must both be alive; a dead wrapper means nobody will clean the route up.
// Aliases (PID == -1) are user-managed and always report alive. Records
// without a wrapper PID (persisted before it was recorded) fall back to a
// child-only check.
func RouteAlive(r Route) bool {
	if !livenessSupported() {
		return true // unsupported OS: unknown, fail open (never prune)
	}
	if r.PID == -1 {
		return true
	}
	if r.PID <= 0 {
		return false
	}
	if !SameProcess(r.PID, r.ChildStartTime) {
		return false
	}
	if r.WrapperPID <= 0 {
		return true
	}
	return SameProcess(r.WrapperPID, r.WrapperStartTime)
}

// linuxClockTicksPerSec is USER_HZ. It is 100 on effectively all modern
// Linux kernels; since registration and sweep use the same constant on the
// same machine, reuse detection is exact even where it differs.
const linuxClockTicksPerSec = 100

func linuxStartTimeMs(pid int) (int64, error) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, err
	}
	// comm (field 2) is parenthesized and may itself contain spaces or
	// parens, so split after the LAST ')'.
	s := string(data)
	rparen := strings.LastIndexByte(s, ')')
	if rparen < 0 {
		return 0, fmt.Errorf("unparsable /proc/%d/stat", pid)
	}
	// Fields after ')' start at field 3; starttime is field 22.
	fields := strings.Fields(s[rparen+1:])
	if len(fields) < 20 {
		return 0, fmt.Errorf("unparsable /proc/%d/stat: too few fields", pid)
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unparsable starttime for pid %d: %w", pid, err)
	}
	btime, err := linuxBootTimeSec()
	if err != nil {
		return 0, err
	}
	return btime*1000 + int64(startTicks*1000/linuxClockTicksPerSec), nil
}

func linuxBootTimeSec() (int64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		f, ok := strings.CutPrefix(line, "btime ")
		if !ok {
			continue
		}
		return strconv.ParseInt(strings.TrimSpace(f), 10, 64)
	}
	return 0, fmt.Errorf("btime not found in /proc/stat")
}

var psStartLayouts = []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"}

func psStartTimeMs(pid int) (int64, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return 0, fmt.Errorf("ps lookup failed for pid %d: %w", pid, err)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0, fmt.Errorf("no such process: %d", pid)
	}
	for _, layout := range psStartLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.UnixMilli(), nil
		}
	}
	return 0, fmt.Errorf("unparsable ps lstart %q for pid %d", s, pid)
}
