//go:build linux

package memory

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// cpuIdentity is the part of /proc/cpuinfo that selects an IMC register layout.
type cpuIdentity struct {
	Vendor string
	Family int
	Model  int
	Name   string
}

// ReadMCHBARTimings reads per-channel IMC timing registers from /dev/mem for a
// verified platform. It needs root, and strict kernels may additionally require
// iomem=relaxed. Callers that do not need raw registers should use the default
// DMI/SPD detection instead.
func ReadMCHBARTimings() ([]IMCTimings, error) {
	platform, cpu, err := currentIMCPlatform()
	if err != nil {
		return nil, err
	}

	mchbar, err := getMCHBARBase()
	if err != nil {
		return nil, err
	}

	f, err := os.Open("/dev/mem")
	if err != nil {
		return nil, fmt.Errorf("/dev/mem: %v (need root; may need iomem=relaxed)", err)
	}
	defer f.Close()

	var results []IMCTimings
	var firstMapErr error
	for ch := 0; ch < platform.channels; ch++ {
		raw := make(map[string]uint32, len(platform.registers))
		for name, offset := range platform.registers {
			addr := mchbar + uint64(ch)*platform.channelStride + offset
			data, pageOff, err := mmapRegion(f, addr, 4)
			if err != nil {
				if firstMapErr == nil {
					firstMapErr = err
				}
				continue
			}
			raw[name] = binary.LittleEndian.Uint32(data[pageOff:])
			_ = syscall.Munmap(data)
		}
		if len(raw) > 0 {
			results = append(results, decodeIMC(platform, ch, raw))
		}
	}

	if len(results) == 0 {
		if firstMapErr != nil {
			return nil, fmt.Errorf("map %s MCHBAR: %v (need root; iomem=relaxed may be required)", cpu.Name, firstMapErr)
		}
		return nil, fmt.Errorf("no IMC timing data read for %s", cpu.Name)
	}
	return results, nil
}

// currentIMCPlatform identifies the CPU and returns its verified layout.
func currentIMCPlatform() (*imcPlatform, cpuIdentity, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return nil, cpuIdentity{}, fmt.Errorf("read /proc/cpuinfo: %v", err)
	}
	cpu := parseCPUInfo(string(data))
	platform := lookupIMCPlatform(cpu.Vendor, cpu.Family, cpu.Model)
	if platform == nil {
		return nil, cpu, fmt.Errorf(
			"IMC register layout is not verified for %s (vendor %q, family %d, model %d); SPD/DMI timings still work",
			cpu.Name, cpu.Vendor, cpu.Family, cpu.Model)
	}
	return platform, cpu, nil
}

// parseCPUInfo extracts the first processor's identity.
func parseCPUInfo(data string) cpuIdentity {
	var cpu cpuIdentity
	for _, line := range strings.Split(data, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "vendor_id":
			cpu.Vendor = value
		case "cpu family":
			cpu.Family, _ = strconv.Atoi(value)
		case "model":
			cpu.Model, _ = strconv.Atoi(value)
		case "model name":
			cpu.Name = value
		case "processor":
			// The first processor block is enough; /proc/cpuinfo repeats the
			// identity for every logical CPU.
			if cpu.Vendor != "" || cpu.Name != "" {
				return cpu
			}
		}
	}
	if cpu.Name == "" {
		cpu.Name = fmt.Sprintf("%s family %d model %d", cpu.Vendor, cpu.Family, cpu.Model)
	}
	return cpu
}

// getMCHBARBase reads the MCHBAR base address from the host bridge's PCI
// configuration space.
func getMCHBARBase() (uint64, error) {
	f, err := os.Open("/sys/bus/pci/devices/0000:00:00.0/config")
	if err != nil {
		return 0, fmt.Errorf("PCI config: %v (need root)", err)
	}
	defer f.Close()

	buf := make([]byte, 8)
	if _, err := f.ReadAt(buf, 0x48); err != nil {
		return 0, fmt.Errorf("read MCHBAR offset: %v (need root)", err)
	}

	mchbar := binary.LittleEndian.Uint64(buf)
	if mchbar&1 == 0 {
		return 0, fmt.Errorf("MCHBAR not enabled")
	}
	// Clear low 14 bits (enable + reserved), keep base address.
	mchbar &= ^uint64(0x3FFF)
	if mchbar == 0 {
		return 0, fmt.Errorf("MCHBAR base is zero")
	}
	return mchbar, nil
}

func mmapRegion(f *os.File, physAddr uint64, size int) ([]byte, int, error) {
	pageSize := uint64(os.Getpagesize())
	pageBase := physAddr & ^(pageSize - 1)
	pageOff := int(physAddr - pageBase)
	mapLen := pageOff + size
	if rem := mapLen % int(pageSize); rem != 0 {
		mapLen += int(pageSize) - rem
	}
	data, err := syscall.Mmap(int(f.Fd()), int64(pageBase), mapLen, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, 0, err
	}
	return data, pageOff, nil
}
