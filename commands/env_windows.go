//go:build windows
// +build windows

package commands

import (
	"fmt"
	"os"
)

func resolveRegisteredEnvFiles(command, shellOverride string) ([]string, error) {
	if shellOverride != "" {
		return nil, fmt.Errorf("--shell is supported only on Linux")
	}

	proxyPath, _, err := findExistingProxyPath(command)
	if err != nil {
		return nil, err
	}

	contentBytes, err := os.ReadFile(proxyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proxy: %w", err)
	}

	return parseWindowsEnvFiles(string(contentBytes)), nil
}
