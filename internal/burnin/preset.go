package burnin

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// memoryJob is the built-in "memory-heavy" preset, copied from the repo's
// burnin-memory.job. The repo file is never read or modified at run time; the
// chosen duration is materialized into the state dir instead.
//
//go:embed presets/memory.job
var memoryJob []byte

// Preset is one selectable burn-in job source.
type Preset struct {
	Name    string // display name in the config row
	Path    string // "" for the embedded preset, else the file it came from
	Content []byte
}

// ShortName is the compact preset label used in run history.
func (p Preset) ShortName() string {
	if p.Path == "" {
		return "memory"
	}
	return filepath.Base(p.Path)
}

// Presets returns the embedded memory-heavy preset plus the custom job file
// when one is configured (explicit path, MEM_STATION_JOB, ./burnin-memory.job,
// or a file next to the executable). A configured-but-unreadable custom file is
// returned as a warning instead of failing construction.
func Presets(customPath string) ([]Preset, string) {
	presets := []Preset{{Name: "memory-heavy (embedded)", Content: memoryJob}}

	path, fromEnv := discoverJobPath(customPath)
	if path == "" {
		return presets, ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if fromEnv {
			return presets, fmt.Sprintf("job file %s unavailable: %v", path, err)
		}
		return presets, ""
	}
	if len(data) == 0 {
		return presets, fmt.Sprintf("job file %s is empty", path)
	}
	return append(presets, Preset{Name: "custom: " + path, Path: path, Content: data}), ""
}

// discoverJobPath resolves the optional custom job file. fromEnv reports
// whether the path was explicitly configured, so an unreadable explicit path is
// worth a warning while a missing convenience file is not.
func discoverJobPath(customPath string) (path string, fromEnv bool) {
	if p := strings.TrimSpace(customPath); p != "" {
		return p, true
	}
	if p := strings.TrimSpace(os.Getenv("MEM_STATION_JOB")); p != "" {
		return p, true
	}
	if _, err := os.Stat("burnin-memory.job"); err == nil {
		return "burnin-memory.job", false
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "burnin-memory.job")
		if _, statErr := os.Stat(p); statErr == nil {
			return p, false
		}
	}
	return "", false
}
