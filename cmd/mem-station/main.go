// Command mem-station runs the MemStation memory timing workspace TUI.
package main

import (
	"flag"
	"fmt"
	"os"

	"mem-station/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	opts := app.DefaultOptions()
	var (
		jobPath    string
		initialTab int
		dark       bool
		version    bool
	)
	flag.StringVar(&jobPath, "job", "", "path to a burn-in job file (overrides MEM_STATION_JOB)")
	flag.IntVar(&initialTab, "tab", 0, "initial tab: 0 timings, 1 benchmark, 2 burn-in")
	flag.BoolVar(&dark, "dark", true, "use the dark theme")
	flag.BoolVar(&version, "version", false, "print the version and exit")
	flag.Parse()

	if version {
		fmt.Println(app.Version)
		return
	}
	opts.InitialTab = initialTab
	opts.JobPath = jobPath
	opts.Dark = dark

	programOptions := []tea.ProgramOption{tea.WithAltScreen()}
	if opts.Mouse {
		programOptions = append(programOptions, tea.WithMouseCellMotion())
	}

	p := tea.NewProgram(app.New(opts), programOptions...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mem-station:", err)
		os.Exit(1)
	}
}
