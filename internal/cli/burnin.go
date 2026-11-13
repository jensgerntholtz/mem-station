package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"mem-station/internal/burnin"
	"mem-station/internal/burnin/parse"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"
)

// runBurnin materializes a burn-in job with the chosen timeout and runs it
// headless, reporting the parsed verdict on stdout.
func runBurnin(deps Deps, args []string) int {
	fs := newFlagSet("burnin", deps)
	duration := fs.Duration("duration", 30*time.Minute, "run length, e.g. 5m, 30m, 1h")
	presetName := fs.String("preset", "", "preset name or index (default: memory-heavy embedded)")
	jobPath := fs.String("job", "", "custom stress-ng job file (overrides MEM_STATION_JOB)")
	asJSON := fs.Bool("json", false, "print one JSON object on stdout")
	out := fs.String("out", "", "also write the JSON result to this path")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(deps.Stderr, "mem-station burnin: unexpected argument %q\n", fs.Arg(0))
		return ExitUsage
	}

	opts := burnin.Options{Duration: *duration, StopOnError: true}
	if err := opts.Validate(); err != nil {
		fmt.Fprintln(deps.Stderr, "mem-station burnin:", err)
		return ExitUsage
	}
	preset, err := resolvePreset(deps, *jobPath, *presetName)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "mem-station burnin:", err)
		return ExitUsage
	}

	materialized, err := store.MaterializeJob(preset.Content, opts.Duration)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "mem-station burnin: cannot materialize job:", err)
		return ExitFailed
	}

	ctx, stop := deps.context()
	defer stop()

	spec := runner.Spec{
		Kind:     runner.KindBurnIn,
		Name:     "stress-ng",
		Args:     []string{"--job", materialized},
		LogPath:  store.LogPath(runner.KindBurnIn, time.Now()),
		Duration: opts.Duration,
	}
	res := runJob(ctx, deps.Start, spec)
	results := parse.ParseBurnInOutput(res.Output, res.ExitCode, res.Err)

	verdict := burnin.VerdictFailed
	switch {
	case res.cancelled():
		verdict = burnin.VerdictCancelled
	case results.Success:
		verdict = burnin.VerdictPassed
	}
	payload := burnin.NewPayload(opts, preset.ShortName(), verdict, results.Errors, opts.StopOnError, results.Duration)

	rec := newRecord(runner.KindBurnIn, res, payload)
	persistRecord(deps, rec)

	doc := burnin.Export{
		Kind:       string(runner.KindBurnIn),
		StartedAt:  rec.StartedAt,
		DurationMS: rec.DurationMS,
		ExitCode:   rec.ExitCode,
		Error:      rec.Error,
		LogPath:    rec.LogPath,
		Config:     rec.Config,
		Result:     payload,
	}
	saved := ""
	if *out != "" {
		path, err := store.ExportFile(*out, doc)
		if err != nil {
			fmt.Fprintln(deps.Stderr, "mem-station burnin: export failed:", err)
			return ExitFailed
		}
		saved = path
	}
	if *asJSON {
		if err := writeJSON(deps.Stdout, doc); err != nil {
			fmt.Fprintln(deps.Stderr, "mem-station burnin:", err)
			return ExitFailed
		}
	} else {
		printBurninHuman(deps, preset, opts, res, payload, materialized, saved)
	}
	return burninExitCode(payload)
}

// resolvePreset selects a job source by name, index, or the default (the
// embedded memory-heavy preset).
func resolvePreset(deps Deps, jobPath, name string) (burnin.Preset, error) {
	presets, warn := burnin.Presets(jobPath)
	if warn != "" {
		fmt.Fprintln(deps.Stderr, "mem-station burnin:", warn)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return presets[0], nil
	}
	if i, err := strconv.Atoi(name); err == nil {
		if i < 0 || i >= len(presets) {
			return burnin.Preset{}, fmt.Errorf("preset index %d out of range (0-%d)", i, len(presets)-1)
		}
		return presets[i], nil
	}
	for _, p := range presets {
		if strings.EqualFold(p.Name, name) || strings.EqualFold(p.ShortName(), name) {
			return p, nil
		}
	}
	available := make([]string, 0, len(presets))
	for i, p := range presets {
		available = append(available, fmt.Sprintf("%d=%s", i, p.ShortName()))
	}
	return burnin.Preset{}, fmt.Errorf("unknown preset %q (available: %s)", name, strings.Join(available, ", "))
}

// burninExitCode maps the run verdict to a process exit code.
func burninExitCode(payload burnin.Payload) int {
	switch payload.Verdict {
	case burnin.VerdictPassed:
		return ExitOK
	case burnin.VerdictCancelled:
		return ExitSigint
	default:
		return ExitFailed
	}
}

// printBurninHuman renders the burn-in verdict for a terminal.
func printBurninHuman(deps Deps, preset burnin.Preset, opts burnin.Options, res jobResult, payload burnin.Payload, jobPath, saved string) {
	fmt.Fprintf(deps.Stdout, "Burn-in: %s (%s)\n", preset.Name, opts.Label())
	switch payload.Verdict {
	case burnin.VerdictPassed:
		fmt.Fprintf(deps.Stdout, "  Verdict   %s passed\n", ui.MarkOK)
	case burnin.VerdictCancelled:
		fmt.Fprintf(deps.Stdout, "  Verdict   %s cancelled - no stability conclusion\n", ui.MarkWarn)
	default:
		fmt.Fprintf(deps.Stdout, "  Verdict   %s failed\n", ui.MarkError)
	}
	fmt.Fprintf(deps.Stdout, "  Errors    %d\n", payload.Errors)
	fmt.Fprintf(deps.Stdout, "  Exit      %d\n", res.ExitCode)
	if payload.ParsedDuration != "" {
		fmt.Fprintf(deps.Stdout, "  Duration  %s\n", payload.ParsedDuration)
	}
	if res.Err != nil {
		fmt.Fprintf(deps.Stdout, "  Reason    %s\n", res.errText())
	}
	fmt.Fprintf(deps.Stdout, "  Job       %s\n", jobPath)
	if res.LogPath != "" {
		fmt.Fprintf(deps.Stdout, "  Log       %s\n", res.LogPath)
	}
	if saved != "" {
		fmt.Fprintf(deps.Stdout, "  Saved     %s\n", saved)
	}
}
