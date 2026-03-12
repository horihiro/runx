package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"horihiro.net/runx/commands/utils"
)

func AddCommand(args []string) error {
	command, originalCommand, envFiles, shellOverride, err := parseAddArgs(args)
	if err != nil {
		return err
	}

	runxPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve runx path: %w", err)
	}
	runxPath, err = filepath.EvalSymlinks(runxPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink for runx path: %w", err)
	}

	runxBase := filepath.Base(runxPath)

	if command == strings.TrimSuffix(runxBase, filepath.Ext(runxBase)) {
		return fmt.Errorf("cannot create shim for runx itself")
	}

	return addCommandPlatform(command, originalCommand, envFiles, shellOverride, runxPath)
}

func parseAddArgs(args []string) (string, string, []string, string, error) {
	var envFiles []string
	aliasCommand := ""
	originalCommand := ""
	shellOverride := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--alias=") {
			aliasCommand = strings.TrimSpace(strings.TrimPrefix(arg, "--alias="))
			if aliasCommand == "" {
				return "", "", nil, "", fmt.Errorf("--alias requires a value")
			}
			continue
		}
		if arg == "--alias" {
			if i+1 >= len(args) {
				return "", "", nil, "", fmt.Errorf("--alias requires a value")
			}
			aliasCommand = strings.TrimSpace(args[i+1])
			if aliasCommand == "" {
				return "", "", nil, "", fmt.Errorf("--alias requires a value")
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "--shell=") {
			shellOverride = strings.TrimSpace(strings.TrimPrefix(arg, "--shell="))
			if shellOverride == "" {
				return "", "", nil, "", fmt.Errorf("--shell requires a value")
			}
			continue
		}
		if arg == "--shell" {
			if i+1 >= len(args) {
				return "", "", nil, "", fmt.Errorf("--shell requires a value")
			}
			shellOverride = strings.TrimSpace(args[i+1])
			if shellOverride == "" {
				return "", "", nil, "", fmt.Errorf("--shell requires a value")
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "--envfile=") {
			name, err := utils.NormalizeEnvFileName(strings.TrimSpace(strings.TrimPrefix(arg, "--envfile=")))
			if err != nil {
				return "", "", nil, "", err
			}
			envFiles = append(envFiles, name)
			continue
		}
		if arg == "--envfile" {
			if i+1 >= len(args) {
				return "", "", nil, "", fmt.Errorf("--envfile requires a value")
			}
			name, err := utils.NormalizeEnvFileName(strings.TrimSpace(args[i+1]))
			if err != nil {
				return "", "", nil, "", err
			}
			envFiles = append(envFiles, name)
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return "", "", nil, "", fmt.Errorf("unknown option for add: %s", arg)
		}
		if originalCommand != "" {
			return "", "", nil, "", fmt.Errorf("usage: runx add ORIGINAL_COMMAND [--alias=SHIM_NAME] [--envfile=FILE ...] [--shell=bash|zsh|fish]")
		}
		originalCommand = strings.TrimSpace(arg)
	}

	if originalCommand == "" {
		return "", "", nil, "", fmt.Errorf("usage: runx add ORIGINAL_COMMAND [--alias=SHIM_NAME] [--envfile=FILE ...] [--shell=bash|zsh|fish]")
	}

	if aliasCommand == "" {
		aliasCommand = originalCommand
	}

	return aliasCommand, originalCommand, envFiles, shellOverride, nil
}
