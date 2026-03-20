package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ShowCommand(args []string) error {
	command, shellOverride, err := parseShowArgs(args)
	if err != nil {
		return err
	}

	return showCommandPlatform(command, shellOverride)
}

func parseShowArgs(args []string) (string, string, error) {
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
			return "", "", fmt.Errorf("unknown option for show: %s", arg)
		}
		if command != "" {
			return "", "", fmt.Errorf("usage: runx show COMMAND_OR_ALIAS [--shell=bash|zsh|fish]")
		}
		command = strings.TrimSpace(arg)
	}

	if command == "" {
		return "", "", fmt.Errorf("usage: runx show COMMAND_OR_ALIAS [--shell=bash|zsh|fish]")
	}
	return command, shellOverride, nil
}

func resolveSelfPath() (string, error) {
	runxPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve runx path: %w", err)
	}
	runxPath, err = filepath.EvalSymlinks(runxPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlink for runx path: %w", err)
	}
	return runxPath, nil
}
