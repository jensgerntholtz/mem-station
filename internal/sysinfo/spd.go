package sysinfo

import (
	"strings"

	"mem-station/internal/style"
)

// spdLines renders every parsed SPD field, grouped for scanning. Nothing is
// silently dropped: the panel scrolls instead.
func (m *Model) spdLines() []string {
	label := func(s string) string { return style.FieldLabel().Width(28).Render(s) }
	value := func(s string) string {
		if s == "" {
			return style.Muted().Render("-")
		}
		return style.ValueText().Render(s)
	}
	line := func(k, v string) string { return label(k) + value(v) }

	s := m.spd
	if s == nil {
		return []string{
			style.Section().Render("SPD Information"),
			"",
			style.Muted().Render("No SPD data available. Detection needs dmidecode (root) or decode-dimms."),
			style.Muted().Render("Press R on the Timings view to retry detection."),
		}
	}

	lines := []string{
		style.Section().Render("Module"),
		line("Manufacturer", s.ModuleManufacturer),
		line("Part number", s.PartNumber),
		line("Module type", s.ModuleType),
		line("Memory type", s.MemoryType),
		line("Size", s.SizeLabel()),
		line("Ranks", s.Ranks),
		line("DIMMs", s.NumDIMMs),
		"",
		style.Section().Render("Speed & Timings"),
		line("Module speed", s.ModuleSpeed),
		line("Timings", s.TimingString),
		line("Supported CL", s.SupportedCL),
		line("tCK min", s.TCKmin),
		"",
		style.Section().Render("Primary"),
		line("tRC", s.TRC),
		line("tWR", s.TWR),
		line("tRRD", s.TRRD),
		line("tWTR", s.TWTR),
		line("tRTP", s.TRTP),
		line("tFAW", s.TFAW),
		"",
		style.Section().Render("Electrical / Mechanical"),
		line("DRAM manufacturer", s.DRAMManufacturer),
		line("Voltage", s.Voltage),
		line("Bus width", s.BusWidth),
		line("Device width", s.DeviceWidth),
		line("Bank layout", s.BankLayout),
		line("DLL off", s.DLLOff),
		line("Auto self-refresh", s.AutoSR),
		line("Temp range", s.TempRange),
		line("Module height", s.ModuleHeight),
		line("Reference card", s.RefCard),
	}

	if len(s.TimingsBySpeed) > 0 {
		lines = append(lines, "", style.Section().Render("JEDEC profiles"))
		for _, t := range s.TimingsBySpeed {
			lines = append(lines, style.Muted().Render("· "+t))
		}
	}
	return lines
}

// spdSummary renders a single header line for the SPD data.
func (m *Model) spdSummary() string {
	if m.spd == nil {
		return ""
	}
	parts := []string{}
	for _, p := range []string{m.spd.ModuleSpeed, m.spd.SizeLabel(), m.spd.PartNumber} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}
