package cli

import (
	"fmt"
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/benchmark"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"
)

// runBench runs the stress-ng memrate benchmark headless and reports the parsed
// result on stdout.
func runBench(deps Deps, args []string) int {
	defaults := benchmark.DefaultOptions()
	fs := newFlagSet("bench", deps)
	duration := fs.Duration("duration", defaults.Duration, "run length, e.g. 15s, 1m, 5m")
	buffer := fs.String("buffer", defaults.Buffer, "stress-ng --memrate-bytes: 128M, 256M, 512M, 1G")
	workers := fs.Int("workers", defaults.Workers, "concurrent memrate workers")
	asJSON := fs.Bool("json", false, "print one JSON object on stdout")
	out := fs.String("out", "", "also write the JSON result to this path")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(deps.Stderr, "mem-station bench: unexpected argument %q\n", fs.Arg(0))
		return ExitUsage
	}

	opts := benchmark.Options{Duration: *duration, Buffer: *buffer, Workers: *workers}
	if err := opts.Validate(); err != nil {
		fmt.Fprintln(deps.Stderr, "mem-station bench:", err)
		return ExitUsage
	}

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
	if *out != "" {
		path, err := store.ExportFile(*out, doc)
		if err != nil {
			fmt.Fprintln(deps.Stderr, "mem-station bench: export failed:", err)
			return ExitFailed
		}
		saved = path
	}
	if *asJSON {
		if err := writeJSON(deps.Stdout, doc); err != nil {
			fmt.Fprintln(deps.Stderr, "mem-station bench:", err)
			return ExitFailed
		}
	} else {
		printBenchHuman(deps, opts, res, payload, saved)
	}
	return benchExitCode(res, payload)
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
