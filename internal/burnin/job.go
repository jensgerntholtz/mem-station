package burnin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolvePreset locates the burn-in job file. Order: explicit path, the
// MEM_STATION_JOB environment variable, ./burnin-memory.job, then a file next
// to the executable. It returns the preset content and a human-readable source
// description.
func resolvePreset(explicit string) ([]byte, string, error) {
	tryRead := func(path string) ([]byte, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read job %s: %w", path, err)
		}
		return data, nil
	}

	if explicit != "" {
		data, err := tryRead(explicit)
		return data, explicit, err
	}

	if p := strings.TrimSpace(os.Getenv("MEM_STATION_JOB")); p != "" {
		data, err := tryRead(p)
		return data, p, err
	}

	if _, err := os.Stat("burnin-memory.job"); err == nil {
		data, err := tryRead("burnin-memory.job")
		return data, "./burnin-memory.job", err
	}

	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "burnin-memory.job")
		if _, statErr := os.Stat(p); statErr == nil {
			data, err := tryRead(p)
			return data, p, err
		}
	}

	return nil, "", fmt.Errorf("burnin-memory.job not found; set MEM_STATION_JOB or pass --job <path>")
}
