//go:build windows
// +build windows

package utils

import "golang.org/x/sys/windows/registry"

// IsElevated checks whether the current process has administrator privileges
// by attempting to open the Machine PATH registry key for writing.
func IsElevated() bool {
	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`System\CurrentControlSet\Control\Session Manager\Environment`,
		registry.SET_VALUE,
	)
	if err != nil {
		return false
	}
	k.Close()
	return true
}
