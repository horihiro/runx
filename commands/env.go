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
	envFiles, err := parseEnvLikeArgs(commandName, args)
	if err != nil {
		return err
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

func parseEnvLikeArgs(commandName string, args []string) ([]string, error) {
	var envFiles []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--envfile=") {
			name, err := utils.NormalizeEnvFileName(strings.TrimSpace(strings.TrimPrefix(arg, "--envfile=")))
			if err != nil {
				return nil, err
			}
			envFiles = append(envFiles, name)
			continue
		}
		if arg == "--envfile" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--envfile requires a value")
			}
			name, err := utils.NormalizeEnvFileName(strings.TrimSpace(args[i+1]))
			if err != nil {
				return nil, err
			}
			envFiles = append(envFiles, name)
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf("unknown option for %s: %s", commandName, arg)
		}
		return nil, fmt.Errorf("usage: runx %s [--envfile=FILE ...]", commandName)
	}

	return envFiles, nil
}
