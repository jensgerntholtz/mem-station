// Package burnin parses stress-ng burn-in output into a Results summary.
//
// It is a leaf package: it does not execute commands or write to stdout/stderr.
package burnin

import (
	"regexp"
	"strings"
)

// Results summarizes a finished burn-in run.
type Results struct {
	Duration  string // parsed from output; empty if unknown
	Errors    int    // number of output lines that look like errors
	ExitCode  int
	LogPath   string // populated by the caller after parsing
	RawOutput string // bounded to the last maxOutputLines lines
	Success   bool   // exit code 0 and zero error lines and no runner error
}

// maxOutputLines bounds the retained RawOutput.
const maxOutputLines = 2000

var (
	errorLineRe = regexp.MustCompile(`(?i)\b(error|errors|failed|failure|fatal|aborted|abort|segfault|out of memory)\b`)
	durationRe  = regexp.MustCompile(`(?i)(?:successful run completed in|completed in)\s+([\d.]+\s*(?:secs?|seconds|mins?|minutes|hrs?|hours))`)
	realTimeRe  = regexp.MustCompile(`(?i)real time\s+([\d.]+)\s*secs?`)
)

// ParseBurnInOutput parses a completed run. err is the runner's terminal error
// (nil on success, non-nil on cancellation/exit error/lookup failure).
func ParseBurnInOutput(output string, exitCode int, err error) *Results {
	r := &Results{
		ExitCode:  exitCode,
		Errors:    countErrorLines(output),
		RawOutput: tailLines(output, maxOutputLines),
	}
	r.Duration = parseDuration(output)
	r.Success = (err == nil) && (exitCode == 0) && (r.Errors == 0)
	return r
}

func countErrorLines(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if errorLineRe.MatchString(line) {
			count++
		}
	}
	return count
}

func parseDuration(output string) string {
	if m := durationRe.FindStringSubmatch(output); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := realTimeRe.FindStringSubmatch(output); len(m) > 1 {
		return strings.TrimSpace(m[1]) + " secs"
	}
	return ""
}

func tailLines(output string, n int) string {
	lines := strings.Split(output, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
