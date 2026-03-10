package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ResolveLinuxShell(override string) (string, error) {
	override = strings.ToLower(strings.TrimSpace(override))
	if override != "" {
		if !IsSupportedLinuxShell(override) {
			return "", fmt.Errorf("unsupported shell: %s (supported: bash|zsh|fish)", override)
		}
		return override, nil
	}

	detected := strings.ToLower(strings.TrimSpace(filepath.Base(os.Getenv("SHELL"))))
	if IsSupportedLinuxShell(detected) {
		return detected, nil
	}

	// Safe fallback.
	return "bash", nil
}

func IsSupportedLinuxShell(shell string) bool {
	switch shell {
	case "bash", "zsh", "fish":
		return true
	default:
		return false
	}
}

func LinuxShellRCPath(shell string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	switch shell {
	case "bash":
		return filepath.Join(homeDir, ".bashrc"), nil
	case "zsh":
		return filepath.Join(homeDir, ".zshrc"), nil
	default:
		return "", fmt.Errorf("rc file is not supported for shell: %s", shell)
	}
}

func FishFunctionsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "fish", "functions"), nil
}
