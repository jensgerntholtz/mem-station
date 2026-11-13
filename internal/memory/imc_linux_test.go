//go:build linux

package memory

import "testing"

// TestParseCPUInfo checks the CPU identity comes from the first processor block
// and does not stop at the leading processor number line.
func TestParseCPUInfo(t *testing.T) {
	data := `processor	: 0
vendor_id	: GenuineIntel
cpu family	: 6
model		: 60
model name	: Intel(R) Core(TM) i5-4670K CPU @ 3.40GHz

processor	: 1
vendor_id	: GenuineIntel
cpu family	: 6
model		: 62
model name	: ignored
`
	cpu := parseCPUInfo(data)
	if cpu.Vendor != "GenuineIntel" || cpu.Family != 6 || cpu.Model != 60 {
		t.Errorf("cpu = %+v", cpu)
	}
	if cpu.Name != "Intel(R) Core(TM) i5-4670K CPU @ 3.40GHz" {
		t.Errorf("name = %q", cpu.Name)
	}
}
