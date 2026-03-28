package commands

import (
	"fmt"
	"os"
	"strings"

	"horihiro.net/runx/commands/utils"
)

func EnvCommand(args []string) error {
	return runEnvLikeCommand("env", args)
}

func runEnvLikeCommand(commandName string, args []string) error {
	registeredCommand, shellOverride, envFiles, err := parseEnvLikeArgs(commandName, args)
	if err != nil {
		return err
	}

	if registeredCommand != "" {
		registeredEnvFiles, err := resolveRegisteredEnvFiles(registeredCommand, shellOverride)
		if err != nil {
			return err
		}
		// Apply envfiles from registered command first, then explicit --envfile values.
		envFiles = append(registeredEnvFiles, envFiles...)
	}

	_, resolvedEnvFiles, mergedDetails, err := utils.LoadEnvFilesDetailed(envFiles)
	if err != nil {
		return err
	}

	for _, f := range resolvedEnvFiles {
		if !f.Found {
			fmt.Fprintf(os.Stderr, "[runx][env] envfile not found: %s\n", f.Name)
		}
	}

	for _, key := range utils.SortedMergedEnvKeys(mergedDetails) {
		entry := mergedDetails[key]
		fmt.Printf("%s=%s\n", key, entry.Value)
	}

	if len(mergedDetails) == 0 {
		fmt.Println("(none)")
	}

	return nil
}

func parseEnvLikeArgs(commandName string, args []string) (string, string, []string, error) {
	var envFiles []string
	registeredCommand := ""
	shellOverride := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--command=") {
			if registeredCommand != "" {
				return "", "", nil, fmt.Errorf("--command can be specified only once")
			}
			registeredCommand = strings.TrimSpace(strings.TrimPrefix(arg, "--command="))
			if registeredCommand == "" {
				return "", "", nil, fmt.Errorf("--command requires a value")
			}
			continue
		}
		if arg == "--command" {
			if registeredCommand != "" {
				return "", "", nil, fmt.Errorf("--command can be specified only once")
			}
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("--command requires a value")
			}
			registeredCommand = strings.TrimSpace(args[i+1])
			if registeredCommand == "" {
				return "", "", nil, fmt.Errorf("--command requires a value")
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "--shell=") {
			shellOverride = strings.TrimSpace(strings.TrimPrefix(arg, "--shell="))
			if shellOverride == "" {
				return "", "", nil, fmt.Errorf("--shell requires a value")
			}
			continue
		}
		if arg == "--shell" {
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("--shell requires a value")
			}
			shellOverride = strings.TrimSpace(args[i+1])
			if shellOverride == "" {
				return "", "", nil, fmt.Errorf("--shell requires a value")
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "--envfile=") {
			name, err := utils.NormalizeEnvFileName(strings.TrimSpace(strings.TrimPrefix(arg, "--envfile=")))
			if err != nil {
				return "", "", nil, err
			}
			envFiles = append(envFiles, name)
			continue
		}
		if arg == "--envfile" {
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("--envfile requires a value")
			}
			name, err := utils.NormalizeEnvFileName(strings.TrimSpace(args[i+1]))
			if err != nil {
				return "", "", nil, err
			}
			envFiles = append(envFiles, name)
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return "", "", nil, fmt.Errorf("unknown option for %s: %s", commandName, arg)
		}
		if registeredCommand != "" {
			return "", "", nil, fmt.Errorf("command can be specified either positionally or via --command, not both")
		}
		registeredCommand = strings.TrimSpace(arg)
		if registeredCommand == "" {
			return "", "", nil, fmt.Errorf("usage: runx %s [COMMAND_OR_ALIAS] [--envfile=FILE ...] [--shell=bash|zsh|fish]", commandName)
		}
	}

	return registeredCommand, shellOverride, envFiles, nil
}
