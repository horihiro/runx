//go:build !windows
// +build !windows

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"horihiro.net/runx/commands/utils"
)

var (
	posixEnvFileRegexp = regexp.MustCompile(`--envfile='([^']*)'`)
	posixOrigRegexp    = regexp.MustCompile(`exec(?:\s+--envfile='[^']*')*\s+'([^']+)'\s+"\$@"`)
	fishOrigRegexp     = regexp.MustCompile(`exec(?:\s+--envfile='[^']*')*\s+'([^']+)'\s+\$argv`)
)

func showCommandPlatform(command, shellOverride string) error {
	shellName, err := utils.ResolveLinuxShell(shellOverride)
	if err != nil {
		return err
	}

	if shellName == "fish" {
		return showCommandLinuxFish(command)
	}
	return showCommandLinuxPosix(command, shellName)
}

func showCommandLinuxPosix(command, shellName string) error {
	rcPath, err := utils.LinuxShellRCPath(shellName)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return fmt.Errorf("failed to read shell rc file: %w", err)
	}

	block, err := extractPosixFunctionBlock(string(content), command)
	if err != nil {
		return err
	}

	envFiles := parseEnvFilesFromExecLine(block)
	original := parseOriginalPosixCommand(block, command)
	runxPath, err := resolveSelfPath()
	if err != nil {
		return err
	}
	functionContent := buildPosixFunction(command, original, envFiles, runxPath)

	fmt.Printf("Existing runx function in %s:\n", filepath.Base(rcPath))
	fmt.Printf("  Shell: %s\n", shellName)
	fmt.Printf("  Command: %s\n", command)
	fmt.Printf("  File: %s\n", rcPath)
	if len(envFiles) > 0 {
		fmt.Printf("  Environment files: %v\n", envFiles)
	} else {
		fmt.Println("  Environment files: (none)")
	}
	fmt.Println("\nFunction content (current):")
	fmt.Println(functionContent)
	return nil
}

func showCommandLinuxFish(command string) error {
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

	envFiles := parseEnvFilesFromExecLine(string(content))
	original := parseOriginalFishCommand(string(content), command)
	runxPath, err := resolveSelfPath()
	if err != nil {
		return err
	}
	functionContent := buildFishFunction(command, original, envFiles, runxPath)

	fmt.Println("Existing runx fish function file:")
	fmt.Println("  Shell: fish")
	fmt.Printf("  Command: %s\n", command)
	fmt.Printf("  File: %s\n", functionPath)
	if len(envFiles) > 0 {
		fmt.Printf("  Environment files: %v\n", envFiles)
	} else {
		fmt.Println("  Environment files: (none)")
	}
	fmt.Println("\nFunction content (current):")
	fmt.Println(functionContent)
	return nil
}

func extractPosixFunctionBlock(rcContent, command string) (string, error) {
	marker := "# " + utils.ProxyMarker + " " + command
	lines := strings.Split(rcContent, "\n")

	start := -1
	end := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == marker {
			start = i
			continue
		}
		if start >= 0 && strings.TrimSpace(line) == "}" {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		return "", fmt.Errorf("function not found: %s", command)
	}
	return strings.Join(lines[start:end+1], "\n"), nil
}

func parseEnvFilesFromExecLine(content string) []string {
	matches := posixEnvFileRegexp.FindAllStringSubmatch(content, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			result = append(result, m[1])
		}
	}
	return result
}

func parseOriginalPosixCommand(content, fallback string) string {
	m := posixOrigRegexp.FindStringSubmatch(content)
	if len(m) >= 2 && m[1] != "" {
		return m[1]
	}
	return fallback
}

func parseOriginalFishCommand(content, fallback string) string {
	m := fishOrigRegexp.FindStringSubmatch(content)
	if len(m) >= 2 && m[1] != "" {
		return m[1]
	}
	return fallback
}
