//go:build !windows
// +build !windows

package commands

import "errors"

// These functions are only available on Windows

func addToUserPath(_ string) error                                      { return nil }
func promptAddToPath(_ string) bool                                     { return false }
func verifyPathResolution(_, _ string) (bool, string, error)            { return true, "", nil }
func promptElevatePath(_, _, _ string) (elevate bool, useAlias bool)    { return false, false }
func handlePathElevation(_, _, _ string, _ []string, _, _ string) error { return nil }
func determinePathSource(_ string) string                               { return "neither" }
func machineShimDir() (string, error)                                   { return "", errors.New("not supported on this platform") }
