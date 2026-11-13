//go:build unix

package app

import "os"

// isRoot reports whether the process already has root privileges, in which
// case detection needs no elevation.
func isRoot() bool { return os.Geteuid() == 0 }
