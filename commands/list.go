package commands

import (
	"fmt"
	"strings"
)

func ListCommand(args []string) error {
	shellOverride, err := parseListArgs(args)
	if err != nil {
		return err
	}

	proxies, err := listProxiesPlatform(shellOverride)
	if err != nil {
		return err
	}

	if len(proxies) == 0 {
		fmt.Println("No runx proxies found.")
		return nil
	}

	for _, name := range proxies {
		fmt.Println(name)
	}
	return nil
}

func parseListArgs(args []string) (string, error) {
	shellOverride := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--shell=") {
			shellOverride = strings.TrimSpace(strings.TrimPrefix(arg, "--shell="))
			if shellOverride == "" {
				return "", fmt.Errorf("--shell requires a value")
			}
			continue
		}
		if arg == "--shell" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("--shell requires a value")
			}
			shellOverride = strings.TrimSpace(args[i+1])
			if shellOverride == "" {
				return "", fmt.Errorf("--shell requires a value")
			}
			i++
			continue
		}
		return "", fmt.Errorf("usage: runx list [--shell=bash|zsh|fish]")
	}
	return shellOverride, nil
}
