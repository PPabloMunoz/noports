//go:build linux

package pki

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// System trust anchor locations, ordered by preference for checks.
// Debian/Ubuntu/Mint/Pop/Alpine (+ openSUSE via update-ca-certificates):
// /usr/local/share/ca-certificates/*.crt + update-ca-certificates.
// Fedora/RHEL/CentOS/Rocky/Alma + Arch/Manjaro:
// /etc/pki/ca-trust/source/anchors/ or /etc/ca-certificates/trust-source/anchors/ + update-ca-trust extract.
// openSUSE native: /etc/pki/trust/anchors/ + update-ca-certificates.
const (
	linuxAnchorDebian = "/usr/local/share/ca-certificates/noports-local-ca.crt"
	linuxAnchorRHEL   = "/etc/pki/ca-trust/source/anchors/noports-local-ca.pem"
	linuxAnchorArch   = "/etc/ca-certificates/trust-source/anchors/noports-local-ca.crt"
	linuxAnchorSUSE   = "/etc/pki/trust/anchors/noports-local-ca.pem"
)

// linuxInstallTarget is a destination anchor file plus the command that
// refreshes the system bundle after the file is placed/removed.
type linuxInstallTarget struct {
	dest        string
	refreshName string
	refreshArgs []string
}

// IsCATrusted reports whether the current local CA is installed in the
// system trust store. It returns false (not an error) when the CA file is
// missing or no anchor matches it.
func IsCATrusted() (bool, error) {
	certPath, err := GetCACertPath()
	if err != nil {
		return false, err
	}
	caBytes, err := os.ReadFile(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, anchor := range linuxAnchorPaths() {
		if anchorMatches(anchor, caBytes) {
			return true, nil
		}
	}

	// Secondary check via p11-kit: the anchor file may live somewhere
	// distro-specific while still trusted (e.g. custom trust module path).
	if commandExists("trust") && trustAnchorShows(caBytes) {
		return true, nil
	}

	return false, nil
}

func installCA() error {
	certPath, err := GetCACertPath()
	if err != nil {
		return fmt.Errorf("failed to get CA cert path: %w", err)
	}
	caBytes, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read CA cert: %w", err)
	}

	target, trustStore, err := pickInstallTarget()
	if err != nil {
		return err
	}

	// Pure p11-kit fallback when neither updater exists.
	if trustStore {
		cmd := privilegedCommand("trust", "anchor", "--store", certPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to store CA with trust anchor: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := installFilePrivileged(caBytes, target.dest); err != nil {
		return err
	}
	if err := refreshTrust(target); err != nil {
		return err
	}
	return nil
}

// UninstallCA removes the local CA from all known system anchor locations
// and refreshes the bundle. Missing files are ignored so the operation is
// idempotent.
func UninstallCA() error {
	for _, anchor := range linuxAnchorPaths() {
		if err := removeFilePrivileged(anchor); err != nil {
			return err
		}
	}

	// Best effort: drop the p11-kit entry too when the tool exists.
	// File removal above already covers the common case; failures here
	// must not fail the uninstall when the cert file is already gone.
	if commandExists("trust") {
		if certPath, err := GetCACertPath(); err == nil {
			if _, statErr := os.Stat(certPath); statErr == nil {
				cmd := privilegedCommand("trust", "anchor", "--remove", certPath)
				_ = cmd.Run()
			}
		}
	}

	var refreshErr error
	if commandExists("update-ca-certificates") {
		if err := runPrivileged("update-ca-certificates"); err != nil {
			refreshErr = err
		}
	}
	if commandExists("update-ca-trust") {
		if err := runPrivileged("update-ca-trust", "extract"); err != nil {
			refreshErr = err
		} else if commandExists("trust") {
			// Arch derivatives also need the compat bundle for OpenSSL.
			_ = runPrivileged("trust", "extract-compat")
		}
	}
	return refreshErr
}

// linuxAnchorPaths returns every known anchor location for cleanup and
// trust checks.
func linuxAnchorPaths() []string {
	return []string{
		linuxAnchorDebian,
		linuxAnchorRHEL,
		linuxAnchorArch,
		linuxAnchorSUSE,
	}
}

// pickInstallTarget selects the anchor destination and refresh command.
// It prefers the native tool for the current distro and falls back to
// whatever updater is available. The trustStore return is true when only
// `trust anchor --store` is available (no file copy needed).
func pickInstallTarget() (linuxInstallTarget, bool, error) {
	ids := distroIDs()
	hasCerts := commandExists("update-ca-certificates")
	hasTrustExtract := commandExists("update-ca-trust")
	hasTrust := commandExists("trust")

	if hasID(ids, "nixos") {
		return linuxInstallTarget{}, false, fmt.Errorf(
			"nixos manages the trust store declaratively; add %s to security.pki.certificates in configuration.nix",
			"~/.noports/certs/ca.pem",
		)
	}

	isDebianLike := hasAnyID(ids, "debian", "ubuntu", "linuxmint", "pop", "raspbian", "kali", "neon", "parrot", "deepin", "zorin", "elementary")
	isAlpine := hasID(ids, "alpine")
	isSUSE := hasAnyID(ids, "opensuse", "sles", "sled", "suse")
	isRHEL := hasAnyID(ids, "fedora", "rhel", "centos", "rocky", "alma", "ol", "amzn", "scientific")
	isArch := hasAnyID(ids, "arch", "manjaro", "endeavouros", "garuda", "cachyos")

	switch {
	case isSUSE:
		if hasCerts {
			return linuxInstallTarget{dest: linuxAnchorSUSE, refreshName: "update-ca-certificates"}, false, nil
		}
		if hasTrustExtract {
			return linuxInstallTarget{dest: linuxAnchorRHEL, refreshName: "update-ca-trust", refreshArgs: []string{"extract"}}, false, nil
		}
	case isDebianLike || isAlpine:
		if hasCerts {
			return linuxInstallTarget{dest: linuxAnchorDebian, refreshName: "update-ca-certificates"}, false, nil
		}
		// Debian-like without its updater but with p11-kit (rare).
		if hasTrustExtract {
			return linuxInstallTarget{dest: linuxAnchorRHEL, refreshName: "update-ca-trust", refreshArgs: []string{"extract"}}, false, nil
		}
	case isRHEL || isArch:
		if hasTrustExtract {
			return linuxInstallTarget{dest: rhelAnchorDest(), refreshName: "update-ca-trust", refreshArgs: []string{"extract"}}, false, nil
		}
		if isArch && hasTrust {
			// Arch without update-ca-trust wrapper still refreshes via p11-kit.
			return linuxInstallTarget{dest: linuxAnchorArch, refreshName: "trust", refreshArgs: []string{"extract-compat"}}, false, nil
		}
		if hasCerts {
			return linuxInstallTarget{dest: linuxAnchorDebian, refreshName: "update-ca-certificates"}, false, nil
		}
	}

	// Unknown distro: use whatever updater is present.
	if hasCerts {
		dest := linuxAnchorDebian
		if !dirExists(filepath.Dir(dest)) && dirExists(filepath.Dir(linuxAnchorSUSE)) {
			dest = linuxAnchorSUSE
		}
		return linuxInstallTarget{dest: dest, refreshName: "update-ca-certificates"}, false, nil
	}
	if hasTrustExtract {
		return linuxInstallTarget{dest: rhelAnchorDest(), refreshName: "update-ca-trust", refreshArgs: []string{"extract"}}, false, nil
	}
	if hasTrust {
		return linuxInstallTarget{}, true, nil
	}

	return linuxInstallTarget{}, false, fmt.Errorf(
		"no supported trust store tool found (need update-ca-certificates, update-ca-trust, or trust from p11-kit); " +
			"install ca-certificates or p11-kit and retry",
	)
}

// rhelAnchorDest prefers /etc/pki/... but falls back to the Arch-native
// directory when /etc/pki does not exist (minimal Arch containers).
func rhelAnchorDest() string {
	if dirExists("/etc/pki/ca-trust/source/anchors") {
		return linuxAnchorRHEL
	}
	if dirExists("/etc/ca-certificates/trust-source/anchors") {
		return linuxAnchorArch
	}
	return linuxAnchorRHEL
}

// refreshTrust regenerates the system bundle after an install.
func refreshTrust(target linuxInstallTarget) error {
	if target.refreshName == "" {
		return fmt.Errorf("no trust refresh command selected")
	}
	if err := runPrivileged(target.refreshName, target.refreshArgs...); err != nil {
		return fmt.Errorf("failed to refresh system trust (%s): %w", target.refreshName, err)
	}
	// Keep the OpenSSL compat bundle in sync on p11-kit systems.
	if target.refreshName == "update-ca-trust" && commandExists("trust") {
		_ = runPrivileged("trust", "extract-compat")
	}
	return nil
}

// anchorMatches reports whether path exists and holds the same bytes as
// the current CA (catches stale anchors after CA regeneration).
func anchorMatches(path string, want []byte) bool {
	got, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want))
}

// trustAnchorShows is a best-effort secondary check via p11-kit.
func trustAnchorShows(caBytes []byte) bool {
	cmd := exec.Command("trust", "anchor", "--show")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	if bytes.Contains(out, bytes.TrimSpace(caBytes)) {
		return true
	}
	return strings.Contains(string(out), CASubjectName)
}

// installFilePrivileged copies cert bytes to a root-owned anchor path.
// Non-root callers stage through a temp file and `sudo cp` so the
// destination keeps the right ownership; root writes directly.
func installFilePrivileged(data []byte, dest string) error {
	if os.Geteuid() == 0 {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dest, err)
		}
		return nil
	}

	for _, bin := range []string{"mkdir", "cp", "chmod"} {
		if !commandExists(bin) {
			return fmt.Errorf("missing required tool %q for privileged install", bin)
		}
	}

	tmp, err := os.CreateTemp("", "noports-ca-*.crt")
	if err != nil {
		return fmt.Errorf("failed to stage CA cert: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to stage CA cert: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to stage CA cert: %w", err)
	}
	defer func() { _ = os.Remove(tmpName) }()
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("failed to stage CA cert: %w", err)
	}

	if err := runPrivileged("mkdir", "-p", filepath.Dir(dest)); err != nil {
		return err
	}
	if err := runPrivileged("cp", tmpName, dest); err != nil {
		return fmt.Errorf("failed to copy CA to %s (hint: sudo access is required): %w", dest, err)
	}
	if err := runPrivileged("chmod", "644", dest); err != nil {
		return err
	}
	return nil
}

// removeFilePrivileged deletes an anchor path, ignoring missing files so
// uninstall stays idempotent.
func removeFilePrivileged(path string) error {
	if os.Geteuid() == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
		return nil
	}
	// sudo rm -f is already idempotent for missing files.
	if err := runPrivileged("rm", "-f", path); err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}
	return nil
}

// privilegedCommand builds a command that uses sudo when not root and
// inherits stdio so password prompts work, matching truststore_darwin.go.
func privilegedCommand(name string, args ...string) *exec.Cmd {
	if os.Geteuid() == 0 {
		cmd := exec.Command(name, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd
	}
	full := append([]string{name}, args...)
	cmd := exec.Command("sudo", full...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// runPrivileged runs a privileged command and wraps failures with output.
func runPrivileged(name string, args ...string) error {
	// Fast path for --show style probes is handled elsewhere; here we
	// always want stdio inherited, but capture output on failure via
	// a second run would lose the prompt, so use CombinedOutput only
	// when stdio is not a terminal? Keep it simple: inherit stdio and
	// wrap the exit error.
	cmd := privilegedCommand(name, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// distroIDs returns the lowercase ID and ID_LIKE tokens from
// /etc/os-release (e.g. {"ubuntu", "debian"}). Empty when unreadable.
func distroIDs() map[string]bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return map[string]bool{}
	}
	return parseOSReleaseIDs(string(data))
}

// parseOSReleaseIDs extracts ID and ID_LIKE tokens, lowercased.
func parseOSReleaseIDs(data string) map[string]bool {
	ids := map[string]bool{}
	for line := range strings.SplitSeq(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if key != "ID" && key != "ID_LIKE" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		for _, token := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ' ' || r == '\t' || r == ','
		}) {
			token = strings.ToLower(strings.TrimSpace(token))
			if token != "" {
				ids[token] = true
			}
		}
	}
	return ids
}

func hasID(ids map[string]bool, id string) bool {
	return ids[strings.ToLower(id)]
}

func hasAnyID(ids map[string]bool, candidates ...string) bool {
	for _, c := range candidates {
		if hasID(ids, c) {
			return true
		}
	}
	return false
}
