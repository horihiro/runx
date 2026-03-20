//go:build windows
// +build windows

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"horihiro.net/runx/commands/utils"
)

var (
	windowsEnvFileRegexp = regexp.MustCompile(`--envfile="([^"]*)"`)
	windowsOrigRegexp    = regexp.MustCompile(`exec(?:\s+--envfile="[^"]*")*\s+"([^"]+)"\s+%\*`)
)

func showCommandPlatform(command, shellOverride string) error {
	if shellOverride != "" {
		return fmt.Errorf("--shell is supported only on Linux")
	}

	proxyPath, scope, err := findExistingProxyPath(command)
	if err != nil {
		return err
	}

	contentBytes, err := os.ReadFile(proxyPath)
	if err != nil {
		return fmt.Errorf("failed to read proxy: %w", err)
	}
	content := string(contentBytes)

	envFiles := parseWindowsEnvFiles(content)
	original := parseWindowsOriginalCommand(content, command)
	runxPath, err := resolveSelfPath()
	if err != nil {
		return err
	}
	originalPath := firstNonManagedWindowsCommandPath(original)
	_, previewContent := buildProxyWindows(command, original, envFiles, runxPath, originalPath, filepath.Dir(proxyPath))

	fmt.Printf("Existing runx %s proxy:\n", scope)
	fmt.Printf("  Command: %s\n", command)
	if command != original {
		fmt.Printf("  Original command: %s\n", original)
	}
	fmt.Printf("  Path: %s\n", proxyPath)
	if len(envFiles) > 0 {
		fmt.Printf("  Environment files: %v\n", envFiles)
	} else {
		fmt.Println("  Environment files: (none)")
	}
	if originalPath != "" {
		fmt.Printf("  Current command path: %s\n", originalPath)
	} else {
		fmt.Println("  Current command path: (not found)")
	}
	fmt.Println("\nProxy content (current):")
	fmt.Println(previewContent)
	return nil
}

func findExistingProxyPath(command string) (string, string, error) {
	if proxyDir, err := windowsProxyDir(); err == nil {
		proxyPath := filepath.Join(proxyDir, command+".cmd")
		if managed, mErr := utils.IsManagedProxy(proxyPath); mErr == nil && managed {
			return proxyPath, "user", nil
		}
	}

	// Backward compatibility: proxies created by old versions next to runx.
	runxPath, err := os.Executable()
	if err == nil {
		runxPath, _ = filepath.EvalSymlinks(runxPath)
		runxDir := filepath.Dir(runxPath)
		legacy := filepath.Join(runxDir, command+".cmd")
		if managed, mErr := utils.IsManagedProxy(legacy); mErr == nil && managed {
			return legacy, "user", nil
		}
	}

	if machineDir, err := machineProxyDir(); err == nil {
		machineProxy := filepath.Join(machineDir, command+".cmd")
		if managed, mErr := utils.IsManagedProxy(machineProxy); mErr == nil && managed {
			return machineProxy, "machine", nil
		}
	}

	return "", "", fmt.Errorf("proxy not found: %s", command)
}

func parseWindowsEnvFiles(content string) []string {
	matches := windowsEnvFileRegexp.FindAllStringSubmatch(content, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			result = append(result, m[1])
		}
	}
	return result
}

func parseWindowsOriginalCommand(content, fallback string) string {
	m := windowsOrigRegexp.FindStringSubmatch(content)
	if len(m) >= 2 && m[1] != "" {
		return m[1]
	}
	return fallback
}

func firstNonManagedWindowsCommandPath(command string) string {
	for _, p := range findWindowsCommandPaths(command) {
		managed, err := utils.IsManagedProxy(p)
		if err == nil && managed {
			continue
		}
		if err != nil {
			continue
		}
		return p
	}
	return ""
}
