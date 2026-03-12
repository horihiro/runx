package commands

import (
	"fmt"
	"strings"
)

func RemoveCommand(args []string) error {
	command, shellOverride, err := parseRemoveArgs(args)
	if err != nil {
		return err
	}

	return removeCommandPlatform(command, shellOverride)
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
