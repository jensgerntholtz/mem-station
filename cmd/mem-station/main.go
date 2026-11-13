package main

import (
	"flag"

	"mem-station/internal/model"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	jobFlag := flag.String("job", "", "path to burn-in job file (overrides MEM_STATION_JOB)")
	flag.Parse()

	p := tea.NewProgram(model.InitialModel(model.Options{JobPath: *jobFlag}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
