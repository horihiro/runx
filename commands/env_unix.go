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

func resolveRegisteredEnvFiles(command, shellOverride string) ([]string, error) {
	shellName, err := utils.ResolveLinuxShell(shellOverride)
	if err != nil {
		return nil, err
	}

	if shellName == "fish" {
		return resolveRegisteredEnvFilesFish(command)
	}
	return resolveRegisteredEnvFilesPosix(command, shellName)
}

func resolveRegisteredEnvFilesPosix(command, shellName string) ([]string, error) {
	rcPath, err := utils.LinuxShellRCPath(shellName)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read shell rc file: %w", err)
	}

	block, err := extractPosixFunctionBlock(string(content), command)
	if err != nil {
		return nil, err
	}

	return parseEnvFilesFromExecLine(block), nil
}

func resolveRegisteredEnvFilesFish(command string) ([]string, error) {
	functionsDir, err := utils.FishFunctionsDir()
	if err != nil {
		return nil, err
	}
	functionPath := filepath.Join(functionsDir, command+".fish")

	content, err := os.ReadFile(functionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("function file not found: %s", functionPath)
		}
		return nil, fmt.Errorf("failed to read fish function file: %w", err)
	}

	marker := "# " + utils.ProxyMarker + " " + command
	if !strings.Contains(string(content), marker) {
		return nil, fmt.Errorf("file is not a runx-generated fish function: %s", functionPath)
	}

	return parseEnvFilesFromExecLine(string(content)), nil
}
