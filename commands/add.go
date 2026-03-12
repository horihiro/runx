package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	if runtime.GOOS == "windows" {
		if shellOverride != "" {
			return fmt.Errorf("--shell is supported only on Linux")
		}
		return addCommandWindows(command, originalCommand, envFiles, runxPath)
	}

	shellName, err := utils.ResolveLinuxShell(shellOverride)
	if err != nil {
		return err
	}
	return addCommandLinux(command, originalCommand, envFiles, runxPath, shellName)
}

func addCommandWindows(command, originalCommand string, envFiles []string, runxPath string) error {
	shimDir, err := windowsShimDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return fmt.Errorf("failed to create shim directory: %w", err)
	}

	pathInEnv := isDirInPath(shimDir)
	originalPaths := findWindowsCommandPaths(originalCommand) // Get ALL paths
	originalPath := ""
	originalPathSource := "neither"
	if len(originalPaths) > 0 {
		originalPath = originalPaths[0]
		originalPathSource = determinePathSource(filepath.Dir(originalPath))
	}

	// If original command is in Machine PATH, recommend Machine shim immediately
	if originalPathSource == "machine" {
		fmt.Println()
		fmt.Println("┌────────────────────────────────────────────────────────────────┐")
		fmt.Println("│ Machine PATH Detected                                          │")
		fmt.Println("└────────────────────────────────────────────────────────────────┘")
		fmt.Println()
		fmt.Printf("The original command is in Machine PATH:\n")
		fmt.Printf("  Command: %s\n", originalCommand)
		fmt.Printf("  Location: %s\n", originalPath)
		fmt.Println()
		fmt.Println("Windows prioritizes Machine PATH over User PATH.")
		fmt.Println("Creating a User shim will not work - the Machine PATH entry will")
		fmt.Println("always take precedence.")
		fmt.Println()
		fmt.Println("Recommendation: Create a Machine shim instead.")
		fmt.Println("  • Requires administrator privileges")
		fmt.Println("  • Will be placed in: C:\\ProgramData\\runx\\shim")
		fmt.Println("  • Will be added to Machine PATH (system-wide)")
		fmt.Println()
		fmt.Print("Create Machine shim now? (y/N): ")

		var response string
		if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
			return nil
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			// Directly create Machine shim
			return handlePathElevation(shimDir, command, originalCommand, envFiles, runxPath, originalPath)
		}

		fmt.Println()
		fmt.Println("Alternative options:")
		fmt.Printf("  1. Create an alias shim: runx add %s --alias=my%s --envfile=...\n", originalCommand, originalCommand)
		fmt.Println("  2. Use runx exec directly: runx exec --envfile=... " + originalCommand)
		return nil
	}

	shimPath, content := buildShimWindows(command, originalCommand, envFiles, runxPath, originalPath, shimDir)
	precedesOriginal := true
	if pathInEnv && originalPath != "" {
		shimIdx, okShim := pathEntryIndex(shimDir)
		originalIdx, okOriginal := pathEntryIndex(filepath.Dir(originalPath))
		if okShim && okOriginal {
			precedesOriginal = shimIdx < originalIdx
		}
	}

	overwriting := false
	if info, err := os.Stat(shimPath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("shim path is a directory: %s", shimPath)
		}
		managed, mErr := utils.IsManagedShim(shimPath)
		if mErr != nil {
			return mErr
		}
		if !managed {
			return fmt.Errorf("refusing to overwrite unmanaged file: %s", shimPath)
		}
		overwriting = true
	}

	// Show what will be created and ask for confirmation
	fmt.Println("This will create a user shim:")
	fmt.Printf("  Command: %s\n", command)
	if command != originalCommand {
		fmt.Printf("  Original command: %s\n", originalCommand)
	}
	fmt.Printf("  Path: %s\n", shimPath)
	if len(envFiles) > 0 {
		fmt.Printf("  Environment files: %v\n", envFiles)
	} else {
		fmt.Println("  Environment files: (none)")
	}
	if originalPath != "" {
		fmt.Printf("  Current command path: %s\n", originalPath)
		if originalPathSource == "user" {
			fmt.Println("  Original command location: User PATH ✓")
		}
		if len(originalPaths) > 1 {
			fmt.Println("  Other matches in PATH:")
			for _, p := range originalPaths[1:] {
				fmt.Printf("    - %s\n", p)
			}
		}
	} else {
		fmt.Println("  Current command path: (not found)")
	}
	if !pathInEnv {
		fmt.Println("  Warning: shim directory is not in PATH")
		fmt.Printf("  Hint: add `%s` to User PATH environment variable\n", shimDir)
	} else if !precedesOriginal {
		fmt.Println("  Warning: shim directory appears after the original command in PATH")
		fmt.Println("  Note: on Windows, Machine PATH entries take precedence over User PATH")
		fmt.Println("  If original command is in System/Machine PATH, it will always run first")
		fmt.Print("        regardless of User PATH order. ")
		fmt.Println("Please manually reorder System PATH or check PATH via 'echo %PATH%'")
	}
	if overwriting {
		fmt.Println("  Action: Overwrite existing shim")
	} else {
		fmt.Println("  Action: Create new shim")
	}
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

	if err := os.WriteFile(shimPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write shim: %w", err)
	}

	fmt.Printf("User shim created: %s\n", shimPath)

	// On Windows, always run PATH setup/check flow.
	if runtime.GOOS == "windows" {
		return handleWindowsPathSetupWithPrediction(command, shimDir, shimPath, pathInEnv, envFiles, runxPath)
	}

	return nil
}

// handleWindowsPathSetupWithPrediction checks if shimPath is already resolvable via where,
// and only promotes to User PATH addition if needed.
func handleWindowsPathSetupWithPrediction(command, shimDir, shimPath string, pathInEnv bool, envFiles []string, runxPath string) error {
	// First, check if shim is already resolvable (either already in PATH or existing)
	fmt.Println()
	fmt.Println("Checking if shim is resolvable...")

	shimFirst, resolvedPath, err := verifyPathResolution(command, shimPath)
	if err == nil {
		if shimFirst {
			// Shim already resolves correctly!
			fmt.Printf("✓ User shim will be used when you run: %s\n", command)
			fmt.Println("Please restart your terminal for changes to take effect")
			return nil
		}

		// Found something else - either need to elevate or add to User PATH
		fmt.Printf("⚠ Currently resolves to: %s\n", resolvedPath)

		if pathInEnv {
			// Shim dir is in PATH but something else takes priority (Machine PATH)
			elevate, useAlias := promptElevatePath(shimDir, command, resolvedPath)
			if elevate {
				return handlePathElevation(shimDir, command, command, envFiles, runxPath, resolvedPath)
			}
			if useAlias {
				fmt.Println()
				fmt.Printf("You can create a shim with an alias. For example:\n")
				fmt.Printf("  runx add %s --alias=my%s --envfile=...\n", command, command)
				fmt.Println()
			}
			fmt.Println("Keeping current setup. Use 'runx exec' to run with environment:")
			fmt.Printf("  runx exec --envfile=... %s\n", command)
			return nil
		}

		// Not in PATH yet - need to add it
	}

	// Either where failed or command not found - need to add to User PATH
	if !pathInEnv {
		if !promptAddToPath(shimDir) {
			fmt.Println("Skipped adding to PATH. You can manually add it later:")
			fmt.Printf("  %s\n", shimDir)
			return nil
		}

		// Add to User PATH
		if err := addToUserPath(shimDir); err != nil {
			fmt.Printf("Warning: Failed to add to User PATH: %v\n", err)
			fmt.Println("You can manually add it to your PATH environment variable.")
			return nil
		}
		fmt.Println("✓ Added to User PATH successfully")
	}

	// Now verify again after adding to User PATH
	fmt.Println()
	fmt.Println("Verifying PATH resolution using registry PATH order...")
	shimFirst, actualPath, err := verifyPathResolution(command, shimPath)
	if err != nil {
		fmt.Printf("⚠ Could not verify PATH resolution: %v\n", err)
		fmt.Println("Please restart your terminal and test manually.")
		return nil
	}

	if shimFirst {
		fmt.Printf("✓ User shim will be used when you run: %s\n", command)
		fmt.Println("Please restart your terminal for changes to take effect")
		return nil
	}

	// Still blocked by Machine PATH even after adding to User PATH
	fmt.Printf("⚠ Warning: '%s' still resolves to: %s\n", command, actualPath)

	// Ask user if they want to elevate to Machine PATH
	elevate, useAlias := promptElevatePath(shimDir, command, actualPath)

	if elevate {
		return handlePathElevation(shimDir, command, command, envFiles, runxPath, actualPath)
	}

	if useAlias {
		fmt.Println()
		fmt.Printf("You can create a shim with an alias. For example:\n")
		fmt.Printf("  runx add %s --alias=my%s --envfile=...\n", command, command)
		fmt.Println()
	}

	fmt.Println("Keeping current setup. Use 'runx exec' to run with environment:")
	fmt.Printf("  runx exec --envfile=... %s\n", command)
	return nil
}

func addCommandLinux(command string, originalCommand string, envFiles []string, runxPath, shellName string) error {
	if shellName == "fish" {
		return addCommandLinuxFish(command, originalCommand, envFiles, runxPath)
	}
	return addCommandLinuxPosix(command, originalCommand, envFiles, runxPath, shellName)
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

func buildShimWindows(command, originalCommand string, envFiles []string, runxPath, originalPath, shimDir string) (string, string) {
	shimPath := filepath.Join(shimDir, command+".cmd")

	var args []string
	for _, f := range envFiles {
		args = append(args, "--envfile="+utils.QuoteForCmd(f))
	}
	args = append(args, utils.QuoteForCmd(originalCommand), "%*")

	var content strings.Builder
	content.WriteString("@echo off\r\n")
	content.WriteString("REM " + utils.ShimMarker + " (user shim)\r\n")
	content.WriteString("setlocal\r\n")
	content.WriteString("set \"RUNX_SHIM_DIR=%~dp0\"\r\n")
	content.WriteString("if defined RUNX_SHIM_DIRS (set \"RUNX_SHIM_DIRS=%RUNX_SHIM_DIR%;%RUNX_SHIM_DIRS%\") else (set \"RUNX_SHIM_DIRS=%RUNX_SHIM_DIR%\")\r\n")
	content.WriteString("if exist " + utils.QuoteForCmd(runxPath) + " (\r\n")
	content.WriteString("  " + utils.QuoteForCmd(runxPath) + " exec " + strings.Join(args, " ") + "\r\n")
	content.WriteString("  set \"RUNX_EXIT_CODE=%ERRORLEVEL%\"\r\n")
	content.WriteString("  endlocal & exit /b %RUNX_EXIT_CODE%\r\n")
	content.WriteString(")\r\n")

	if strings.TrimSpace(originalPath) != "" {
		content.WriteString("if exist " + utils.QuoteForCmd(originalPath) + " (\r\n")
		content.WriteString("  " + utils.QuoteForCmd(originalPath) + " %*\r\n")
		content.WriteString("  set \"RUNX_EXIT_CODE=%ERRORLEVEL%\"\r\n")
		content.WriteString("  endlocal & exit /b %RUNX_EXIT_CODE%\r\n")
		content.WriteString(")\r\n")
	}

	content.WriteString("echo command not found: " + originalCommand + " 1>&2\r\n")
	content.WriteString("endlocal & exit /b 9009\r\n")
	return shimPath, content.String()
}

func windowsShimDir() (string, error) {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve LOCALAPPDATA and home directory: %w", err)
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(localAppData, "runx", "bin"), nil
}

func isDirInPath(target string) bool {
	_, ok := pathEntryIndex(target)
	return ok
}

func pathEntryIndex(target string) (int, bool) {
	target = strings.ToLower(filepath.Clean(target))
	entries := filepath.SplitList(os.Getenv("PATH"))
	for i, entry := range entries {
		if strings.ToLower(filepath.Clean(entry)) == target {
			return i, true
		}
	}
	return -1, false
}

func findWindowsCommandPaths(command string) []string {
	out, err := exec.Command("where", command).CombinedOutput()
	if err != nil {
		return []string{}
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths
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
		functionExists = strings.Contains(string(existing), "# "+utils.ShimMarker+" "+command)
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
		if strings.Contains(line, marker) {
			inFunction = true
			continue
		}
		if inFunction && line == "}" {
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
