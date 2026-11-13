// Package config holds shared leaf types for MemStation: profile snapshots
// and the canonical field keys. It has no dependencies on other packages in
// this repository so it can be imported anywhere without creating cycles.
package config

import (
	"strconv"
	"time"
)

// Canonical field keys. Values are stored as raw display strings ("2T",
// "1.35", "1600") in Snapshot/Profile; parsing happens in helpers, not in
// every renderer.
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

// Snapshot is a point-in-time capture of configuration values.
type Snapshot struct {
	Values     map[string]string `json:"values"` // raw display values
	CapturedAt time.Time         `json:"captured_at"`
	Source     string            `json:"source"` // detected|applied|draft
}

// Profile is a named, persisted set of values.
type Profile struct {
	Name      string            `json:"name"`
	Values    map[string]string `json:"values"`
	UpdatedAt time.Time         `json:"updated_at"`
	Source    string            `json:"source,omitempty"`
}

// Float returns the numeric value for key, if it parses as a float.
func (s Snapshot) Float(key string) (float64, bool) {
	v, ok := s.Values[key]
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// FreqMT returns the DRAM frequency in MT/s (0 if unknown).
func (s Snapshot) FreqMT() float64 {
	f, _ := s.Float(KeyFreq)
	return f
}

// Channels returns the detected channel count (0 if unknown).
func (s Snapshot) Channels() int {
	f, _ := s.Float(KeyChannels)
	return int(f)
}

// PeakMBs returns the theoretical peak bandwidth in MB/s (freq * 8 * channels).
func (s Snapshot) PeakMBs() float64 {
	return s.FreqMT() * 8 * float64(s.Channels())
}

// CASNs returns the CAS latency in nanoseconds (0 if unknown).
func (s Snapshot) CASNs() float64 {
	f := s.FreqMT()
	if f <= 0 {
		return 0
	}
	cl, ok := s.Float(KeyCL)
	if !ok {
		return 0
	}
	return 2000 * cl / f
}

// Clone returns a deep copy of the snapshot's values map.
func (s Snapshot) Clone() Snapshot {
	c := Snapshot{
		Values:     make(map[string]string, len(s.Values)),
		CapturedAt: s.CapturedAt,
		Source:     s.Source,
	}
	for k, v := range s.Values {
		c.Values[k] = v
	}
	return c
}
