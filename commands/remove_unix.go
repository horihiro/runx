//go:build !windows
// +build !windows

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"horihiro.net/runx/commands/utils"
)

func removeCommandPlatform(command, shellOverride string) error {
	shellName, err := utils.ResolveLinuxShell(shellOverride)
	if err != nil {
		return err
	}
	return removeCommandLinux(command, shellName)
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

	marker := "# " + utils.ProxyMarker + " " + command
	foundMarker := false
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == marker {
			foundMarker = true
			break
		}
	}
	if !foundMarker {
		return fmt.Errorf("function not found in %s: %s", rcPath, command)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	inFunction := false
	foundFunction := false

	for _, line := range lines {
		if strings.TrimSpace(line) == marker {
			inFunction = true
			foundFunction = true
			continue
		}
		if inFunction && strings.TrimSpace(line) == "}" {
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
	fmt.Println("Note: 'source' alone may not remove an already-loaded function from the current shell session.")
	fmt.Println("      Please restart your shell, or run: unset -f " + command)
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

	marker := "# " + utils.ProxyMarker + " " + command
	if !strings.Contains(string(content), marker) {
		return fmt.Errorf("file is not a runx-generated fish function: %s", functionPath)
	}

	if err := os.Remove(functionPath); err != nil {
		return fmt.Errorf("failed to remove fish function file: %w", err)
	}

	fmt.Printf("Fish function removed: %s\n", functionPath)
	fmt.Println("Note: If the function is already loaded in the current fish session,")
	fmt.Println("      restart fish or run: functions -e " + command)
	return nil
}
