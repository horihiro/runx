//go:build !windows
// +build !windows

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
	shellName, err := utils.ResolveLinuxShell(shellOverride)
	if err != nil {
		return nil, err
	}
	return listLinuxProxies(shellName)
}

func listLinuxProxies(shellName string) ([]string, error) {
	if shellName == "fish" {
		return listFishProxies()
	}
	return listPosixProxies(shellName)
}

func listPosixProxies(shellName string) ([]string, error) {
	rcPath, err := utils.LinuxShellRCPath(shellName)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read shell rc file: %w", err)
	}

	prefix := "# " + utils.ProxyMarker + " "
	seen := map[string]bool{}
	result := []string{}

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}

	sort.Strings(result)
	return result, nil
}

func listFishProxies() ([]string, error) {
	functionsDir, err := utils.FishFunctionsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(functionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read fish functions directory: %w", err)
	}

	result := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".fish" {
			continue
		}
		fullPath := filepath.Join(functionsDir, entry.Name())
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), "# "+utils.ProxyMarker+" ") {
			continue
		}
		result = append(result, strings.TrimSuffix(entry.Name(), ".fish"))
	}

	sort.Strings(result)
	return result, nil
}
