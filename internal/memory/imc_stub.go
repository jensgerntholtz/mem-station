//go:build !linux

package memory

// ReadMCHBARTimings is not supported on non-Linux platforms.
func ReadMCHBARTimings() ([]IMCTimings, error) {
	return nil, nil
}
