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
	proxyDir, err := windowsProxyDir()
	if err != nil {
		return err
	}
	proxyPath := filepath.Join(proxyDir, command+".cmd")

	managed, err := utils.IsManagedProxy(proxyPath)
	if err == nil {
		if !managed {
			return fmt.Errorf("file is not a runx-generated proxy: %s", proxyPath)
		}
		if err := os.Remove(proxyPath); err != nil {
			return fmt.Errorf("failed to remove proxy: %w", err)
		}
		fmt.Printf("Proxy removed: %s\n", proxyPath)
		if err := cleanupUserProxyDirPathIfEmpty(proxyDir); err != nil {
			return err
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	// Fallback for proxies created by older versions near runx executable.
	runxPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve runx path: %w", err)
	}
	runxPath, err = filepath.EvalSymlinks(runxPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink for runx path: %w", err)
	}
	runxDir := filepath.Dir(runxPath)
	proxyPath = filepath.Join(runxDir, command+".cmd")
	managed, err = utils.IsManagedProxy(proxyPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Also check machine proxy directory.
			if machineDir, mErr := machineProxyDir(); mErr == nil {
				machineProxy := filepath.Join(machineDir, command+".cmd")
				if mManaged, mErr := utils.IsManagedProxy(machineProxy); mErr == nil {
					if !mManaged {
						return fmt.Errorf("file is not a runx-generated proxy: %s", machineProxy)
					}
					if err := os.Remove(machineProxy); err != nil {
						return fmt.Errorf("failed to remove machine proxy (try running as Administrator): %w", err)
					}
					fmt.Printf("Machine proxy removed: %s\n", machineProxy)
					if err := cleanupMachineProxyDirPathIfEmpty(machineDir); err != nil {
						return err
					}
					return nil
				}
			}
			return fmt.Errorf("proxy not found: %s", filepath.Join(proxyDir, command+".cmd"))
		}
		return err
	}
	if !managed {
		return fmt.Errorf("file is not a runx-generated proxy: %s", proxyPath)
	}

	if err := os.Remove(proxyPath); err != nil {
		return fmt.Errorf("failed to remove proxy: %w", err)
	}

	fmt.Printf("Proxy removed: %s\n", proxyPath)
	if err := cleanupUserProxyDirPathIfEmpty(proxyDir); err != nil {
		return err
	}
	return nil
}
