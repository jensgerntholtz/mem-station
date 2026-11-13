// Package cli implements the mem-station command line: the TUI launcher and the
// headless info/bench/burnin subcommands.
//
// It is the only place in the module allowed to write to stdout/stderr. JSON
// goes to stdout as one document per command, human output goes to stdout, and
// diagnostics go to stderr, so a script can always parse stdout.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"mem-station/internal/app"
	"mem-station/internal/memory"
	"mem-station/internal/runner"
)

// Exit codes. 0 means "the command did what was asked"; a failed benchmark or
// burn-in is a 1 because the run itself failed, and 2 is reserved for usage
// errors so callers can tell them apart.
const (
	ExitOK     = 0
	ExitFailed = 1
	ExitUsage  = 2
	ExitSigint = 130
)

// Deps are the injected dependencies of the command line. Every field has a
// production default through DefaultDeps, so tests can drive the commands
// without hardware, without stress-ng, and without a terminal.
type Deps struct {
	Stdout io.Writer
	Stderr io.Writer

	// Detect runs hardware detection.
	Detect func() (values map[string]string, msg string, spd *memory.SPDInfo)
	// ReadIMC reads the IMC register dump.
	ReadIMC func() ([]memory.IMCTimings, error)
	// Start launches a job through the runner.
	Start runner.StartFunc
	// RunTUI launches the interactive program.
	RunTUI func(opts app.Options) error
	// Ctx is the run context; nil means "background plus SIGINT/SIGTERM".
	Ctx context.Context
	// Version is printed by --version and shown in the TUI header.
	Version string
}

// DefaultDeps returns the production dependencies.
func DefaultDeps() Deps {
	return Deps{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Detect:  memory.DetectDefaults,
		ReadIMC: memory.ReadMCHBARTimings,
		Start: func(ctx context.Context, spec runner.Spec) runner.Handle {
			return runner.Start(ctx, spec)
		},
		RunTUI:  runTUIProgram,
		Version: app.Version,
	}
}

// Run dispatches one command line (os.Args[1:]) and returns the process exit
// code.
func Run(args []string, deps Deps) int {
	deps = deps.withDefaults()

	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "tui":
			return runTUI(deps, args[1:])
		case "info":
			return runInfo(deps, args[1:])
		case "bench":
			return runBench(deps, args[1:])
		case "burnin":
			return runBurnin(deps, args[1:])
		case "help":
			usage(deps.Stdout)
			return ExitOK
		}
		fmt.Fprintf(deps.Stderr, "mem-station: unknown command %q\n\n", args[0])
		usage(deps.Stderr)
		return ExitUsage
	}
	// No subcommand (or leading flags such as --version) launches the TUI.
	return runTUI(deps, args)
}

// withDefaults fills in any dependency a test left out.
func (d Deps) withDefaults() Deps {
	def := DefaultDeps()
	if d.Stdout == nil {
		d.Stdout = def.Stdout
	}
	if d.Stderr == nil {
		d.Stderr = def.Stderr
	}
	if d.Detect == nil {
		d.Detect = def.Detect
	}
	if d.ReadIMC == nil {
		d.ReadIMC = def.ReadIMC
	}
	if d.Start == nil {
		d.Start = def.Start
	}
	if d.RunTUI == nil {
		d.RunTUI = def.RunTUI
	}
	if d.Version == "" {
		d.Version = def.Version
	}
	return d
}

// context returns the run context, installing a SIGINT/SIGTERM handler when the
// caller did not inject one. Cancelling it kills the runner's process group.
func (d Deps) context() (context.Context, func()) {
	if d.Ctx != nil {
		return d.Ctx, func() {}
	}
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// usage prints the command overview.
func usage(w io.Writer) {
	fmt.Fprint(w, `MemStation - memory timing workspace and stability tools

Usage:
  mem-station [tui] [flags]        launch the terminal UI (default)
  mem-station info [-json]         print detected hardware values
  mem-station bench [flags]        run the stress-ng memrate benchmark
  mem-station burnin [flags]       run a stress-ng burn-in job
  mem-station help                 print this text

TUI flags:
  --tab NAME|N      initial tab: timings (0), benchmark (1), burn-in (2)
  --dark[=false]    dark theme (default true)
  --no-color        disable color output
  --mouse           enable mouse wheel scrolling
  --state-dir DIR   state directory (overrides MEM_STATION_STATE_DIR)
  --job PATH        burn-in job file (overrides MEM_STATION_JOB)
  --version         print the version and exit

Headless flags:
  info:   -json
  bench:  -duration 15s|30s|1m|5m -buffer 128M|256M|512M|1G -workers N -json -out PATH
  burnin: -duration 5m|10m|30m|1h -preset NAME -job PATH -json -out PATH

Safety: applying timings only writes a profile. Nothing is written to MSRs, BIOS,
or hardware timings.
`)
}

// writeJSON writes v as one indented JSON document plus a newline. JSON always
// goes to stdout so stderr stays free for diagnostics.
func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// newFlagSet builds a flag set that reports errors to stderr and never exits
// the process itself.
func newFlagSet(name string, deps Deps) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)
	return fs
}
