//go:build !unix

package app

// isRoot reports whether the process already has root privileges. Elevation
// via sudo is a unix-only path, so this is always false elsewhere.
func isRoot() bool { return false }
