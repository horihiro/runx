//go:build windows
// +build windows

package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
	"horihiro.net/runx/commands/utils"
)

func addCommandPlatform(command, originalCommand string, envFiles []string, shellOverride, runxPath string) error {
	if shellOverride != "" {
		return fmt.Errorf("--shell is supported only on Linux")
	}
	return addCommandWindows(command, originalCommand, envFiles, runxPath)
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
	originalPaths := findWindowsCommandPaths(originalCommand) // Get ALL paths for original command
	originalPath := ""
	originalPathSource := "neither"
	for _, p := range originalPaths {
		managed, mErr := utils.IsManagedShim(p)
		if mErr != nil || managed {
			continue // skip runx-managed shims; the real command is further down
		}
		originalPath = p
		originalPathSource = determinePathSource(filepath.Dir(p))
		break
	}

	// Check if the shim name (command) conflicts with an existing non-shim entry in Machine PATH.
	// Skipping runx-managed shims avoids false positives when a same-named shim already exists.
	// When no alias is used command == originalCommand, so reuse the lookup above.
	// When an alias is used, check the alias name separately.
	shimConflictPath := ""
	shimConflictSource := "neither"
	if command == originalCommand {
		shimConflictPath = originalPath
		shimConflictSource = originalPathSource
	} else {
		aliasPaths := findWindowsCommandPaths(command)
		for _, p := range aliasPaths {
			managed, mErr := utils.IsManagedShim(p)
			if mErr != nil || managed {
				continue
			}
			shimConflictPath = p
			shimConflictSource = determinePathSource(filepath.Dir(shimConflictPath))
			break
		}
	}

	// If the shim name is already in Machine PATH, User shim will never take precedence.
	if shimConflictSource == "machine" {
		fmt.Println()
		fmt.Println("┌────────────────────────────────────────────────────────────────┐")
		fmt.Println("│ Machine PATH Detected                                          │")
		fmt.Println("└────────────────────────────────────────────────────────────────┘")
		fmt.Println()
		fmt.Printf("The command '%s' is already in Machine PATH:\n", command)
		fmt.Printf("  Location: %s\n", shimConflictPath)
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
			return handlePathElevation(shimDir, command, originalCommand, envFiles, runxPath, shimConflictPath)
		}

		fmt.Println()
		fmt.Println("Alternative options:")
		if command == originalCommand {
			fmt.Printf("  1. Create an alias shim: runx add %s --alias=my%s --envfile=...\n", originalCommand, originalCommand)
			fmt.Println("  2. Use runx exec directly: runx exec --envfile=... " + originalCommand)
		} else {
			fmt.Println("  1. Choose a different alias name that is not in Machine PATH")
			fmt.Println("  2. Use runx exec directly: runx exec --envfile=... " + originalCommand)
		}
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
	return handleWindowsPathSetupWithPrediction(command, shimDir, shimPath, pathInEnv, envFiles, runxPath)
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

const (
	hwndBroadcast   = 0xffff
	wmSettingChange = 0x001A
	smtoAbortIfHung = 0x0002
	notifyTimeoutMS = 5000
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
)

// getUserPath reads the User PATH from Windows registry
func getUserPath() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	path, _, err := k.GetStringValue("Path")
	if err != nil {
		// Path might not exist yet
		return "", nil
	}
	return path, nil
}

// setUserPath writes the User PATH to Windows registry
func setUserPath(path string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key for writing: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("Path", path); err != nil {
		return fmt.Errorf("failed to set Path value: %w", err)
	}
	if err := notifyEnvironmentChange(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: PATH updated but failed to notify Windows shell: %v\n", err)
	}
	return nil
}

// addToUserPath adds a directory to the beginning of User PATH if not already present
func addToUserPath(dir string) error {
	currentPath, err := getUserPath()
	if err != nil {
		return err
	}

	// Check if already in path
	entries := strings.Split(currentPath, ";")
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(entry), dir) {
			// Already exists
			return nil
		}
	}

	// Add to the beginning
	var newPath string
	if currentPath == "" {
		newPath = dir
	} else {
		newPath = dir + ";" + currentPath
	}

	return setUserPath(newPath)
}

// promptAddToPath asks user if they want to add directory to User PATH
func promptAddToPath(dir string) bool {
	fmt.Println()
	fmt.Println("┌────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ PATH Configuration Required                                    │")
	fmt.Println("└────────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Printf("The shim directory is not in your User PATH:\n  %s\n\n", dir)
	fmt.Println("Would you like runx to add it to your User PATH now?")
	fmt.Println()
	fmt.Println("This will:")
	fmt.Println("  • Modify HKEY_CURRENT_USER\\Environment\\Path registry key")
	fmt.Printf("  • Add '%s' to the beginning of User PATH\n", dir)
	fmt.Println("  • Require restarting your terminal for changes to take effect")
	fmt.Println()
	fmt.Println("Note: Windows prioritizes Machine PATH over User PATH.")
	fmt.Println("      If the command exists in System32 or other Machine PATH locations,")
	fmt.Println("      those will still take precedence.")
	fmt.Println()
	fmt.Print("Add to User PATH? (y/N): ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// getMachinePath reads the Machine PATH from Windows registry
func getMachinePath() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `System\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	path, _, err := k.GetStringValue("Path")
	if err != nil {
		return "", fmt.Errorf("failed to read Path value: %w", err)
	}
	return path, nil
}

// setMachinePath writes the Machine PATH to Windows registry (requires admin)
func setMachinePath(path string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `System\CurrentControlSet\Control\Session Manager\Environment`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key for writing: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("Path", path); err != nil {
		return fmt.Errorf("failed to set Path value: %w", err)
	}
	if err := notifyEnvironmentChange(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Machine PATH updated but failed to notify Windows shell: %v\n", err)
	}
	return nil
}

func notifyEnvironmentChange() error {
	section := syscall.StringToUTF16Ptr("Environment")
	var result uintptr
	r1, _, callErr := procSendMessageTimeoutW.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(section)),
		smtoAbortIfHung,
		notifyTimeoutMS,
		uintptr(unsafe.Pointer(&result)),
	)
	if r1 == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("SendMessageTimeoutW failed")
	}
	return nil
}

// removeFromUserPath removes a directory from User PATH
func removeFromUserPath(dir string) error {
	currentPath, err := getUserPath()
	if err != nil {
		return err
	}

	entries := strings.Split(currentPath, ";")
	var newEntries []string
	for _, entry := range entries {
		if !strings.EqualFold(strings.TrimSpace(entry), dir) {
			newEntries = append(newEntries, entry)
		}
	}

	newPath := strings.Join(newEntries, ";")
	return setUserPath(newPath)
}

// addToMachinePath adds a directory to the beginning of Machine PATH (requires admin)
func addToMachinePath(dir string) error {
	currentPath, err := getMachinePath()
	if err != nil {
		return err
	}

	// Check if already in path
	entries := strings.Split(currentPath, ";")
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(entry), dir) {
			// Already exists
			return nil
		}
	}

	// Add to the beginning
	var newPath string
	if currentPath == "" {
		newPath = dir
	} else {
		newPath = dir + ";" + currentPath
	}

	return setMachinePath(newPath)
}

// determinePathSource checks if a directory is in Machine PATH, User PATH, or neither.
// Returns "machine", "user", or "neither".
func determinePathSource(dir string) string {
	dir = strings.ToLower(filepath.Clean(dir))

	// Check Machine PATH first (takes precedence)
	if machinePath, err := getMachinePath(); err == nil {
		entries := strings.Split(machinePath, ";")
		for _, entry := range entries {
			if strings.EqualFold(strings.TrimSpace(entry), dir) {
				return "machine"
			}
		}
	}

	// Check User PATH
	if userPath, err := getUserPath(); err == nil {
		entries := strings.Split(userPath, ";")
		for _, entry := range entries {
			if strings.EqualFold(strings.TrimSpace(entry), dir) {
				return "user"
			}
		}
	}

	return "neither"
}

func machineShimDir() (string, error) {
	programData := strings.TrimSpace(os.Getenv("ProgramData"))
	if programData == "" {
		programData = filepath.Join(`C:\`, "ProgramData")
	}
	return filepath.Join(programData, "runx", "shim"), nil
}

func machineShimPath(command string) (string, error) {
	dir, err := machineShimDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, command+".cmd"), nil
}

func buildMachineShim(originalCommand string, envFiles []string, runxPath, originalPath string) string {
	var content strings.Builder
	var runxArgs []string
	for _, f := range envFiles {
		runxArgs = append(runxArgs, "--envfile="+utils.QuoteForCmd(f))
	}
	runxArgs = append(runxArgs, utils.QuoteForCmd(originalCommand), "%*")

	content.WriteString("@echo off\r\n")
	content.WriteString("REM generated by runx add (machine shim)\r\n")
	content.WriteString("setlocal\r\n")
	content.WriteString("set \"RUNX_SHIM_DIR=%~dp0\"\r\n")
	content.WriteString("if defined RUNX_SHIM_DIRS (set \"RUNX_SHIM_DIRS=%RUNX_SHIM_DIR%;%RUNX_SHIM_DIRS%\") else (set \"RUNX_SHIM_DIRS=%RUNX_SHIM_DIR%\")\r\n")
	content.WriteString("if exist " + utils.QuoteForCmd(runxPath) + " (\r\n")
	content.WriteString("  " + utils.QuoteForCmd(runxPath) + " exec " + strings.Join(runxArgs, " ") + "\r\n")
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
	return content.String()
}

func isAccessDeniedError(err error) bool {
	if err == nil {
		return false
	}

	var errno syscall.Errno
	if errors.As(err, &errno) && errno == syscall.ERROR_ACCESS_DENIED {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") || strings.Contains(msg, "access denied")
}

func effectivePathFromRegistry() (string, error) {
	machinePath, err := getMachinePath()
	if err != nil {
		return "", err
	}
	userPath, err := getUserPath()
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(machinePath) == "" {
		return userPath, nil
	}
	if strings.TrimSpace(userPath) == "" {
		return machinePath, nil
	}
	return machinePath + ";" + userPath, nil
}

// verifyPathResolution checks if shim is resolved first based on registry PATH order.
// It does not rely on the current process environment, which may be stale.
func verifyPathResolution(command, shimPath string) (shimFirst bool, actualPath string, err error) {
	effectivePath, err := effectivePathFromRegistry()
	if err != nil {
		return false, "", fmt.Errorf("failed to read effective PATH from registry: %w", err)
	}

	cmd := exec.Command("where", command)
	env := os.Environ()
	pathSet := false
	for i, kv := range env {
		if strings.HasPrefix(strings.ToUpper(kv), "PATH=") {
			env[i] = "PATH=" + effectivePath
			pathSet = true
			break
		}
	}
	if !pathSet {
		env = append(env, "PATH="+effectivePath)
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("failed to run 'where %s': %w", command, err)
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// First match is what will be executed
		if strings.EqualFold(filepath.Clean(line), filepath.Clean(shimPath)) {
			return true, line, nil
		}
		return false, line, nil
	}

	return false, "", fmt.Errorf("command not found")
}

// promptElevatePath asks user if they want to elevate to Machine PATH
func promptElevatePath(dir, command, blockedBy string) (elevate bool, useAlias bool) {
	fmt.Println()
	fmt.Println("┌────────────────────────────────────────────────────────────────┐")
	fmt.Println("│ PATH Priority Issue Detected                                   │")
	fmt.Println("└────────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Printf("The user shim was added to User PATH, but won't be used because:\n")
	fmt.Printf("  Original command: %s\n", blockedBy)
	fmt.Printf("  User shim location: %s\\%s.cmd\n", dir, command)
	fmt.Println()
	fmt.Println("This happens because Windows prioritizes Machine PATH over User PATH.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  1. Elevate to Machine PATH (requires administrator privileges)")
	fmt.Println("     - Remove from User PATH")
	fmt.Println("     - Add machine shim to Machine PATH (system-wide)")
	fmt.Println("     - Machine shim will take precedence")
	fmt.Println()
	fmt.Println("  2. Use an alias instead")
	fmt.Printf("     - Keep user shim in User PATH as-is\n")
	fmt.Printf("     - Create shim with a different name (e.g., 'my%s')\n", command)
	fmt.Println()
	fmt.Println("  3. Keep current setup (shim won't work as expected)")
	fmt.Println()
	fmt.Print("Choose option (1/2/3): ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
		return false, false
	}

	response = strings.TrimSpace(response)
	switch response {
	case "1":
		return true, false
	case "2":
		return false, true
	default:
		return false, false
	}
}

// handlePathElevation handles the process of elevating from User PATH to Machine PATH
func handlePathElevation(dir, command, originalCommand string, envFiles []string, runxPath, originalPath string) error {
	shimDir, err := machineShimDir()
	if err != nil {
		return fmt.Errorf("failed to resolve machine shim directory: %w", err)
	}
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return fmt.Errorf("failed to create machine shim directory: %w", err)
	}

	shimPath, err := machineShimPath(originalCommand)
	if err != nil {
		return fmt.Errorf("failed to resolve machine shim path: %w", err)
	}
	shimContent := buildMachineShim(originalCommand, envFiles, runxPath, originalPath)
	if err := os.WriteFile(shimPath, []byte(shimContent), 0755); err != nil {
		return fmt.Errorf("failed to write machine shim: %w", err)
	}
	fmt.Printf("✓ Created machine shim: %s\n", shimPath)

	// Add shared shim directory to Machine PATH first so we do not break existing setup on failure.
	if err := addToMachinePath(shimDir); err != nil {
		if isAccessDeniedError(err) {
			fmt.Println()
			fmt.Println("❌ Administrator privileges required to modify Machine PATH")
			fmt.Println()
			fmt.Println("Please run this command in an elevated terminal (Run as Administrator):")
			fmt.Printf("  runx add %s --alias=%s\n", originalCommand, command)
			fmt.Println()
			fmt.Println("Note: 'sudo cmd.exe' does not grant a Windows administrator token.")
			fmt.Println("      Start Command Prompt/PowerShell with 'Run as administrator'.")
			fmt.Println("Or choose option 2 to use an alias instead.")
			return fmt.Errorf("administrator privileges required: %w", err)
		}
		return fmt.Errorf("failed to add machine shim directory to Machine PATH: %w", err)
	}
	fmt.Printf("✓ Added to Machine PATH (system-wide): %s\n", shimDir)

	// Remove the old user shim file; machine shim now handles the command.
	userShimPath := filepath.Join(dir, command+".cmd")
	if err := os.Remove(userShimPath); err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("⚠ Warning: failed to remove old user shim: %s (%v)\n", userShimPath, err)
		}
	} else {
		fmt.Printf("✓ Removed old user shim: %s\n", userShimPath)
	}

	// Remove user-local shim directory from User PATH after successful Machine PATH update.
	if err := removeFromUserPath(dir); err != nil {
		return fmt.Errorf("failed to remove from User PATH: %w", err)
	}
	fmt.Println("✓ Removed from User PATH")

	// Verify resolution again against machine shim path.
	shimFirst, actualPath, err := verifyPathResolution(command, shimPath)
	if err != nil {
		fmt.Printf("⚠ Could not verify PATH resolution: %v\n", err)
	} else if shimFirst {
		fmt.Println("✓ Machine shim will now be used when you run:", command)
	} else {
		fmt.Printf("⚠ Warning: '%s' still resolves to: %s\n", command, actualPath)
		fmt.Println("  You may need to restart your terminal or reorder Machine PATH manually")
	}

	fmt.Println()
	fmt.Println("Please restart your terminal for changes to take effect")
	return nil
}
