package sysinfo

import (
	"fmt"
	"strconv"

	"mem-station/internal/memory"
	"mem-station/internal/style"
	"mem-station/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// imcLines renders the IMC register dump: one block per memory channel with
// the turnaround matrix and the raw decoded register values.
func (m *Model) imcLines() []string {
	lines := []string{style.Section().Render("IMC Tertiary Timings (Haswell)")}
	if m.imcNote != "" {
		lines = append(lines, style.Warning().Render(m.imcNote))
	}
	if len(m.imc) == 0 {
		lines = append(lines, "",
			style.Muted().Render("No IMC data. Reading MCHBAR needs root and the iomem=relaxed kernel parameter."),
		)
		return lines
	}
	for _, ch := range m.imc {
		lines = append(lines, "", m.channelIMC(ch))
	}
	return lines
}

func (m *Model) channelIMC(t memory.IMCTimings) string {
	cell := func(v int) string { return strconv.Itoa(v) }

	matrix := ui.JoinColumns([][]string{
		{"", "sg", "dg", "dr", "dd"},
		{"tRDRD", cell(t.TRDRDsg), cell(t.TRDRDdg), cell(t.TRDRDdr), cell(t.TRDRDdd)},
		{"tWRWR", cell(t.TWRWRsg), cell(t.TWRWRdg), cell(t.TWRWRdr), cell(t.TWRWRdd)},
		{"tRDWR", cell(t.TRDWRsg), cell(t.TRDWRdg), cell(t.TRDWRdr), cell(t.TRDWRdd)},
		{"tWRRD", cell(t.TWRRDsg), cell(t.TWRRDdg), cell(t.TWRRDdr), cell(t.TWRRDdd)},
	}, []int{12, 4, 4, 4, 4})

	registers := ui.JoinColumns([][]string{
		{"tRAS", cell(t.TRAS)},
		{"tRCD", cell(t.TRCD)},
		{"tRP", cell(t.TRP)},
		{"tRTP", cell(t.TRTP)},
		{"tWTR", cell(t.TWTR)},
		{"tRRD sg/dg", fmt.Sprintf("%d/%d", t.TRRDsg, t.TRRDdg)},
		{"tCKE", cell(t.TCKE)},
		{"tREFI", cell(t.TREFI)},
		{"tRFC", cell(t.TRFC)},
	}, []int{14, 6})

	left := style.Section().Render(fmt.Sprintf("Channel %d - Turnaround", t.Channel)) + "\n" + matrix
	right := style.Section().Render("Register Values") + "\n" + registers
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(34).Render(left), "  ", right)
}
