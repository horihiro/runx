package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"horihiro.net/runx/commands/utils"
)

func ExecCommand(args []string) error {
	envFiles, commandArgs, err := parseExecArgs(args)
	if err != nil {
		return err
	}

	merged, resolvedEnvFiles, mergedDetails, err := utils.LoadEnvFilesDetailed(envFiles)
	if err != nil {
		return err
	}

	debugLevel := getDebugLevel()
	if debugLevel > 0 {
		printEnvMergeDebug(debugLevel, resolvedEnvFiles, mergedDetails)
	}

	// Resolve command path, avoiding shim directories on Windows
	cmdPath, err := resolveCommandPath(commandArgs[0])
	if err != nil {
		return fmt.Errorf("failed to resolve command: %w", err)
	}

	cmd := exec.Command(cmdPath, commandArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = utils.MergeWithCurrentEnv(merged)

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}

func getDebugLevel() int {
	raw := strings.TrimSpace(os.Getenv("RUNX_DEBUG"))
	if raw == "" || raw == "0" {
		return 0
	}
	if strings.EqualFold(raw, "true") {
		return 1
	}
	level, err := strconv.Atoi(raw)
	if err != nil {
		// Keep backward compatibility for non-empty non-numeric values.
		return 1
	}
	if level < 0 {
		return 0
	}
	return level
}

func printEnvMergeDebug(level int, resolvedEnvFiles []utils.ResolvedEnvFile, mergedDetails map[string]utils.MergedEnvEntry) {
	if level >= 1 {
		fmt.Fprintln(os.Stderr, "[runx][debug] resolved envfiles:")
		if len(resolvedEnvFiles) == 0 {
			fmt.Fprintln(os.Stderr, "[runx][debug]   (none)")
		} else {
			for _, f := range resolvedEnvFiles {
				if f.Found {
					fmt.Fprintf(os.Stderr, "[runx][debug]   %s: %s\n", f.Name, f.Path)
				} else {
					fmt.Fprintf(os.Stderr, "[runx][debug]   %s: (not found)\n", f.Name)
				}
			}
		}
	}

	if level >= 2 {
		fmt.Fprintln(os.Stderr, "[runx][debug] envfile search trace:")
		if len(resolvedEnvFiles) == 0 {
			fmt.Fprintln(os.Stderr, "[runx][debug]   (none)")
		} else {
			for _, f := range resolvedEnvFiles {
				fmt.Fprintf(os.Stderr, "[runx][debug]   %s:\n", f.Name)
				if len(f.Trace) == 0 {
					fmt.Fprintln(os.Stderr, "[runx][debug]     (no probe)")
					continue
				}
				for _, p := range f.Trace {
					fmt.Fprintf(os.Stderr, "[runx][debug]     - %s\n", p)
				}
			}
		}
	}

	if level >= 1 {
		fmt.Fprintln(os.Stderr, "[runx][debug] merged environment entries:")
		if len(mergedDetails) == 0 {
			fmt.Fprintln(os.Stderr, "[runx][debug]   (none)")
			return
		}

		for _, key := range utils.SortedMergedEnvKeys(mergedDetails) {
			entry := mergedDetails[key]
			fmt.Fprintf(os.Stderr, "[runx][debug]   - %s=%s (from: %s)\n", key, entry.Value, entry.LastSource)
		}
	}
}

func parseExecArgs(args []string) ([]string, []string, error) {
	var envFiles []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if strings.HasPrefix(arg, "--envfile=") {
			name, err := utils.NormalizeEnvFileName(strings.TrimSpace(strings.TrimPrefix(arg, "--envfile=")))
			if err != nil {
				return nil, nil, err
			}
			envFiles = append(envFiles, name)
			i++
			continue
		}
		if arg == "--envfile" {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--envfile requires a value")
			}
			name, err := utils.NormalizeEnvFileName(strings.TrimSpace(args[i+1]))
			if err != nil {
				return nil, nil, err
			}
			envFiles = append(envFiles, name)
			i += 2
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return nil, nil, fmt.Errorf("unknown option for exec: %s", arg)
		}
		break
	}

	if i >= len(args) {
		return nil, nil, fmt.Errorf("usage: runx exec [--envfile=FILE ...] COMMAND [ARGS...]")
	}

	commandArgs := args[i:]
	return envFiles, commandArgs, nil
}

func resolveCommandPath(command string) (string, error) {
	// If called from shim, need to find real command
	shimActive := os.Getenv("RUNX_SHIM_ACTIVE")
	shimDir := os.Getenv("RUNX_SHIM_DIR")
	shimDirs := collectShimDirs(shimDir, os.Getenv("RUNX_SHIM_DIRS"))

	if runtime.GOOS == "windows" && len(shimDirs) > 0 {
		// On Windows, exclude all shim directories (user shim and machine shim)
		return findCommandExcludingDirs(command, shimDirs)
	}

	if runtime.GOOS != "windows" && shimActive != "" {
		// On Unix, use 'command -v' to bypass shell functions
		return findCommandViaShell(command)
	}

	// Default behavior: use exec.LookPath
	path, err := exec.LookPath(command)
	if err != nil {
		return "", err
	}
	return path, nil
}

func collectShimDirs(primary string, rawList string) []string {
	seen := map[string]bool{}
	var dirs []string

	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		clean := filepath.Clean(v)
		key := strings.ToLower(clean)
		if seen[key] {
			return
		}
		seen[key] = true
		dirs = append(dirs, clean)
	}

	add(primary)
	for _, p := range filepath.SplitList(rawList) {
		add(p)
	}

	return dirs
}

func findCommandExcludingDirs(command string, excludeDirs []string) (string, error) {
	excluded := map[string]bool{}
	for _, d := range excludeDirs {
		n := strings.ToLower(filepath.Clean(d))
		excluded[n] = true
	}

	// Get PATH environment variable
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("PATH environment variable is empty")
	}

	// Search for command in each PATH directory
	pathEntries := filepath.SplitList(pathEnv)
	pathExts := []string{""}
	if runtime.GOOS == "windows" {
		// On Windows, search with common executable extensions
		pathExts = []string{".exe", ".cmd", ".bat", ".com"}
		if extEnv := os.Getenv("PATHEXT"); extEnv != "" {
			pathExts = strings.Split(strings.ToLower(extEnv), ";")
		}
	}

	for _, dir := range pathEntries {
		dir = filepath.Clean(strings.TrimSpace(dir))
		if dir == "." || dir == "" {
			continue
		}

		// Skip shim directories (both user shim and machine shim)
		if excluded[strings.ToLower(dir)] {
			continue
		}

		// Try each extension
		for _, ext := range pathExts {
			candidate := filepath.Join(dir, command+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				// Found executable, return absolute path
				absPath, err := filepath.Abs(candidate)
				if err != nil {
					return candidate, nil
				}
				return absPath, nil
			}
		}
	}

	return "", fmt.Errorf("command not found in PATH (excluding shim directories): %s", command)
}

func findCommandViaShell(command string) (string, error) {
	// Use 'command -v' to find the actual executable, bypassing shell functions
	cmd := exec.Command("sh", "-c", "command -v "+shellEscape(command))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command not found: %s", command)
	}

	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("command not found: %s", command)
	}

	return path, nil
}

func shellEscape(s string) string {
	// Simple shell escaping for command -v argument
	if strings.ContainsAny(s, " \t\n'\"\\$`!*?[](){};<>|&") {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}
	return s
}
