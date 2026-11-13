package burnin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"mem-station/internal/ui"
)

// durations are the selectable burn-in lengths. The entry at
// customDurationIndex is the "custom" option, whose value the user types.
var durations = []time.Duration{
	5 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
	time.Hour,
}

// customDurationIndex is the duration option that takes a typed value.
const customDurationIndex = 4

// durationIndexDefault selects the 30m option, matching the repo job file.
const durationIndexDefault = 2

// minDuration and maxDuration bound a burn-in run; the upper bound keeps a
// typo from becoming an overnight job.
const (
	minDuration = time.Minute
	maxDuration = 8 * time.Hour
)

// Options is the burn-in configuration used by one run.
type Options struct {
	Duration    time.Duration
	StopOnError bool
}

// Validate rejects a duration outside the supported window.
func (o Options) Validate() error {
	if o.Duration < minDuration || o.Duration > maxDuration {
		return fmt.Errorf("duration must be between %s and %s", ui.DurationLabel(minDuration), ui.DurationLabel(maxDuration))
	}
	return nil
}

// Label is the human-readable configuration summary shown in the tab and in
// run history.
func (o Options) Label() string {
	return fmt.Sprintf("%s · stop on error: %s", ui.DurationLabel(o.Duration), onOff(o.StopOnError))
}

// Signature identifies the configuration a run used so history is never
// compared across different settings.
func (o Options) Signature() string {
	return ui.DurationLabel(o.Duration) + "/" + strconv.FormatBool(o.StopOnError)
}

// payloadOptions is the JSON form stored in run history and exports.
type payloadOptions struct {
	PresetName      string `json:"preset"`
	DurationSeconds int    `json:"duration_s"`
	StopOnError     bool   `json:"stop_on_error"`
}

func (o Options) payload(presetName string) payloadOptions {
	return payloadOptions{
		PresetName:      presetName,
		DurationSeconds: int(o.Duration.Round(time.Second).Seconds()),
		StopOnError:     o.StopOnError,
	}
}

// options rebuilds the run options from their JSON form.
func (p payloadOptions) options() Options {
	o := Options{StopOnError: p.StopOnError}
	if p.DurationSeconds > 0 {
		o.Duration = time.Duration(p.DurationSeconds) * time.Second
	} else {
		o.Duration = durations[durationIndexDefault]
	}
	return o
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
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
func cycleDuration(index *int, dir int) bool {
	next := *index + dir
	if next < 0 || next > customDurationIndex {
		return false
	}
	*index = next
	return true
}

// parseDurationInput accepts a Go duration ("90m", "2h") or a bare number of
// minutes, which is the natural unit for a burn-in.
func parseDurationInput(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("enter a duration such as 45m or 2h")
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return time.Duration(n) * time.Minute, nil
	}
	return 0, fmt.Errorf("cannot parse %q; use 45m or 2h", s)
}
