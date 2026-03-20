//go:build windows
// +build windows

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"horihiro.net/runx/commands/utils"
)

func listProxiesPlatform(shellOverride string) ([]string, error) {
	if shellOverride != "" {
		return nil, fmt.Errorf("--shell is supported only on Linux")
	}
	candidateDirs := []string{}

	if proxyDir, err := windowsProxyDir(); err == nil {
		candidateDirs = append(candidateDirs, proxyDir)
	}
	if machineDir, err := machineProxyDir(); err == nil {
		candidateDirs = append(candidateDirs, machineDir)
	}

	// Backward compatibility: proxies created by old versions next to runx.
	runxPath, err := os.Executable()
	if err == nil {
		runxPath, _ = filepath.EvalSymlinks(runxPath)
		candidateDirs = append(candidateDirs, filepath.Dir(runxPath))
	}

	seen := map[string]bool{}
	result := []string{}

	for _, dir := range candidateDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read proxy directory %s: %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.EqualFold(filepath.Ext(name), ".cmd") {
				continue
			}

			fullPath := filepath.Join(dir, name)
			managed, err := utils.IsManagedProxy(fullPath)
			if err != nil || !managed {
				continue
			}

			command := strings.TrimSuffix(name, filepath.Ext(name))
			lower := strings.ToLower(command)
			if seen[lower] {
				continue
			}
			seen[lower] = true
			result = append(result, command)
		}
	}

	sort.Strings(result)
	return result, nil
}
