package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"horihiro.net/runx/commands/utils"
)

func ListCommand(args []string) error {
	shellOverride, err := parseListArgs(args)
	if err != nil {
		return err
	}

	var shims []string
	if runtime.GOOS == "windows" {
		if shellOverride != "" {
			return fmt.Errorf("--shell is supported only on Linux")
		}
		shims, err = listWindowsShims()
	} else {
		shellName, rErr := utils.ResolveLinuxShell(shellOverride)
		if rErr != nil {
			return rErr
		}
		shims, err = listLinuxShims(shellName)
	}
	if err != nil {
		return err
	}

	if len(shims) == 0 {
		fmt.Println("No runx shims found.")
		return nil
	}

	for _, name := range shims {
		fmt.Println(name)
	}
	return nil
}

func parseListArgs(args []string) (string, error) {
	shellOverride := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--shell=") {
			shellOverride = strings.TrimSpace(strings.TrimPrefix(arg, "--shell="))
			if shellOverride == "" {
				return "", fmt.Errorf("--shell requires a value")
			}
			continue
		}
		if arg == "--shell" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("--shell requires a value")
			}
			shellOverride = strings.TrimSpace(args[i+1])
			if shellOverride == "" {
				return "", fmt.Errorf("--shell requires a value")
			}
			i++
			continue
		}
		return "", fmt.Errorf("usage: runx list [--shell=bash|zsh|fish]")
	}
	return shellOverride, nil
}

func listLinuxShims(shellName string) ([]string, error) {
	if shellName == "fish" {
		return listFishShims()
	}
	return listPosixShims(shellName)
}

func listPosixShims(shellName string) ([]string, error) {
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

	prefix := "# " + utils.ShimMarker + " "
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

func listFishShims() ([]string, error) {
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
		if !strings.Contains(string(content), "# "+utils.ShimMarker+" ") {
			continue
		}
		result = append(result, strings.TrimSuffix(entry.Name(), ".fish"))
	}

	sort.Strings(result)
	return result, nil
}

func listWindowsShims() ([]string, error) {
	candidateDirs := []string{}

	shimDir, err := windowsShimDir()
	if err == nil {
		candidateDirs = append(candidateDirs, shimDir)
	}

	if machineDir, err := machineShimDir(); err == nil {
		candidateDirs = append(candidateDirs, machineDir)
	}

	// Backward compatibility: shims created by old versions next to runx.
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
			return nil, fmt.Errorf("failed to read shim directory %s: %w", dir, err)
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
			managed, err := utils.IsManagedShim(fullPath)
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
