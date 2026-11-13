package cli

import (
	"fmt"
	"strconv"

	"mem-station/internal/app"
	"mem-station/internal/style"

	tea "charm.land/bubbletea/v2"
)

// tabNames maps the --tab argument to a tab index, so both "1" and "benchmark"
// work on the command line.
var tabNames = map[string]int{
	"0": 0, "timings": 0, "timing": 0, "sysinfo": 0,
	"1": 1, "benchmark": 1, "bench": 1,
	"2": 2, "burnin": 2, "burn-in": 2, "burn": 2,
}

// parseTab resolves a --tab value.
func parseTab(s string) (int, error) {
	if i, ok := tabNames[s]; ok {
		return i, nil
	}
	if i, err := strconv.Atoi(s); err == nil && i >= 0 && i <= 2 {
		return i, nil
	}
	return 0, fmt.Errorf("unknown tab %q (use timings, benchmark, burn-in, or 0-2)", s)
}

// runTUI parses the TUI flags and launches the interactive program.
func runTUI(deps Deps, args []string) int {
	fs := newFlagSet("tui", deps)
	tab := fs.String("tab", "timings", "initial tab: timings, benchmark, burn-in (or 0-2)")
	dark := fs.Bool("dark", true, "use the dark theme (pass --dark=false for light)")
	noColor := fs.Bool("no-color", false, "disable color output")
	mouse := fs.Bool("mouse", false, "enable mouse wheel scrolling")
	imc := fs.Bool("imc", false, "read raw IMC registers at startup (root; may need iomem=relaxed)")
	stateDir := fs.String("state-dir", "", "state directory (overrides MEM_STATION_STATE_DIR)")
	job := fs.String("job", "", "burn-in job file (overrides MEM_STATION_JOB)")
	version := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(deps.Stderr, "mem-station: unexpected argument %q\n", fs.Arg(0))
		return ExitUsage
	}
	if *version {
		fmt.Fprintln(deps.Stdout, deps.Version)
		return ExitOK
	}

	index, err := parseTab(*tab)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "mem-station:", err)
		return ExitUsage
	}
	if *noColor {
		// Must happen before the program starts: Bubble Tea v2 detects the
		// output color profile with colorprofile.Detect, which reads NO_COLOR.
		style.UsePlainOutput()
	}

	opts := app.DefaultOptions()
	opts.InitialTab = index
	opts.JobPath = *job
	opts.StateDir = *stateDir
	opts.Dark = *dark
	opts.Mouse = *mouse
	opts.IMC = *imc
	opts.Version = deps.Version

	if err := deps.RunTUI(opts); err != nil {
		fmt.Fprintln(deps.Stderr, "mem-station:", err)
		return ExitFailed
	}
	return ExitOK
}

// runTUIProgram starts the Bubble Tea program. It is the production RunTUI
// dependency; tests inject their own so no terminal is required.
//
// Bubble Tea v2 has no alt-screen or mouse program options: the model declares
// them on the view, so this launcher stays option-free. --no-color is honoured
// through the NO_COLOR environment variable that style.UsePlainOutput sets
// before the program starts, which colorprofile.Detect reads.
func runTUIProgram(opts app.Options) error {
	_, err := tea.NewProgram(app.New(opts)).Run()
	return err
}
