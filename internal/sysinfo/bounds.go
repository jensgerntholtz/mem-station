package sysinfo

import (
	"fmt"
	"math"

	"mem-station/internal/style"
)

// boundsFor returns the safe operating range for a field label. Ranges are
// derived from the current draft frequency, mirroring the Phase 0 rules; the
// formulas are intentionally conservative JEDEC/XMP-ish heuristics.
func (m *Model) boundsFor(label string) fieldBounds {
	freq, ok := m.parseField("DRAM Freq")
	if !ok || freq <= 0 {
		freq = 1600
	}
	cl, _ := m.parseField("CAS (tCL)")
	rcd, _ := m.parseField("tRCD")

	// Minimum cycles for a given nanosecond budget at the current frequency.
	minCycles := func(ns float64) float64 { return math.Ceil(freq * ns / 2000.0) }

	switch label {
	case "CAS (tCL)", "tRCD", "tRP":
		minCL := math.Max(5, minCycles(10))
		return fieldBounds{min: minCL, max: 18, hasMin: true, hasMax: true}
	case "tRAS":
		min := math.Max(15, math.Max(cl+rcd, minCycles(35)))
		return fieldBounds{min: min, max: 63, hasMin: true, hasMax: true, note: "≥CL+tRCD"}
	case "tRFC":
		return fieldBounds{min: minCycles(160), max: 511, hasMin: true, hasMax: true, note: "≥160ns"}
	case "Command Rate":
		return fieldBounds{min: 1, max: 2, hasMin: true, hasMax: true, note: "1T-2T"}
	case "DRAM Voltage":
		return fieldBounds{min: 1.35, max: 1.65, hasMin: true, hasMax: true, note: "1.35-1.65 V"}
	default:
		return fieldBounds{}
	}
}

// renderBounds renders the safe-range cell, colored by severity. Unknown
// values are never colored, because there is nothing to compare yet.
func (m *Model) renderBounds(f field, b fieldBounds, width int) string {
	if !b.hasMin && !b.hasMax {
		return style.Bounds.Width(width).Render("")
	}
	label := m.boundsLabel(f.label, b)
	val, ok := parseNumber(f.input.Value())
	if !ok {
		return style.Bounds.Width(width).Render(label)
	}

	below := b.hasMin && val < b.min
	above := b.hasMax && val > b.max
	voltage := f.label == "DRAM Voltage"

	switch {
	case below:
		return style.Unsafe.Width(width).Render(label)
	case above && voltage:
		return style.Unsafe.Width(width).Render(label)
	case above:
		return style.Warn.Width(width).Render(label)
	default:
		return style.Safe.Width(width).Render(label)
	}
}

func (m *Model) boundsLabel(label string, b fieldBounds) string {
	switch label {
	case "DRAM Voltage":
		return fmt.Sprintf("%.2f-%.2f V", b.min, b.max)
	case "Command Rate":
		return b.note
	}
	switch {
	case b.hasMin && b.hasMax:
		s := fmt.Sprintf("%.0f-%.0f", b.min, b.max)
		if b.note != "" {
			s += " " + b.note
		}
		return s
	case b.hasMin:
		return fmt.Sprintf("≥%.0f", b.min)
	case b.hasMax:
		return fmt.Sprintf("≤%.0f", b.max)
	default:
		return ""
	}
}
