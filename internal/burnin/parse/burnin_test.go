package parse

import (
	"strings"
	"testing"

	"errors"
)

// successLog is a realistic stress-ng burn-in tail with no errors.
const successLog = `stress-ng: info: [123] setting to run for up to 1800 seconds
stress-ng: info: [123] dispatching hogs: 42 stressors
stress-ng: info: [123] successful run completed in 1800.02 secs (0 errors)`

// errorLog has two real error lines and one phrase reporting the absence of
// errors, which must not be counted.
const errorLog = `stress-ng: info: [123] dispatching hogs: 42 stressors
stress-ng: error: [124] memthrash: memthrash: error, mismatched data
stress-ng: fail: [125] vm: brk: out of memory
stress-ng: info: [123] unsuccessful run completed in 12.44 secs`

// TestParseSuccess checks a clean run is a success with its duration.
func TestParseSuccess(t *testing.T) {
	r := ParseBurnInOutput(successLog, 0, nil)
	if !r.Success {
		t.Errorf("Success = false, want true (errors=%d)", r.Errors)
	}
	if r.Errors != 0 {
		t.Errorf("Errors = %d, want 0", r.Errors)
	}
	if r.Duration != "1800.02 secs" {
		t.Errorf("Duration = %q", r.Duration)
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode)
	}
	if !strings.Contains(r.RawOutput, "dispatching hogs") {
		t.Error("RawOutput lost the log")
	}
}

// TestParseErrorsRequireZeroLines checks success needs both exit 0 and no error
// lines.
func TestParseErrorsRequireZeroLines(t *testing.T) {
	r := ParseBurnInOutput(errorLog, 0, nil)
	if r.Success {
		t.Error("Success = true with error lines and exit 0")
	}
	if r.Errors != 2 {
		t.Errorf("Errors = %d, want 2", r.Errors)
	}
	if strings.Contains(strings.ToLower(r.Duration), "unsuccessful") {
		t.Errorf("Duration parsed from an unsuccessful line: %q", r.Duration)
	}
}

// TestParseRunnerError checks a runner error is never a success.
func TestParseRunnerError(t *testing.T) {
	r := ParseBurnInOutput(successLog, -1, errors.New("job cancelled"))
	if r.Success {
		t.Error("Success = true for a cancelled job")
	}
	if r.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", r.ExitCode)
	}
}

// TestIsErrorLine covers the shared live error counter and log filter.
func TestIsErrorLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"stress-ng: error: [1] vm: failed", true},
		{"stress-ng: fail: [2] malloc: out of memory", true},
		{"stress-ng: info: completed in 10.00 secs", false},
		{"stress-ng: info: successful run completed in 10.00 secs (0 errors)", false},
		{"stress-ng: info: 0 errors detected", false},
		{"stress-ng: info: no errors seen", false},
		{"kernel: segfault at 0 ip 0000", true},
	}
	for _, tc := range cases {
		if got := IsErrorLine(tc.line); got != tc.want {
			t.Errorf("IsErrorLine(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

// TestRawOutputIsBounded checks the tail keeps only the last lines.
func TestRawOutputIsBounded(t *testing.T) {
	lines := make([]string, 0, maxOutputLines+10)
	for i := 0; i < maxOutputLines+10; i++ {
		lines = append(lines, "stress-ng: info: line")
	}
	lines[len(lines)-1] = "stress-ng: info: last line"
	r := ParseBurnInOutput(strings.Join(lines, "\n"), 0, nil)
	got := strings.Split(r.RawOutput, "\n")
	if len(got) != maxOutputLines {
		t.Errorf("RawOutput lines = %d, want %d", len(got), maxOutputLines)
	}
	if got[len(got)-1] != "stress-ng: info: last line" {
		t.Errorf("RawOutput tail = %q", got[len(got)-1])
	}
}
