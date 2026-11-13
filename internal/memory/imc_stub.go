//go:build !linux

package memory

import "fmt"

// ReadMCHBARTimings returns an unsupported-platform error on non-Linux systems.
func ReadMCHBARTimings() ([]IMCTimings, error) {
	return nil, fmt.Errorf("IMC timing read is not supported on this platform")
}
