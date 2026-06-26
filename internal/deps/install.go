package deps

import (
	"os/exec"
	"runtime"
	"strings"
)

// PackageManager describes how to install packages on the host system.
type PackageManager struct {
	Name string   // display name, e.g. "pacman"
	bin  string   // executable probed on PATH, e.g. "pacman"
	sudo bool     // whether installing needs sudo
	args []string // install subcommand + non-interactive flags
}

// managers lists supported package managers in preference order for the OS.
func managers() []PackageManager {
	brew := PackageManager{"brew", "brew", false, []string{"install"}}
	if runtime.GOOS == "darwin" {
		return []PackageManager{brew}
	}
	return []PackageManager{
		{"apt", "apt-get", true, []string{"install", "-y"}},
		{"dnf", "dnf", true, []string{"install", "-y"}},
		{"pacman", "pacman", true, []string{"-S", "--needed", "--noconfirm"}},
		{"zypper", "zypper", true, []string{"install", "-y"}},
		{"apk", "apk", true, []string{"add"}},
		brew, // Linuxbrew, last resort
	}
}

// DetectManager returns the first package manager found on PATH.
func DetectManager() (PackageManager, bool) {
	for _, m := range managers() {
		if _, err := exec.LookPath(m.bin); err == nil {
			return m, true
		}
	}
	return PackageManager{}, false
}

// Command builds the argv (incl. sudo when required) to install pkgs.
func (m PackageManager) Command(pkgs []string) []string {
	var argv []string
	if m.sudo {
		argv = append(argv, "sudo")
	}
	argv = append(argv, m.bin)
	argv = append(argv, m.args...)
	return append(argv, pkgs...)
}

// CommandString renders the install command for display.
func (m PackageManager) CommandString(pkgs []string) string {
	return strings.Join(m.Command(pkgs), " ")
}
