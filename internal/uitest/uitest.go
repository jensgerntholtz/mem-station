// Package uitest is the shared golden-file harness for view tests. It keeps one
// ANSI-stripping rule, one -update flag per test binary, and one exact-size
// assertion, so every package's golden files are compared the same way.
package uitest

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// update rewrites golden files instead of comparing them.
var update = flag.Bool("update", false, "rewrite golden files")

// ansiRe matches CSI/SGR escape sequences emitted by lipgloss.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// Update reports whether goldens should be rewritten. Either the -update flag
// (which only exists in test binaries that link this package) or the
// MEM_STATION_UPDATE_GOLDEN environment variable rewrites them. The env var is
// what makes `go test ./... -run TestGolden` work repo-wide: packages without
// golden tests never register the flag, so passing -update to ./... fails
// before any test runs.
func Update() bool {
	if *update {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MEM_STATION_UPDATE_GOLDEN"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// StripANSI removes escape sequences so goldens do not depend on the terminal
// color profile.
func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// Golden compares got with dir/name, creating or rewriting it when Update is
// true. Comparison ignores ANSI sequences and trailing newlines.
func Golden(t *testing.T, dir, name, got string) {
	t.Helper()
	got = strings.TrimRight(StripANSI(got), "\n")
	path := filepath.Join(dir, name)
	if Update() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update): %v", path, err)
	}
	if got != strings.TrimRight(string(want), "\n") {
		t.Errorf("golden %s mismatch:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

// GoldenView renders nothing itself: it asserts that got is exactly w x h cells
// and then compares it with the golden file, which is what view tests need at
// 80x24 and 120x40.
func GoldenView(t *testing.T, dir, name string, w, h int, got string) {
	t.Helper()
	if hh := lipgloss.Height(got); hh != h {
		t.Errorf("%s: height = %d, want %d\n%s", name, hh, h, StripANSI(got))
	}
	if ww := lipgloss.Width(got); ww != w {
		t.Errorf("%s: width = %d, want %d\n%s", name, ww, w, StripANSI(got))
	}
	Golden(t, dir, name, got)
}
