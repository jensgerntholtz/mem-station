package benchmark

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"mem-station/internal/ui"
)

// durations are the selectable run lengths. The entry at customDurationIndex
// is the "custom" option, whose value the user types.
var durations = []time.Duration{
	15 * time.Second,
	30 * time.Second,
	time.Minute,
	5 * time.Minute,
}

// customDurationIndex is the duration option that takes a typed value.
const customDurationIndex = 4

// buffers are the stress-ng --memrate-bytes values offered.
var buffers = []string{"128M", "256M", "512M", "1G"}

// defaultBufferIndex selects 256M, the Phase 1 default.
const defaultBufferIndex = 1

// minDuration and maxDuration bound a benchmark run (the custom option
// included) so a typo cannot turn into an all-night job.
const (
	minDuration = time.Second
	maxDuration = 2 * time.Hour
)

// Options is the benchmark configuration: exactly what one run executes.
type Options struct {
	Duration time.Duration
	Buffer   string
	Workers  int
}

// DefaultOptions returns the Phase 1 defaults with configurable fields.
func DefaultOptions() Options {
	return Options{Duration: 30 * time.Second, Buffer: buffers[defaultBufferIndex], Workers: 1}
}

// maxWorkers is the number of concurrent memrate stressors offered, derived
// from the host CPU count.
func maxWorkers() int {
	if n := runtime.NumCPU(); n > 1 {
		return n
	}
	return 1
}

// Validate rejects values that would build a bogus stress-ng invocation.
func (o Options) Validate() error {
	if o.Duration < minDuration || o.Duration > maxDuration {
		return fmt.Errorf("duration must be between %s and %s", ui.DurationLabel(minDuration), ui.DurationLabel(maxDuration))
	}
	if !validBuffer(o.Buffer) {
		return fmt.Errorf("unsupported buffer size %q (use %s)", o.Buffer, strings.Join(buffers, ", "))
	}
	if o.Workers < 1 || o.Workers > maxWorkers() {
		return fmt.Errorf("workers must be between 1 and %d", maxWorkers())
	}
	return nil
}

// Args returns the stress-ng argv for the run. Arguments are always passed as
// a slice, never as a shell string.
func (o Options) Args() []string {
	return []string{
		"--memrate", strconv.Itoa(o.Workers),
		"--memrate-bytes", o.Buffer,
		"--metrics",
		"-t", strconv.Itoa(int(o.Duration.Round(time.Second).Seconds())),
	}
}

// Label is the human-readable configuration summary shown in the tab and in
// run history.
func (o Options) Label() string {
	return fmt.Sprintf("%s · %s · %s", ui.DurationLabel(o.Duration), o.Buffer, workerLabel(o.Workers))
}

// Signature identifies the configuration a run used, so history rows are never
// compared across different settings.
func (o Options) Signature() string {
	return fmt.Sprintf("%s/%s/%d", ui.DurationLabel(o.Duration), o.Buffer, o.Workers)
}

// payloadOptions is the JSON form stored in run history and exports.
type payloadOptions struct {
	DurationSeconds int    `json:"duration_s"`
	Buffer          string `json:"buffer"`
	Workers         int    `json:"workers"`
}

func (o Options) payload() payloadOptions {
	return payloadOptions{
		DurationSeconds: int(o.Duration.Round(time.Second).Seconds()),
		Buffer:          o.Buffer,
		Workers:         o.Workers,
	}
}

// options rebuilds Options from its JSON form; unknown buffers fall back to the
// default so an old record still renders a label.
func (p payloadOptions) options() Options {
	o := DefaultOptions()
	if p.DurationSeconds > 0 {
		o.Duration = time.Duration(p.DurationSeconds) * time.Second
	}
	if p.Buffer != "" {
		o.Buffer = p.Buffer
	}
	if p.Workers > 0 {
		o.Workers = p.Workers
	}
	return o
}

func workerLabel(n int) string {
	if n == 1 {
		return "1 worker"
	}
	return fmt.Sprintf("%d workers", n)
}

func validBuffer(b string) bool {
	for _, want := range buffers {
		if b == want {
			return true
		}
	}
	return false
}

// durationFor returns the duration of the selected option, using custom for the
// custom entry.
func durationFor(index int, custom time.Duration) time.Duration {
	if index == customDurationIndex || index < 0 || index >= len(durations) {
		return custom
	}
	return durations[index]
}

// durationLabel names the selected duration option in the config row.
func durationLabel(index int, custom time.Duration) string {
	if index == customDurationIndex {
		return "custom " + ui.DurationLabel(custom)
	}
	return ui.DurationLabel(durationFor(index, custom))
}

// cycleDuration moves the duration selection by dir, clamping at both ends.
// It reports whether the selection moved.
func cycleDuration(index *int, dir int) bool {
	next := *index + dir
	if next < 0 || next > customDurationIndex {
		return false
	}
	*index = next
	return true
}

// cycleBuffer moves the buffer selection by dir, clamping at both ends.
func cycleBuffer(index *int, dir int) bool {
	next := *index + dir
	if next < 0 || next >= len(buffers) {
		return false
	}
	*index = next
	return true
}

// cycleWorkers moves the worker count by dir within 1..maxWorkers.
func cycleWorkers(workers *int, dir int) bool {
	next := *workers + dir
	if next < 1 || next > maxWorkers() {
		return false
	}
	*workers = next
	return true
}

// parseDurationInput accepts a Go duration ("90s", "2m", "1h") or a bare
// number of seconds.
func parseDurationInput(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("enter a duration such as 90s or 2m")
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	return 0, fmt.Errorf("cannot parse %q; use 90s, 2m or 1h", s)
}

// wrapIndex returns i wrapped into 0..n-1 for a non-empty list.
func wrapIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	i %= n
	if i < 0 {
		i += n
	}
	return i
}
