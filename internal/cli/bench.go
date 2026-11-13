package cli

import (
	"fmt"
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/benchmark"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	"github.com/spf13/cobra"
)

// benchOptions are the flags of the bench command.
type benchOptions struct {
	duration time.Duration
	buffer   string
	workers  int
	asJSON   bool
	out      string
}

// newBenchCmd builds the bench subcommand.
func newBenchCmd(deps Deps) *cobra.Command {
	defaults := benchmark.DefaultOptions()
	opts := benchOptions{
		duration: defaults.Duration,
		buffer:   defaults.Buffer,
		workers:  defaults.Workers,
	}
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run the stress-ng memrate benchmark",
		Long: `Run the stress-ng memrate benchmark headless and report the parsed result.

The full run log is always written under the state directory; --out also writes
the JSON document to a path you choose. The run is attributed to the applied
profile, so apply a profile first if you want the configuration recorded.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return runBench(deps, opts)
		},
	}
	cmd.Flags().DurationVar(&opts.duration, "duration", defaults.Duration, "run length, e.g. 15s, 1m, 5m")
	cmd.Flags().StringVar(&opts.buffer, "buffer", defaults.Buffer, "stress-ng --memrate-bytes: 128M, 256M, 512M, 1G")
	cmd.Flags().IntVar(&opts.workers, "workers", defaults.Workers, "concurrent memrate workers")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print one JSON object on stdout")
	cmd.Flags().StringVar(&opts.out, "out", "", "also write the JSON result to this path")
	return cmd
}

// runBench runs the stress-ng memrate benchmark headless and reports the parsed
// result on stdout.
func runBench(deps Deps, o benchOptions) error {
	opts := benchmark.Options{Duration: o.duration, Buffer: o.buffer, Workers: o.workers}
	if err := opts.Validate(); err != nil {
		return usageErr(err)
	}
	deps.Log.Debug("starting benchmark",
		"duration", opts.Duration, "buffer", opts.Buffer, "workers", opts.Workers)

	ctx, stop := deps.context()
	defer stop()

	spec := runner.Spec{
		Kind:     runner.KindBenchmark,
		Name:     "stress-ng",
		Args:     opts.Args(),
		LogPath:  store.LogPath(runner.KindBenchmark, time.Now()),
		Duration: opts.Duration,
	}
	res := runJob(ctx, deps.Start, spec)
	results := bench.ParseBenchOutput(res.Output)
	deps.Log.Debug("benchmark finished",
		"exit_code", res.ExitCode, "read_mbs", results.ReadRate, "write_mbs", results.WriteRate,
		"duration", res.duration(), "log", res.LogPath)
	payload := benchmark.NewPayload(opts, results)

	rec := newRecord(runner.KindBenchmark, res, payload)
	persistRecord(deps, rec)

	doc := benchmark.Export{
		Kind:       string(runner.KindBenchmark),
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
		printBenchHuman(deps, opts, res, payload, saved)
	}
	return exitCodeErr(benchExitCode(res, payload))
}

// benchExitCode maps the run outcome to a process exit code.
func benchExitCode(res jobResult, payload benchmark.Payload) int {
	switch {
	case res.cancelled():
		return ExitSigint
	case res.Err != nil, res.ExitCode != 0, !payload.Parsed:
		return ExitFailed
	default:
		return ExitOK
	}
}

// printBenchHuman renders the benchmark result for a terminal.
func printBenchHuman(deps Deps, opts benchmark.Options, res jobResult, payload benchmark.Payload, saved string) {
	fmt.Fprintf(deps.Stdout, "Benchmark: %s\n", opts.Label())
	switch {
	case res.cancelled():
		fmt.Fprintf(deps.Stdout, "  Verdict   %s cancelled\n", ui.MarkWarn)
	case res.Err != nil:
		fmt.Fprintf(deps.Stdout, "  Verdict   %s failed: %v\n", ui.MarkError, res.Err)
	case res.ExitCode != 0:
		fmt.Fprintf(deps.Stdout, "  Verdict   %s failed: exit code %d\n", ui.MarkError, res.ExitCode)
	case !payload.Parsed:
		fmt.Fprintf(deps.Stdout, "  Verdict   %s failed: could not parse memrate output\n", ui.MarkError)
	default:
		fmt.Fprintf(deps.Stdout, "  Verdict   %s done\n", ui.MarkOK)
		fmt.Fprintf(deps.Stdout, "  Read      %.0f MB/s\n", payload.ReadMBs)
		fmt.Fprintf(deps.Stdout, "  Write     %.0f MB/s\n", payload.WriteMBs)
		for _, m := range payload.Metrics {
			fmt.Fprintf(deps.Stdout, "  %-9s %.0f MB/s\n", m.Label, m.MBs)
		}
		if payload.ParsedDuration != "" {
			fmt.Fprintf(deps.Stdout, "  Duration  %s\n", payload.ParsedDuration)
		}
	}
	if res.LogPath != "" {
		fmt.Fprintf(deps.Stdout, "  Log       %s\n", res.LogPath)
	}
	if saved != "" {
		fmt.Fprintf(deps.Stdout, "  Saved     %s\n", saved)
	}
}
