// Package report assembles the `mem-station report` document from the same
// detection result, profile and history the rest of the command line uses, and
// renders it as Markdown.
//
// The JSON form reuses the schemas that already exist: Hardware is exactly what
// `info --json` prints, Profile is the stored profile document, and History is
// the record stream from history.jsonl. Nothing here invents a second shape for
// data another command already emits.
package report

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/memory"
	"mem-station/internal/store"
)

// Hardware is the hardware section of a report and the document `info --json`
// prints. The TUI's elevation payload decodes the same shape.
type Hardware struct {
	Values   map[string]string   `json:"values"`
	Message  string              `json:"message"`
	SPD      *memory.SPDInfo     `json:"spd,omitempty"`
	IMC      []memory.IMCTimings `json:"imc,omitempty"`
	IMCError string              `json:"imc_error,omitempty"`
}

// Document is the stable JSON document `report --json` prints.
type Document struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Version     string          `json:"version"`
	Hardware    Hardware        `json:"hardware"`
	Profile     *config.Profile `json:"profile,omitempty"`
	History     []store.Record  `json:"history"`
}

// ValueOrder is the display order of detected values. Keys that are not listed
// (future detection fields) follow alphabetically, so nothing is ever dropped.
var ValueOrder = []string{
	"Channels",
	"DRAM Ratio",
	"DRAM Freq",
	"CAS (tCL)",
	"tRCD",
	"tRP",
	"tRAS",
	"Command Rate",
	"tRFC",
	"DRAM Voltage",
}

// OrderedValues returns the known keys in display order, then any extra key
// alphabetically.
func OrderedValues(values map[string]string) []string {
	seen := make(map[string]bool, len(values))
	keys := make([]string, 0, len(values))
	for _, k := range ValueOrder {
		if _, ok := values[k]; ok {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	extra := make([]string, 0, len(values))
	for k := range values {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return append(keys, extra...)
}

// Markdown renders the document as GitHub-flavoured Markdown. It emits no ANSI
// sequences, so it is safe to pipe, diff, or hand to glamour.
func Markdown(doc Document) string {
	var b strings.Builder

	b.WriteString("# MemStation report\n\n")
	fmt.Fprintf(&b, "_Generated %s · version %s_\n\n",
		doc.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC"), doc.Version)

	writeHardware(&b, doc.Hardware)
	writeProfile(&b, doc.Profile)
	writeHistory(&b, doc.History)

	b.WriteString("---\n\n")
	b.WriteString("Profile only: MemStation writes profile files, never MSRs, BIOS, or hardware timings.\n")
	return b.String()
}

func writeHardware(b *strings.Builder, h Hardware) {
	b.WriteString("## Hardware\n\n")
	if h.Message != "" {
		fmt.Fprintf(b, "%s\n\n", h.Message)
	}
	if len(h.Values) == 0 {
		b.WriteString("No values detected. Run as root, or press `E` in the TUI to elevate.\n\n")
	} else {
		keys := OrderedValues(h.Values)
		rows := make([][]string, 0, len(keys))
		for _, k := range keys {
			if v := strings.TrimSpace(h.Values[k]); v != "" {
				rows = append(rows, []string{k, v})
			}
		}
		writeTable(b, []string{"Setting", "Value"}, rows)
	}

	b.WriteString("### SPD\n\n")
	if h.SPD == nil {
		b.WriteString("Unavailable (`dmidecode` as root, or `decode-dimms`).\n\n")
	} else {
		writeSPD(b, h.SPD)
	}

	b.WriteString("### IMC registers\n\n")
	switch {
	case len(h.IMC) > 0:
		rows := make([][]string, 0, len(h.IMC))
		for _, t := range h.IMC {
			rows = append(rows, []string{
				fmt.Sprintf("channel %d", t.Channel),
				fmt.Sprintf("tRAS %d · tRP %d · tRCD %d · tRFC %d · tREFI %d",
					t.TRAS, t.TRP, t.TRCD, t.TRFC, t.TREFI),
			})
		}
		writeTable(b, []string{"Channel", "Timings"}, rows)
	default:
		reason := h.IMCError
		if reason == "" {
			reason = "no channels readable"
		}
		fmt.Fprintf(b, "Unavailable: %s\n\n", reason)
	}
}

func writeSPD(b *strings.Builder, spd *memory.SPDInfo) {
	fields := [][2]string{
		{"Part number", spd.PartNumber},
		{"Module manufacturer", spd.ModuleManufacturer},
		{"DRAM manufacturer", spd.DRAMManufacturer},
		{"Module type", spd.ModuleType},
		{"Memory type", spd.MemoryType},
		{"Speed", spd.ModuleSpeed},
		{"Size", spd.SizeLabel()},
		{"Ranks", spd.Ranks},
		{"Bus width", spd.BusWidth},
		{"Device width", spd.DeviceWidth},
		{"Supported CL", spd.SupportedCL},
		{"Voltage", spd.Voltage},
		{"Timings", spd.TimingString},
	}
	rows := make([][]string, 0, len(fields)+len(spd.TimingsBySpeed))
	for _, f := range fields {
		if v := strings.TrimSpace(f[1]); v != "" {
			rows = append(rows, []string{f[0], f[1]})
		}
	}
	for _, t := range spd.TimingsBySpeed {
		if v := strings.TrimSpace(t); v != "" {
			rows = append(rows, []string{"JEDEC profile", v})
		}
	}
	if len(rows) == 0 {
		b.WriteString("No SPD fields readable.\n\n")
		return
	}
	writeTable(b, []string{"Field", "Value"}, rows)
}

func writeProfile(b *strings.Builder, p *config.Profile) {
	b.WriteString("## Applied profile\n\n")
	if p == nil {
		b.WriteString("No profile has been applied yet. Apply one from the Timings tab " +
			"(`ctrl+s`), or import and apply a saved profile.\n\n")
		return
	}
	source := p.Source
	if source == "" {
		source = "applied"
	}
	fmt.Fprintf(b, "Source: **%s**", source)
	if !p.UpdatedAt.IsZero() {
		fmt.Fprintf(b, " · updated %s", p.UpdatedAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	b.WriteString("\n\n")

	keys := make([]string, 0, len(p.Values))
	for k, v := range p.Values {
		if strings.TrimSpace(v) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		b.WriteString("The applied profile is empty.\n\n")
		return
	}
	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, []string{k, p.Values[k]})
	}
	writeTable(b, []string{"Key", "Value"}, rows)
}

func writeHistory(b *strings.Builder, records []store.Record) {
	b.WriteString("## Recent runs\n\n")
	if len(records) == 0 {
		b.WriteString("No runs recorded yet.\n\n")
		return
	}
	// Newest first, regardless of the order the caller read them in.
	sorted := append([]store.Record(nil), records...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartedAt.After(sorted[j].StartedAt) })
	rows := make([][]string, 0, len(sorted))
	for _, r := range sorted {
		note := r.Error
		if note == "" {
			note = "-"
		}
		rows = append(rows, []string{
			r.StartedAt.UTC().Format("2006-01-02 15:04"),
			string(r.Kind),
			formatDuration(r.DurationMS),
			fmt.Sprintf("%d", r.ExitCode),
			note,
		})
	}
	writeTable(b, []string{"When (UTC)", "Kind", "Duration", "Exit", "Note"}, rows)
}

// writeTable renders a GFM table, escaping pipes so a value can never break the
// column layout.
func writeTable(b *strings.Builder, header []string, rows [][]string) {
	b.WriteString("| " + strings.Join(header, " | ") + " |\n")
	b.WriteString("| " + strings.Repeat("--- | ", len(header)-1) + "--- |\n")
	for _, row := range rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			cells[i] = escapeCell(cell)
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	b.WriteString("\n")
}

// formatDuration renders a run length compactly: 30s, 15m, 2h.
func formatDuration(ms int64) string {
	d := (time.Duration(ms) * time.Millisecond).Round(time.Second)
	switch {
	case d <= 0:
		return "0s"
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return fmt.Sprintf("%ds", d/time.Second)
	}
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}
