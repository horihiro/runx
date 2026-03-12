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

func addCommandPlatform(command, originalCommand string, envFiles []string, shellOverride, runxPath string) error {
	shellName, err := utils.ResolveLinuxShell(shellOverride)
	if err != nil {
		return err
	}

	if shellName == "fish" {
		return addCommandLinuxFish(command, originalCommand, envFiles, runxPath)
	}
	return addCommandLinuxPosix(command, originalCommand, envFiles, runxPath, shellName)
}

func buildPosixFunction(command, originalCommand string, envFiles []string, runxPath string) string {
	var envArgs []string
	for _, f := range envFiles {
		envArgs = append(envArgs, "--envfile="+utils.QuoteForSh(f))
	}

	var content strings.Builder
	content.WriteString("# " + utils.ShimMarker + " " + command + "\n")
	content.WriteString(command + "() {\n")

	content.WriteString("  local RUNX_SHIM_ACTIVE=1\n")
	if len(envArgs) > 0 {
		content.WriteString("  " + utils.QuoteForSh(runxPath) + " exec " + strings.Join(envArgs, " ") + " " + utils.QuoteForSh(originalCommand) + " \"$@\"\n")
	} else {
		content.WriteString("  " + utils.QuoteForSh(runxPath) + " exec " + utils.QuoteForSh(originalCommand) + " \"$@\"\n")
	}

	content.WriteString("}\n")
	return content.String()
}

func addCommandLinuxPosix(command string, originalCommand string, envFiles []string, runxPath, shellName string) error {
	rcPath, err := utils.LinuxShellRCPath(shellName)
	if err != nil {
		return err
	}
	functionContent := buildPosixFunction(command, originalCommand, envFiles, runxPath)

	existing, readErr := os.ReadFile(rcPath)
	functionExists := false
	if readErr == nil {
		marker := "# " + utils.ShimMarker + " " + command
		for _, line := range strings.Split(string(existing), "\n") {
			if strings.TrimSpace(line) == marker {
				functionExists = true
				break
			}
		}
	}

	fmt.Printf("This will add a function to %s:\n", filepath.Base(rcPath))
	fmt.Printf("  Shell: %s\n", shellName)
	fmt.Printf("  Command: %s\n", command)
	fmt.Printf("  File: %s\n", rcPath)
	if len(envFiles) > 0 {
		fmt.Printf("  Environment files: %v\n", envFiles)
	} else {
		fmt.Println("  Environment files: (none)")
	}
	if functionExists {
		fmt.Println("  Action: Replace existing function")
	} else {
		fmt.Println("  Action: Add new function")
	}
	fmt.Println("\nFunction content:")
	fmt.Println(functionContent)
	fmt.Println()
	fmt.Print("Proceed? (y/N): ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response != "y" && response != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	if err := upsertPosixShellFunction(rcPath, command, functionContent); err != nil {
		return err
	}

	fmt.Printf("Function added to %s\n", rcPath)
	fmt.Printf("Note: Run 'source %s' or restart your shell to use the new command.\n", rcPath)
	return nil
}

func addCommandLinuxFish(command string, originalCommand string, envFiles []string, runxPath string) error {
	functionsDir, err := utils.FishFunctionsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(functionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create fish functions directory: %w", err)
	}

	functionPath := filepath.Join(functionsDir, command+".fish")
	functionContent := buildFishFunction(command, originalCommand, envFiles, runxPath)

	overwriting := false
	if _, err := os.Stat(functionPath); err == nil {
		overwriting = true
	}

	fmt.Println("This will add a fish function file:")
	fmt.Println("  Shell: fish")
	fmt.Printf("  Command: %s\n", command)
	fmt.Printf("  File: %s\n", functionPath)
	if len(envFiles) > 0 {
		fmt.Printf("  Environment files: %v\n", envFiles)
	} else {
		fmt.Println("  Environment files: (none)")
	}
	if overwriting {
		fmt.Println("  Action: Overwrite existing function file")
	} else {
		fmt.Println("  Action: Create new function file")
	}
	fmt.Println("\nFunction content:")
	fmt.Println(functionContent)
	fmt.Println()
	fmt.Print("Proceed? (y/N): ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response != "y" && response != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	if err := os.WriteFile(functionPath, []byte(functionContent), 0644); err != nil {
		return fmt.Errorf("failed to write fish function: %w", err)
	}

	fmt.Printf("Function file added: %s\n", functionPath)
	fmt.Println("Note: fish loads functions automatically from this directory.")
	return nil
}

func buildFishFunction(command string, originalCommand string, envFiles []string, runxPath string) string {
	var envArgs []string
	for _, f := range envFiles {
		envArgs = append(envArgs, "--envfile="+utils.QuoteForSh(f))
	}

	var content strings.Builder
	content.WriteString("# " + utils.ShimMarker + " " + command + "\n")
	content.WriteString("function " + command + "\n")
	content.WriteString("  set -lx RUNX_SHIM_ACTIVE 1\n")
	if len(envArgs) > 0 {
		content.WriteString("  " + utils.QuoteForSh(runxPath) + " exec " + strings.Join(envArgs, " ") + " " + utils.QuoteForSh(originalCommand) + " $argv\n")
	} else {
		content.WriteString("  " + utils.QuoteForSh(runxPath) + " exec " + utils.QuoteForSh(originalCommand) + " $argv\n")
	}
	content.WriteString("end\n")
	return content.String()
}

func upsertPosixShellFunction(rcPath, command, functionContent string) error {
	marker := "# " + utils.ShimMarker + " " + command

	// Read existing content
	var existingContent []byte
	var err error
	existingContent, err = os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read shell rc file: %w", err)
	}

	lines := strings.Split(string(existingContent), "\n")
	var newLines []string
	inFunction := false

	// Remove existing function if present
	for _, line := range lines {
		if strings.TrimSpace(line) == marker {
			inFunction = true
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

	// Ensure file ends with newline before appending
	if len(newLines) > 0 && newLines[len(newLines)-1] != "" {
		newLines = append(newLines, "")
	}

	// Add new function
	newLines = append(newLines, strings.TrimRight(functionContent, "\n"))

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(rcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write shell rc file: %w", err)
	}

	return nil
}
