package memory

import "testing"

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
