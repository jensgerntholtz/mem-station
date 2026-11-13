//go:build linux

package memory

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
)

// ReadMCHBARTimings reads tertiary timing values from the MCHBAR registers for
// each memory channel. Requires root and iomem=relaxed.
func ReadMCHBARTimings() ([]IMCTimings, error) {
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

		t := IMCTimings{
			Channel: ch,
			RawDBP:  dbp,
			RawRAP:  rap,
			RawRWP:  rwp,
			RawOTH:  oth,
			RawOTH2: oth2,
			RawRFTP: rftp,

			TRAS:   extractBits(dbp, 0, 8),
			TRDPRE: extractBits(dbp, 8, 4),
			TWRPRE: extractBits(dbp, 12, 6),
			TRP:    extractBits(dbp, 18, 5),
			TRCD:   extractBits(dbp, 23, 5),

			TRRDsg: extractBits(rap, 0, 4),
			TRRDdg: extractBits(rap, 4, 4),
			TRTP:   extractBits(rap, 8, 4),
			TCKE:   extractBits(rap, 12, 5),
			TWTR:   extractBits(rap, 17, 5),

			TRDRDsg: extractBits(rwp, 0, 5),
			TRDRDdg: extractBits(rwp, 5, 5),
			TRDRDdr: extractBits(rwp, 10, 5),
			TRDRDdd: extractBits(rwp, 15, 5),
			TWRWRsg: extractBits(rwp, 20, 5),
			TWRWRdg: extractBits(rwp, 25, 5),

			TWRWRdr: extractBits(oth, 0, 5),
			TWRWRdd: extractBits(oth, 5, 5),
			TWRRDsg: extractBits(oth, 10, 6),
			TWRRDdg: extractBits(oth, 16, 6),
			TWRRDdr: extractBits(oth, 22, 5),
			TWRRDdd: extractBits(oth, 27, 5),

			TRDWRsg: extractBits(oth2, 0, 6),
			TRDWRdg: extractBits(oth2, 6, 6),
			TRDWRdr: extractBits(oth2, 12, 6),
			TRDWRdd: extractBits(oth2, 18, 6),

			TREFI: extractBits(rftp, 0, 16),
			TRFC:  extractBits(rftp, 16, 9),
		}

		_ = syscall.Munmap(data)
		results = append(results, t)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no IMC timing data read")
	}
	return results, nil
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
