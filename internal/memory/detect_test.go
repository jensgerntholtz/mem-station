package memory

import (
	"strings"
	"testing"
)

// TestSizeLabel checks that module sizes render with exactly one unit, whether
// dmidecode ("4096 MB") or decode-dimms (bare number) produced the value.
func TestSizeLabel(t *testing.T) {
	cases := map[string]string{
		"4096 MB": "4096 MB",
		"16384":   "16384 MB",
		" 8192 ":  "8192 MB",
		"16 GB":   "16 GB",
		"":        "",
	}
	for in, want := range cases {
		got := SPDInfo{SizeMB: in}.SizeLabel()
		if got != want {
			t.Errorf("SizeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSupportedFrequencies checks SPD speed parsing is sorted, de-duplicated,
// and only reports values that actually appeared.
func TestSupportedFrequencies(t *testing.T) {
	s := SPDInfo{
		ModuleSpeed: "DDR4-3200",
		TimingsBySpeed: []string{
			"DDR4-3600 18-20-20-40",
			"DDR4-2133 15-15-15-36",
			"DDR4-3200 16-18-18-38",
		},
	}
	got := strings.Join(s.SupportedFrequencies(), ",")
	if got != "2133,3200,3600" {
		t.Errorf("SupportedFrequencies = %q, want 2133,3200,3600", got)
	}
	if got := len((SPDInfo{}).SupportedFrequencies()); got != 0 {
		t.Errorf("empty SPD produced %d frequencies", got)
	}
}
