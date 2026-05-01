package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
)

// tertiaryTimings holds IMC register timing values for one memory channel.
// Register layout is Haswell-specific (4th Gen Core, MCHBAR offsets).
type tertiaryTimings struct {
	channel int
	rawDBP  uint32
	rawRAP  uint32
	rawRWP  uint32
	rawOTH  uint32
	rawOTH2 uint32
	rawRFTP uint32
	// TC_DBP (0x4000) — primary timings for cross-reference with SPD
	tRAS   int
	tRP    int
	tRCD   int
	tRDPRE int // related to tRTP
	tWRPRE int // = tCWL + BL/2 + tWR
	// TC_RAP (0x4004) — secondary from IMC
	tRRDsg int
	tRRDdg int
	tRTP   int
	tCKE   int
	tWTR   int
	// TC_RWPDEN (0x4008) — turnaround read-to-read, write-to-write
	tRDRDsg int
	tRDRDdg int
	tRDRDdr int
	tRDRDdd int
	tWRWRsg int
	tWRWRdg int
	// TC_OTHP (0x400C) — turnaround write-to-write cross, write-to-read
	tWRWRdr int
	tWRWRdd int
	tWRRDsg int
	tWRRDdg int
	tWRRDdr int
	tWRRDdd int
	// TC_OTHP2 (0x4010) — turnaround read-to-write
	tRDWRsg int
	tRDWRdg int
	tRDWRdr int
	tRDWRdd int
	// TC_RFTP (0x423C) — refresh
	tREFI int
	tRFC  int
}

func readMCHBARTimings() ([]tertiaryTimings, error) {
	mchbar, err := getMCHBARBase()
	if err != nil {
		return nil, err
	}

	f, err := os.Open("/dev/mem")
	if err != nil {
		return nil, fmt.Errorf("/dev/mem: %v (need root; may need iomem=relaxed)", err)
	}
	defer f.Close()

	var results []tertiaryTimings
	for ch := 0; ch < 2; ch++ {
		chBase := mchbar + 0x4000 + uint64(ch)*0x400

		data, off, err := mmapRegion(f, chBase, 0x240)
		if err != nil {
			continue
		}

		r := func(regOff int) uint32 {
			return binary.LittleEndian.Uint32(data[off+regOff:])
		}

		dbp := r(0x00)
		rap := r(0x04)
		rwp := r(0x08)
		oth := r(0x0C)
		oth2 := r(0x10)
		rftp := r(0x23C)

		t := tertiaryTimings{
			channel: ch,
			rawDBP:  dbp,
			rawRAP:  rap,
			rawRWP:  rwp,
			rawOTH:  oth,
			rawOTH2: oth2,
			rawRFTP: rftp,

			tRAS:   extractBits(dbp, 0, 8),
			tRDPRE: extractBits(dbp, 8, 4),
			tWRPRE: extractBits(dbp, 12, 6),
			tRP:    extractBits(dbp, 18, 5),
			tRCD:   extractBits(dbp, 23, 5),

			tRRDsg: extractBits(rap, 0, 4),
			tRRDdg: extractBits(rap, 4, 4),
			tRTP:   extractBits(rap, 8, 4),
			tCKE:   extractBits(rap, 12, 5),
			tWTR:   extractBits(rap, 17, 5),

			tRDRDsg: extractBits(rwp, 0, 5),
			tRDRDdg: extractBits(rwp, 5, 5),
			tRDRDdr: extractBits(rwp, 10, 5),
			tRDRDdd: extractBits(rwp, 15, 5),
			tWRWRsg: extractBits(rwp, 20, 5),
			tWRWRdg: extractBits(rwp, 25, 5),

			tWRWRdr: extractBits(oth, 0, 5),
			tWRWRdd: extractBits(oth, 5, 5),
			tWRRDsg: extractBits(oth, 10, 6),
			tWRRDdg: extractBits(oth, 16, 6),
			tWRRDdr: extractBits(oth, 22, 5),
			tWRRDdd: extractBits(oth, 27, 5),

			tRDWRsg: extractBits(oth2, 0, 6),
			tRDWRdg: extractBits(oth2, 6, 6),
			tRDWRdr: extractBits(oth2, 12, 6),
			tRDWRdd: extractBits(oth2, 18, 6),

			tREFI: extractBits(rftp, 0, 16),
			tRFC:  extractBits(rftp, 16, 9),
		}

		syscall.Munmap(data)
		results = append(results, t)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no IMC timing data read")
	}
	return results, nil
}

// dumpMCHBARRaw prints raw register hex values for debugging.
func dumpMCHBARRaw() {
	// Step 1: PCI config space raw read
	pciPath := "/sys/bus/pci/devices/0000:00:00.0/config"
	pf, err := os.Open(pciPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ open %s: %v\n", pciPath, err)
		os.Exit(1)
	}
	buf := make([]byte, 8)
	n, err := pf.ReadAt(buf, 0x48)
	pf.Close()
	fmt.Printf("PCI config @0x48: read %d bytes, err=%v\n", n, err)
	fmt.Printf("  raw bytes: %02X\n", buf)
	raw64 := binary.LittleEndian.Uint64(buf)
	fmt.Printf("  uint64:    0x%016X\n", raw64)
	fmt.Printf("  bit0 (en): %d\n", raw64&1)
	base := raw64 & ^uint64(0x3FFF)
	fmt.Printf("  base:      0x%X\n\n", base)

	if base == 0 {
		fmt.Fprintln(os.Stderr, "✗ MCHBAR base = 0, cannot proceed")
		os.Exit(1)
	}

	// Step 2: mmap and dump raw registers per channel
	f, err := os.Open("/dev/mem")
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ /dev/mem: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	regNames := []struct {
		name   string
		offset int
	}{
		{"TC_DBP", 0x00},
		{"TC_RAP", 0x04},
		{"TC_RWPDEN", 0x08},
		{"TC_OTHP", 0x0C},
		{"TC_OTHP2", 0x10},
		{"TC_RFTP", 0x23C},
	}

	for ch := 0; ch < 2; ch++ {
		chBase := base + 0x4000 + uint64(ch)*0x400
		fmt.Printf("── Channel %d  (phys addr 0x%X) ──\n", ch, chBase)

		data, off, err := mmapRegion(f, chBase, 0x240)
		if err != nil {
			fmt.Printf("  ✗ mmap failed: %v\n\n", err)
			continue
		}

		for _, reg := range regNames {
			val := binary.LittleEndian.Uint32(data[off+reg.offset:])
			fmt.Printf("  %-12s (+0x%03X) = 0x%08X  (%032b)\n", reg.name, reg.offset, val, val)
		}

		// Also dump first 32 bytes raw hex for inspection
		fmt.Printf("  first 32 bytes: ")
		end := off + 32
		if end > len(data) {
			end = len(data)
		}
		for i := off; i < end; i++ {
			fmt.Printf("%02X ", data[i])
		}
		fmt.Println()

		syscall.Munmap(data)
		fmt.Println()
	}
}

func getMCHBARBase() (uint64, error) {
	f, err := os.Open("/sys/bus/pci/devices/0000:00:00.0/config")
	if err != nil {
		return 0, fmt.Errorf("PCI config: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 8)
	if _, err := f.ReadAt(buf, 0x48); err != nil {
		return 0, fmt.Errorf("read MCHBAR offset: %v", err)
	}

	mchbar := binary.LittleEndian.Uint64(buf)
	if mchbar&1 == 0 {
		return 0, fmt.Errorf("MCHBAR not enabled")
	}
	// Clear low 14 bits (enable + reserved), keep base address
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

func extractBits(val uint32, start, count int) int {
	return int((val >> uint(start)) & ((1 << uint(count)) - 1))
}
