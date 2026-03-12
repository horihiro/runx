//go:build windows
// +build windows

package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"horihiro.net/runx/commands/utils"
)

func removeCommandPlatform(command, shellOverride string) error {
	if shellOverride != "" {
		return fmt.Errorf("--shell is supported only on Linux")
	}
	return removeCommandWindows(command)
}

func removeCommandWindows(command string) error {
	shimDir, err := windowsShimDir()
	if err != nil {
		return err
	}
	shimPath := filepath.Join(shimDir, command+".cmd")

	managed, err := utils.IsManagedShim(shimPath)
	if err == nil {
		if !managed {
			return fmt.Errorf("file is not a runx-generated shim: %s", shimPath)
		}
		if err := os.Remove(shimPath); err != nil {
			return fmt.Errorf("failed to remove shim: %w", err)
		}
		fmt.Printf("Shim removed: %s\n", shimPath)
		if err := cleanupUserShimDirPathIfEmpty(shimDir); err != nil {
			return err
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	// Fallback for shims created by older versions near runx executable.
	runxPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve runx path: %w", err)
	}
	runxPath, err = filepath.EvalSymlinks(runxPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink for runx path: %w", err)
	}
	runxDir := filepath.Dir(runxPath)
	shimPath = filepath.Join(runxDir, command+".cmd")
	managed, err = utils.IsManagedShim(shimPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Also check machine shim directory.
			if machineDir, mErr := machineShimDir(); mErr == nil {
				machineShim := filepath.Join(machineDir, command+".cmd")
				if mManaged, mErr := utils.IsManagedShim(machineShim); mErr == nil {
					if !mManaged {
						return fmt.Errorf("file is not a runx-generated shim: %s", machineShim)
					}
					if err := os.Remove(machineShim); err != nil {
						return fmt.Errorf("failed to remove machine shim (try running as Administrator): %w", err)
					}
					fmt.Printf("Machine shim removed: %s\n", machineShim)
					if err := cleanupMachineShimDirPathIfEmpty(machineDir); err != nil {
						return err
					}
					return nil
				}
			}
			return fmt.Errorf("shim not found: %s", filepath.Join(shimDir, command+".cmd"))
		}
		return err
	}
	if !managed {
		return fmt.Errorf("file is not a runx-generated shim: %s", shimPath)
	}

	if err := os.Remove(shimPath); err != nil {
		return fmt.Errorf("failed to remove shim: %w", err)
	}

	fmt.Printf("Shim removed: %s\n", shimPath)
	if err := cleanupUserShimDirPathIfEmpty(shimDir); err != nil {
		return err
	}
	return nil
}
