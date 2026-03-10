package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"horihiro.net/runx/commands/utils"
)

func RemoveCommand(args []string) error {
	command, shellOverride, err := parseRemoveArgs(args)
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if shellOverride != "" {
			return fmt.Errorf("--shell is supported only on Linux")
		}
		return removeCommandWindows(command)
	}

	shellName, err := utils.ResolveLinuxShell(shellOverride)
	if err != nil {
		return err
	}
	return removeCommandLinux(command, shellName)
}

func parseRemoveArgs(args []string) (string, string, error) {
	command := ""
	shellOverride := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--shell=") {
			shellOverride = strings.TrimSpace(strings.TrimPrefix(arg, "--shell="))
			if shellOverride == "" {
				return "", "", fmt.Errorf("--shell requires a value")
			}
			continue
		}
		if arg == "--shell" {
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--shell requires a value")
			}
			shellOverride = strings.TrimSpace(args[i+1])
			if shellOverride == "" {
				return "", "", fmt.Errorf("--shell requires a value")
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return "", "", fmt.Errorf("unknown option for remove: %s", arg)
		}
		if command != "" {
			return "", "", fmt.Errorf("usage: runx remove COMMAND [--shell=bash|zsh|fish]")
		}
		command = strings.TrimSpace(arg)
	}

	if command == "" {
		return "", "", fmt.Errorf("usage: runx remove COMMAND [--shell=bash|zsh|fish]")
	}
	return command, shellOverride, nil
}

func removeCommandWindows(command string) error {
	shimDir, err := windowsShimDir()
	if err != nil {
		return err
	}
	shimPath := filepath.Join(shimDir, command+".cmd")

	managed, err := utils.IsManagedShim(shimPath)
	if err == nil {
		if !managed {
			return fmt.Errorf("file is not a runx-generated shim: %s", shimPath)
		}
		if err := os.Remove(shimPath); err != nil {
			return fmt.Errorf("failed to remove shim: %w", err)
		}
		fmt.Printf("Shim removed: %s\n", shimPath)
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	// Fallback for shims created by older versions near runx executable.
	runxPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve runx path: %w", err)
	}
	runxPath, err = filepath.EvalSymlinks(runxPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink for runx path: %w", err)
	}
	runxDir := filepath.Dir(runxPath)
	shimPath = filepath.Join(runxDir, command+".cmd")
	managed, err = utils.IsManagedShim(shimPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Also check machine shim directory.
			if machineDir, mErr := machineShimDir(); mErr == nil {
				machineShim := filepath.Join(machineDir, command+".cmd")
				if mManaged, mErr := utils.IsManagedShim(machineShim); mErr == nil {
					if !mManaged {
						return fmt.Errorf("file is not a runx-generated shim: %s", machineShim)
					}
					if err := os.Remove(machineShim); err != nil {
						return fmt.Errorf("failed to remove machine shim (try running as Administrator): %w", err)
					}
					fmt.Printf("Machine shim removed: %s\n", machineShim)
					return nil
				}
			}
			return fmt.Errorf("shim not found: %s", filepath.Join(shimDir, command+".cmd"))
		}
		return err
	}
	if !managed {
		return fmt.Errorf("file is not a runx-generated shim: %s", shimPath)
	}

	if err := os.Remove(shimPath); err != nil {
		return fmt.Errorf("failed to remove shim: %w", err)
	}

	fmt.Printf("Shim removed: %s\n", shimPath)
	return nil
}

func removeCommandLinux(command, shellName string) error {
	if shellName == "fish" {
		return removeCommandLinuxFish(command)
	}
	return removeCommandLinuxPosix(command, shellName)
}

func removeCommandLinuxPosix(command, shellName string) error {
	rcPath, err := utils.LinuxShellRCPath(shellName)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return fmt.Errorf("failed to read shell rc file: %w", err)
	}

	marker := "# " + utils.ShimMarker + " " + command
	if !strings.Contains(string(content), marker) {
		return fmt.Errorf("function not found in %s: %s", rcPath, command)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	inFunction := false
	foundFunction := false

	for _, line := range lines {
		if strings.Contains(line, marker) {
			inFunction = true
			foundFunction = true
			continue
		}
		if inFunction && line == "}" {
			inFunction = false
			continue
		}
		if !inFunction {
			newLines = append(newLines, line)
		}
	}

	if !foundFunction {
		return fmt.Errorf("function not found in %s: %s", rcPath, command)
	}

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(rcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write shell rc file: %w", err)
	}

	fmt.Printf("Function removed from %s\n", rcPath)
	fmt.Printf("Note: Run 'source %s' or restart your shell for changes to take effect.\n", rcPath)
	return nil
}

func removeCommandLinuxFish(command string) error {
	functionsDir, err := utils.FishFunctionsDir()
	if err != nil {
		return err
	}
	functionPath := filepath.Join(functionsDir, command+".fish")

	content, err := os.ReadFile(functionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("function file not found: %s", functionPath)
		}
		return fmt.Errorf("failed to read fish function file: %w", err)
	}

	marker := "# " + utils.ShimMarker + " " + command
	if !strings.Contains(string(content), marker) {
		return fmt.Errorf("file is not a runx-generated fish function: %s", functionPath)
	}

	if err := os.Remove(functionPath); err != nil {
		return fmt.Errorf("failed to remove fish function file: %w", err)
	}

	fmt.Printf("Fish function removed: %s\n", functionPath)
	return nil
}
