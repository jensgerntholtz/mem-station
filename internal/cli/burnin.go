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

	"github.com/spf13/cobra"
)

// burninOptions are the flags of the burnin command.
type burninOptions struct {
	duration time.Duration
	preset   string
	job      string
	asJSON   bool
	out      string
}

// newBurninCmd builds the burnin subcommand.
func newBurninCmd(deps Deps) *cobra.Command {
	opts := burninOptions{duration: 30 * time.Minute}
	cmd := &cobra.Command{
		Use:   "burnin",
		Short: "Run a stress-ng burn-in job",
		Long: `Materialize a burn-in job with the chosen timeout and run it headless.

The verdict is passed when stress-ng exits cleanly with no errors, failed
otherwise, and cancelled when the run was interrupted. Interrupting a run is
never reported as a stability result.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return runBurnin(deps, opts)
		},
	}
	cmd.Flags().DurationVar(&opts.duration, "duration", 30*time.Minute, "run length, e.g. 5m, 30m, 1h")
	cmd.Flags().StringVar(&opts.preset, "preset", "", "preset name or index (default: memory-heavy embedded)")
	cmd.Flags().StringVar(&opts.job, "job", "", "custom stress-ng job file (overrides MEM_STATION_JOB)")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print one JSON object on stdout")
	cmd.Flags().StringVar(&opts.out, "out", "", "also write the JSON result to this path")
	return cmd
}

// runBurnin materializes a burn-in job with the chosen timeout and runs it
// headless, reporting the parsed verdict on stdout.
func runBurnin(deps Deps, o burninOptions) error {
	opts := burnin.Options{Duration: o.duration, StopOnError: true}
	if err := opts.Validate(); err != nil {
		return usageErr(err)
	}
	preset, err := resolvePreset(deps, o.job, o.preset)
	if err != nil {
		return usageErr(err)
	}

	materialized, err := store.MaterializeJob(preset.Content, opts.Duration)
	if err != nil {
		return fail(fmt.Errorf("cannot materialize job: %w", err))
	}
	deps.Log.Debug("starting burn-in",
		"preset", preset.ShortName(), "duration", opts.Duration, "job", materialized)

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
	deps.Log.Debug("burn-in finished",
		"exit_code", res.ExitCode, "errors", results.Errors,
		"duration", res.duration(), "log", res.LogPath)

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
	if o.out != "" {
		path, err := store.ExportFile(o.out, doc)
		if err != nil {
			return fail(fmt.Errorf("export failed: %w", err))
		}
		saved = path
	}
	if o.asJSON {
		if err := writeJSON(deps.Stdout, doc); err != nil {
			return fail(err)
		}
	} else {
		printBurninHuman(deps, preset, opts, res, payload, materialized, saved)
	}
	return exitCodeErr(burninExitCode(payload))
}

// resolvePreset selects a job source by name, index, or the default (the
// embedded memory-heavy preset).
func resolvePreset(deps Deps, jobPath, name string) (burnin.Preset, error) {
	presets, warn := burnin.Presets(jobPath)
	if warn != "" {
		deps.Log.Warn(warn)
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
