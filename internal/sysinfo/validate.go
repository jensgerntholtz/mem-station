package sysinfo

import (
	"fmt"
	"strings"
)

// Issue is one validation finding for a field. Delta is the signed margin in
// the field's unit: negative means below the safe minimum, positive above the
// safe maximum (or headroom inside it). Zero means "no measurable margin".
type Issue struct {
	FieldID  FieldID
	Severity Severity
	Message  string
	Delta    float64
}

// Validate checks every field plus the derived timing rules against the given
// draft values. Fields without a known value report "waiting for detection"
// instead of a made-up range.
func Validate(ctx ValidationContext) []Issue {
	var out []Issue
	for _, f := range fields {
		if iss, ok := validateField(f, ctx); ok {
			out = append(out, iss)
		}
	}
	return append(out, derivedIssues(ctx)...)
}

// validateField checks one field against its enum options or safe range.
func validateField(f Field, ctx ValidationContext) (Issue, bool) {
	raw := strings.TrimSpace(ctx.Value(f.ID))
	if raw == "" || raw == "?" {
		return Issue{FieldID: f.ID, Severity: SeverityInfo, Message: "waiting for detection"}, true
	}
	if f.Kind == KindEnum {
		for _, opt := range f.Options {
			if strings.EqualFold(opt, raw) {
				return Issue{}, false
			}
		}
		return Issue{FieldID: f.ID, Severity: SeverityDanger,
			Message: "must be " + strings.Join(f.Options, " or ")}, true
	}
	if _, ok := parseNumber(raw); !ok {
		return Issue{FieldID: f.ID, Severity: SeverityWarn, Message: "not a number"}, true
	}
	if f.Bounds == nil {
		return Issue{}, false
	}
	b := f.Bounds(ctx)
	v, ok := parseNumber(raw)
	if !ok || !b.Known() {
		return Issue{}, false
	}
	switch {
	case b.HasMin && v < b.Min:
		return Issue{FieldID: f.ID, Severity: b.severityBelow(), Delta: v - b.Min,
			Message: "below " + formatNumber(b.Min) + " " + b.Unit}, true
	case b.HasMax && v > b.Max:
		return Issue{FieldID: f.ID, Severity: b.severityAbove(), Delta: v - b.Max,
			Message: "above " + formatNumber(b.Max) + " " + b.Unit}, true
	}
	return Issue{}, false
}

// derivedIssues implements the cross-field rules that no single field can
// check on its own. tRFC in nanoseconds and tRC are display-only metrics (see
// derivedLines): a nanosecond floor for either would flag stock XMP profiles,
// which run tRAS+tRP below the JEDEC device minimum by design.
func derivedIssues(ctx ValidationContext) []Issue {
	var out []Issue

	cl, okCL := ctx.Float(FieldCL)
	rcd, okRCD := ctx.Float(FieldRCD)
	ras, okRAS := ctx.Float(FieldRAS)

	// tRAS must stay active long enough to cover CAS latency plus RAS-to-CAS.
	if okCL && okRCD && okRAS {
		if min := cl + rcd; ras < min {
			out = append(out, Issue{FieldID: FieldRAS, Severity: SeverityDanger,
				Message: fmt.Sprintf("tRAS below tCL+tRCD (%s)", formatNumber(min)),
				Delta:   ras - min})
		}
	}
	return out
}

// cyclesToNs converts a cycle count to nanoseconds at freq MT/s.
func cyclesToNs(cycles, freq float64) float64 {
	if freq <= 0 {
		return 0
	}
	return cycles * 2000 / freq
}

// margin returns the signed distance between v and the nearest safe bound, in
// the field's unit, and whether the field has a bound to measure against.
func margin(b Bounds, v float64) (float64, bool) {
	switch {
	case b.HasMin && v < b.Min:
		return v - b.Min, true
	case b.HasMax && v > b.Max:
		return v - b.Max, true
	case b.HasMin:
		return v - b.Min, true
	case b.HasMax:
		return v - b.Max, true
	}
	return 0, false
}

// worstIssue returns the most severe issue for id, if any.
func worstIssue(issues []Issue, id FieldID) (Issue, bool) {
	var found Issue
	ok := false
	for _, iss := range issues {
		if iss.FieldID != id {
			continue
		}
		if !ok || iss.Severity > found.Severity {
			found, ok = iss, true
		}
	}
	return found, ok
}
