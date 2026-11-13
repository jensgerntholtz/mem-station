package sysinfo

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"mem-station/internal/config"
)

// FieldID identifies one editable timing value. IDs are stable: they key the
// draft/applied/detected maps and appear in validation issues.
type FieldID string

// Field IDs. The string values match the canonical config keys so a profile
// written by an older build still loads.
const (
	FieldChannels FieldID = "channels"
	FieldRatio    FieldID = "dram_ratio"
	FieldFreq     FieldID = "dram_freq"
	FieldCL       FieldID = "tCL"
	FieldRCD      FieldID = "tRCD"
	FieldRP       FieldID = "tRP"
	FieldRAS      FieldID = "tRAS"
	FieldCR       FieldID = "command_rate"
	FieldRFC      FieldID = "tRFC"
	FieldVoltage  FieldID = "dram_voltage"
)

// Kind is the value shape of a field.
type Kind int

const (
	KindInt Kind = iota
	KindFloat
	KindEnum
)

// Group is the renderer section a field belongs to.
type Group string

// Field groups, rendered in this order.
const (
	GroupGeneral   Group = "General"
	GroupPrimary   Group = "Primary Timings"
	GroupSecondary Group = "Secondary Timings"
	GroupVoltage   Group = "Voltage"
)

// Severity classifies a validation finding. SeverityNone means "nothing to
// report"; ordering matters (SeverityDanger is the most severe).
type Severity int

const (
	SeverityNone Severity = iota
	SeverityInfo
	SeverityWarn
	SeverityDanger
)

// Field describes one editable value: how it is labelled, formatted, and
// validated. Values themselves are kept as strings so the UI can preserve "2T"
// and "1.35" verbatim; parsing happens in helper functions.
type Field struct {
	ID      FieldID
	Label   string
	Unit    string // unit used for margins and bound messages
	Group   Group
	Kind    Kind
	Options []string // enum values, in display order
	Default string   // placeholder shown until detection/applied values exist
	Desc    string

	// Step is the granularity a rescaled value snaps to (1 cycle, 0.01 V).
	Step float64
	// Scale is the factor applied by the ratio lock when the frequency
	// changes: 1 rescales with the frequency ratio, 0 never rescales.
	Scale float64

	Key         string // canonical config key used for persistence
	DetectLabel string // key used by memory.DetectDefaults

	Bounds func(ValidationContext) Bounds
	Format func(string) string // display formatting; nil renders the raw value
}

// Bounds is the safe operating range of a field, expressed in the field's own
// unit. An empty range (no min, no max) means "no known safe range": the UI
// never invents one.
type Bounds struct {
	Min, Max       float64
	HasMin, HasMax bool
	Unit           string
	Note           string
	// Low/High are the severities used when a value falls outside the range.
	// SeverityNone means the defaults (danger below, warning above).
	Low, High Severity
}

// Known reports whether the range constrains anything.
func (b Bounds) Known() bool { return b.HasMin || b.HasMax }

// severityBelow returns the severity for a value under Min.
func (b Bounds) severityBelow() Severity {
	if b.Low == SeverityNone {
		return SeverityDanger
	}
	return b.Low
}

// severityAbove returns the severity for a value over Max.
func (b Bounds) severityAbove() Severity {
	if b.High == SeverityNone {
		return SeverityWarn
	}
	return b.High
}

// Label renders the range for the guidance panel, e.g. "16-18 cycles".
func (b Bounds) Label() string {
	var text string
	switch {
	case b.HasMin && b.HasMax:
		text = formatNumber(b.Min) + "-" + formatNumber(b.Max)
	case b.HasMin:
		text = ">=" + formatNumber(b.Min)
	case b.HasMax:
		text = "<=" + formatNumber(b.Max)
	}
	if text == "" {
		return ""
	}
	if b.Unit != "" {
		text += " " + b.Unit
	}
	if b.Note != "" {
		text += " (" + b.Note + ")"
	}
	return text
}

// ValidationContext gives bound functions read access to the other draft
// values (timing minimums depend on the DRAM frequency, for example).
type ValidationContext struct {
	values map[FieldID]string
}

// NewValidationContext wraps a draft value map for validation.
func NewValidationContext(values map[FieldID]string) ValidationContext {
	return ValidationContext{values: values}
}

// Value returns the raw value for id.
func (c ValidationContext) Value(id FieldID) string {
	if c.values == nil {
		return ""
	}
	return c.values[id]
}

// Float returns the parsed numeric value for id.
func (c ValidationContext) Float(id FieldID) (float64, bool) {
	return parseNumber(c.Value(id))
}

// fields is the canonical, ordered field list.
var fields = []Field{
	{
		ID: FieldChannels, Label: "Channels", Group: GroupGeneral, Kind: KindInt,
		Default: "2", Step: 1, Desc: "Populated memory channels.",
		Key: config.KeyChannels, DetectLabel: "Channels",
		Bounds: func(ValidationContext) Bounds {
			return Bounds{Min: 1, Max: 8, HasMin: true, HasMax: true, Unit: "channels", Note: "1-8"}
		},
	},
	{
		ID: FieldRatio, Label: "DRAM Ratio", Unit: "x", Group: GroupGeneral, Kind: KindInt,
		Default: "?", Step: 1, Desc: "Memory multiplier relative to the base clock.",
		Key: config.KeyRatio, DetectLabel: "DRAM Ratio",
	},
	{
		ID: FieldFreq, Label: "DRAM Freq", Unit: "MT/s", Group: GroupGeneral, Kind: KindInt,
		Default: "1600", Step: 1, Scale: 1, Desc: "Effective DRAM frequency in MT/s.",
		Key: config.KeyFreq, DetectLabel: "DRAM Freq",
		Bounds: func(ValidationContext) Bounds {
			return Bounds{Min: 800, Max: 5000, HasMin: true, HasMax: true, Unit: "MT/s",
				Note: "plausible range", Low: SeverityWarn, High: SeverityWarn}
		},
	},
	{
		ID: FieldCL, Label: "CAS (tCL)", Unit: "cycles", Group: GroupPrimary, Kind: KindInt,
		Default: "?", Step: 1, Scale: 1, Desc: "Column address strobe latency in cycles.",
		Key: config.KeyCL, DetectLabel: "CAS (tCL)", Bounds: primaryBounds,
	},
	{
		ID: FieldRCD, Label: "tRCD", Unit: "cycles", Group: GroupPrimary, Kind: KindInt,
		Default: "?", Step: 1, Scale: 1, Desc: "RAS to CAS delay in cycles.",
		Key: config.KeyRCD, DetectLabel: "tRCD", Bounds: primaryBounds,
	},
	{
		ID: FieldRP, Label: "tRP", Unit: "cycles", Group: GroupPrimary, Kind: KindInt,
		Default: "?", Step: 1, Scale: 1, Desc: "Row precharge delay in cycles.",
		Key: config.KeyRP, DetectLabel: "tRP", Bounds: primaryBounds,
	},
	{
		ID: FieldRAS, Label: "tRAS", Unit: "cycles", Group: GroupPrimary, Kind: KindInt,
		Default: "?", Step: 1, Scale: 1, Desc: "Active to precharge delay in cycles.",
		Key: config.KeyRAS, DetectLabel: "tRAS", Bounds: rasBounds,
	},
	{
		ID: FieldCR, Label: "Command Rate", Unit: "T", Group: GroupSecondary, Kind: KindEnum,
		Options: []string{"1T", "2T"}, Default: "2T", Step: 1,
		Desc: "Command rate; 1T or 2T.",
		Key:  config.KeyCR, DetectLabel: "Command Rate",
		Bounds: func(ValidationContext) Bounds {
			return Bounds{Min: 1, Max: 2, HasMin: true, HasMax: true, Unit: "T", Note: "1T-2T"}
		},
	},
	{
		ID: FieldRFC, Label: "tRFC", Unit: "cycles", Group: GroupSecondary, Kind: KindInt,
		Default: "?", Step: 1, Scale: 1, Desc: "Refresh cycle time in cycles.",
		Key: config.KeyRFC, DetectLabel: "tRFC", Bounds: rfcBounds,
	},
	{
		ID: FieldVoltage, Label: "DRAM Voltage", Unit: "V", Group: GroupVoltage, Kind: KindFloat,
		Default: "?", Step: 0.01, Desc: "Requested memory rail voltage.",
		Key: config.KeyVoltage, DetectLabel: "DRAM Voltage",
		Bounds: func(ValidationContext) Bounds {
			return Bounds{Min: 1.35, Max: 1.65, HasMin: true, HasMax: true, Unit: "V", Note: "1.35-1.65 V",
				Low: SeverityDanger, High: SeverityDanger}
		},
		Format: func(raw string) string {
			v, ok := parseNumber(raw)
			if !ok {
				return raw
			}
			return fmt.Sprintf("%.2f V", v)
		},
	},
}

// Fields returns the canonical field list in display order.
func Fields() []Field {
	out := make([]Field, len(fields))
	copy(out, fields)
	return out
}

// Groups returns the group render order.
func Groups() []Group {
	return []Group{GroupGeneral, GroupPrimary, GroupSecondary, GroupVoltage}
}

// LookupField returns the field definition for id.
func LookupField(id FieldID) (Field, bool) {
	for _, f := range fields {
		if f.ID == id {
			return f, true
		}
	}
	return Field{}, false
}

// groupFields returns the field indices belonging to group, in order.
func groupFields(g Group, list []Field) []int {
	var out []int
	for i, f := range list {
		if f.Group == g {
			out = append(out, i)
		}
	}
	return out
}

/* --------------- Bound functions --------------- */

// minCycles converts a nanosecond budget to cycles at freq MT/s, rounded up.
func minCycles(freq, ns float64) float64 {
	return math.Ceil(freq * ns / 2000.0)
}

// primaryBounds covers tCL, tRCD and tRP: a 10 ns floor and an 18 cycle cap.
func primaryBounds(ctx ValidationContext) Bounds {
	freq, ok := ctx.Float(FieldFreq)
	if !ok || freq <= 0 {
		return Bounds{}
	}
	return Bounds{Min: math.Max(5, minCycles(freq, 10)), Max: 18, HasMin: true, HasMax: true,
		Unit: "cycles", Note: "10 ns floor"}
}

// rasBounds covers tRAS: at least tCL+tRCD. There is deliberately no
// nanosecond floor: real XMP kits (16-18-18-38 at 3200 MT/s) run tRAS below
// the JEDEC 32 ns device minimum, so a nanosecond floor would flag stock
// profiles as unsafe. The hard rule is the tCL+tRCD dependency, which is also
// checked as a derived rule.
func rasBounds(ctx ValidationContext) Bounds {
	min := 15.0
	if cl, ok := ctx.Float(FieldCL); ok {
		if rcd, ok := ctx.Float(FieldRCD); ok {
			min = math.Max(min, cl+rcd)
		}
	}
	return Bounds{Min: min, Max: 63, HasMin: true, HasMax: true, Unit: "cycles", Note: ">=tCL+tRCD"}
}

// rfcBounds covers tRFC: a 160 ns floor.
func rfcBounds(ctx ValidationContext) Bounds {
	freq, ok := ctx.Float(FieldFreq)
	if !ok || freq <= 0 {
		return Bounds{}
	}
	return Bounds{Min: minCycles(freq, 160), Max: 1023, HasMin: true, HasMax: true,
		Unit: "cycles", Note: "160 ns floor"}
}

/* --------------- Parsing and formatting helpers --------------- */

// parseNumber extracts the leading numeric prefix of a value ("2T" -> 2,
// "1.35V" -> 1.35, "1600" -> 1600, "?" -> not ok).
func parseNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "?" {
		return 0, false
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, true
	}
	var b strings.Builder
	dot := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' && !dot:
			b.WriteRune(r)
			dot = true
		default:
			goto done
		}
	}
done:
	if b.Len() == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// formatNumber renders a bound value without a trailing ".0".
func formatNumber(v float64) string {
	if v == math.Trunc(v) {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// formatValue renders a stored value for the Detected/Draft columns.
func formatValue(f Field, raw string) string {
	if f.Format == nil {
		return raw
	}
	return f.Format(raw)
}
