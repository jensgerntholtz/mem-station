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

// runTUI launches the interactive program from the launcher flags.
func runTUI(deps Deps, o tuiOptions) error {
	if o.version {
		fmt.Fprintln(deps.Stdout, deps.Version)
		return nil
	}
	index, err := parseTab(o.tab)
	if err != nil {
		return usageErr(err)
	}
	if o.noColor {
		// Must happen before the program starts: Bubble Tea v2 detects the
		// output color profile with colorprofile.Detect, which reads NO_COLOR.
		style.UsePlainOutput()
	}

	opts := app.DefaultOptions()
	opts.InitialTab = index
	opts.JobPath = o.job
	opts.StateDir = o.stateDir
	opts.Dark = o.dark
	opts.Mouse = o.mouse
	opts.IMC = o.imc
	opts.Version = deps.Version

	deps.Log.Debug("launching TUI",
		"tab", index, "dark", opts.Dark, "mouse", opts.Mouse,
		"imc", opts.IMC, "state_dir", opts.StateDir, "job", opts.JobPath)

	if err := deps.RunTUI(opts); err != nil {
		return fail(err)
	}
	return nil
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
