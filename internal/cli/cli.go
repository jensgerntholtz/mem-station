// Package cli implements the mem-station command line: the TUI launcher and the
// headless info/bench/burnin subcommands.
//
// It is the only place in the module allowed to write to stdout/stderr. JSON
// goes to stdout as one document per command, human output goes to stdout, and
// diagnostics go to stderr through the leveled logger, so a script can always
// parse stdout.
//
// The command tree is cobra, rendered by fang; the run logic below stays plain
// Go so tests can drive it with injected dependencies and no terminal.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"mem-station/internal/app"
	"mem-station/internal/memory"
	"mem-station/internal/runner"

	"charm.land/fang/v2"
	"charm.land/log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	// Log receives structured diagnostics. A nil Log means a plain logger on
	// Stderr with no timestamps, which is what the tests capture.
	Log *log.Logger

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

// withDefaults fills in any dependency a test left out.
func (d Deps) withDefaults() Deps {
	def := DefaultDeps()
	if d.Stdout == nil {
		d.Stdout = def.Stdout
	}
	if d.Stderr == nil {
		d.Stderr = def.Stderr
	}
	if d.Log == nil {
		// No timestamp: diagnostics belong next to the command that produced
		// them, and a stable prefix keeps stderr diffable in tests.
		d.Log = log.NewWithOptions(d.Stderr, log.Options{
			ReportTimestamp: false,
			Prefix:          "mem-station",
		})
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

// exitError carries a process exit code through cobra's error path. fang prints
// the wrapped error unless silent is set, which is how a run that already
// reported its verdict on stdout keeps stderr clean.
type exitError struct {
	code   int
	err    error
	silent bool
}

func (e exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("exit status %d", e.code)
	}
	return e.err.Error()
}

func (e exitError) Unwrap() error { return e.err }

// usageErr reports a bad flag value or argument. fang prints it and Run maps it
// to ExitUsage.
func usageErr(err error) error { return exitError{code: ExitUsage, err: err} }

// fail reports a command that could not do its job.
func fail(err error) error { return exitError{code: ExitFailed, err: err} }

// exitCodeErr turns a finished run's exit code into cobra's error channel. A
// zero code is success; non-zero codes are silent because stdout already holds
// the verdict.
func exitCodeErr(code int) error {
	if code == ExitOK {
		return nil
	}
	return exitError{code: code, silent: true}
}

// Run executes one command line (os.Args[1:]) and returns the process exit
// code.
func Run(args []string, deps Deps) int {
	deps = deps.withDefaults()

	root, cleanup := newRootCmd(deps)
	defer cleanup()

	root.SetArgs(normalizeArgs(args))

	err := fang.Execute(context.Background(), root,
		fang.WithVersion(deps.Version),
		// A finished-but-failed run already wrote its verdict; only real errors
		// should reach stderr through the error handler.
		fang.WithErrorHandler(func(w io.Writer, styles fang.Styles, err error) {
			var ee exitError
			if errors.As(err, &ee) && ee.silent {
				return
			}
			fang.DefaultErrorHandler(w, styles, err)
		}),
	)
	switch {
	case err == nil:
		return ExitOK
	}
	var ee exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	// Anything else is cobra: an unknown command, an unknown flag, or a bad
	// argument count.
	return ExitUsage
}

// newRootCmd builds the cobra tree. The returned cleanup closes an opened
// --log-file, so Run can defer it.
func newRootCmd(deps Deps) (*cobra.Command, func()) {
	var (
		tuiOpts tuiOptions
		logFile string
		verbose bool
		quiet   bool
		closer  io.Closer
	)

	root := &cobra.Command{
		Use:   "mem-station",
		Short: "Memory timing workspace and stability tools",
		Long: `MemStation - memory timing workspace and stability tools

Inspect timings, sketch a profile, measure bandwidth, and burn in a memory
configuration from one terminal workspace.

Profile only: applying timings writes a profile file. Nothing is written to
MSRs, BIOS, or hardware timings.`,
		// Root with no subcommand launches the TUI.
		Args: cobra.NoArgs,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			c, err := configureLog(deps, logFile, verbose, quiet)
			if err != nil {
				return usageErr(err)
			}
			closer = c
			return nil
		},
		RunE: func(*cobra.Command, []string) error {
			return runTUI(deps, tuiOpts)
		},
	}
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)
	// cobra prints the version itself; keep it to the bare version string so
	// --version stays parseable.
	root.SetVersionTemplate("{{.Version}}\n")

	root.PersistentFlags().StringVar(&logFile, "log-file", "", "append diagnostics to this file")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "log debug diagnostics")
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "only log errors")

	bindTUIFlags(root.Flags(), &tuiOpts)

	tuiCmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch the terminal UI (the default command)",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return runTUI(deps, tuiOpts)
		},
	}
	bindTUIFlags(tuiCmd.Flags(), &tuiOpts)
	root.AddCommand(tuiCmd)

	root.AddCommand(newInfoCmd(deps), newBenchCmd(deps), newBurninCmd(deps), newReportCmd(deps))
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version and exit",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			fmt.Fprintln(deps.Stdout, deps.Version)
			return nil
		},
	})

	return root, func() {
		if closer != nil {
			_ = closer.Close()
		}
	}
}

// configureLog sets the level and destination the persistent flags ask for.
func configureLog(deps Deps, file string, verbose, quiet bool) (io.Closer, error) {
	out := deps.Stderr
	var closer io.Closer
	if file != "" {
		f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("log file: %w", err)
		}
		out = io.MultiWriter(deps.Stderr, f)
		closer = f
	}
	deps.Log.SetOutput(out)
	switch {
	case verbose:
		deps.Log.SetLevel(log.DebugLevel)
	case quiet:
		deps.Log.SetLevel(log.ErrorLevel)
	default:
		deps.Log.SetLevel(log.InfoLevel)
	}
	return closer, nil
}

// longFlags are the documented single-dash flags. They are rewritten to the
// double-dash form before pflag parses, so `-json` keeps working now that the
// parser is pflag (which would otherwise read it as `-j -s -o -n`).
var longFlags = map[string]bool{
	"buffer":    true,
	"dark":      true,
	"duration":  true,
	"help":      true,
	"imc":       true,
	"job":       true,
	"json":      true,
	"log-file":  true,
	"mouse":     true,
	"no-color":  true,
	"out":       true,
	"preset":    true,
	"quiet":     true,
	"state-dir": true,
	"tab":       true,
	"verbose":   true,
	"version":   true,
	"workers":   true,
}

// normalizeArgs rewrites documented single-dash long flags to the pflag form.
// Single-letter shorthands (-v, -q, -h) and everything after a bare "--" are
// left alone.
func normalizeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i, arg := range args {
		if arg == "--" {
			out = append(out, args[i:]...)
			break
		}
		if needsDoubleDash(arg) {
			arg = "-" + arg
		}
		out = append(out, arg)
	}
	return out
}

// needsDoubleDash reports whether arg is a single-dash long flag such as
// "-json" or "-duration=15s".
func needsDoubleDash(arg string) bool {
	if len(arg) < 3 || !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return false
	}
	name, _, _ := strings.Cut(arg[1:], "=")
	return longFlags[name]
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

/* --------------- flag binding --------------- */

// tuiOptions holds the TUI launcher flags, shared by the root command and the
// explicit `tui` subcommand so both accept the same set.
type tuiOptions struct {
	tab      string
	dark     bool
	noColor  bool
	mouse    bool
	imc      bool
	stateDir string
	job      string
	version  bool
}

// bindTUIFlags registers the launcher flags on fs.
func bindTUIFlags(fs *pflag.FlagSet, o *tuiOptions) {
	fs.StringVar(&o.tab, "tab", "timings", "initial tab: timings, benchmark, burn-in (or 0-2)")
	fs.BoolVar(&o.dark, "dark", true, "use the dark theme (pass --dark=false for light)")
	fs.BoolVar(&o.noColor, "no-color", false, "disable color output")
	fs.BoolVar(&o.mouse, "mouse", false, "enable mouse wheel scrolling")
	fs.BoolVar(&o.imc, "imc", false, "read raw IMC registers at startup (root; may need iomem=relaxed)")
	fs.StringVar(&o.stateDir, "state-dir", "", "state directory (overrides MEM_STATION_STATE_DIR)")
	fs.StringVar(&o.job, "job", "", "burn-in job file (overrides MEM_STATION_JOB)")
	fs.BoolVar(&o.version, "version", false, "print the version and exit")
}
