// Package config holds the canonical field keys and snapshot/profile value
// containers shared by the store, tabs, and runners.
package config

import (
	"strconv"
	"strings"
	"time"
)

// Canonical field keys. The UI may use display labels, but persisted snapshots
// and profiles always use these keys.
const (
	KeyChannels = "channels"
	KeyRatio    = "dram_ratio"
	KeyFreq     = "dram_freq"
	KeyCL       = "tCL"
	KeyRCD      = "tRCD"
	KeyRP       = "tRP"
	KeyRAS      = "tRAS"
	KeyCR       = "command_rate"
	KeyRFC      = "tRFC"
	KeyVoltage  = "dram_voltage"
)

// Keys lists every canonical configuration key, in display order. It is the
// allow-list an imported profile is checked against.
var Keys = []string{
	KeyChannels, KeyRatio, KeyFreq, KeyCL, KeyRCD, KeyRP, KeyRAS, KeyCR, KeyRFC, KeyVoltage,
}

// IsKey reports whether k is a canonical configuration key.
func IsKey(k string) bool {
	for _, want := range Keys {
		if k == want {
			return true
		}
	}
	return false
}

// Snapshot is an immutable-ish point-in-time copy of configuration values.
type Snapshot struct {
	Values     map[string]string `json:"values"` // raw display values
	CapturedAt time.Time         `json:"captured_at"`
	Source     string            `json:"source"` // detected|applied|draft
}

// Profile is a named, persisted configuration.
type Profile struct {
	Name      string            `json:"name"`
	Values    map[string]string `json:"values"`
	UpdatedAt time.Time         `json:"updated_at"`
	Source    string            `json:"source,omitempty"`
}

// Float returns the parsed numeric value for key. It accepts bare numbers and
// values with a trailing unit such as "2T" or "1.35V".
func (s Snapshot) Float(key string) (float64, bool) {
	return parseValue(s.Values[key])
}

// FreqMT returns the DRAM frequency in MT/s, or 0 if unknown.
func (s Snapshot) FreqMT() float64 {
	v, _ := s.Float(KeyFreq)
	return v
}

// Channels returns the number of memory channels, or 0 if unknown.
func (s Snapshot) Channels() int {
	v, _ := s.Float(KeyChannels)
	return int(v)
}

// PeakMBs returns the theoretical peak bandwidth in MB/s.
func (s Snapshot) PeakMBs() float64 {
	return s.FreqMT() * 8 * float64(s.Channels())
}

// CASNs returns the CAS latency in nanoseconds, or 0 if unknown.
func (s Snapshot) CASNs() float64 {
	freq := s.FreqMT()
	cl, ok := s.Float(KeyCL)
	if !ok || freq <= 0 {
		return 0
	}
	return 2000 * cl / freq
}

// Clone returns a deep copy of the snapshot.
func (s Snapshot) Clone() Snapshot {
	out := Snapshot{CapturedAt: s.CapturedAt, Source: s.Source}
	if s.Values != nil {
		out.Values = make(map[string]string, len(s.Values))
		for k, v := range s.Values {
			out.Values[k] = v
		}
	}
	return out
}

func parseValue(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "?" {
		return 0, false
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, true
	}
	// Fall back to the leading numeric prefix for values such as "2T" or "1.35V".
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
