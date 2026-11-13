package memory

import "testing"

// setBits places value into a 32-bit register at the given bitfield.
func setBits(reg uint32, start, width int, value uint32) uint32 {
	mask := (uint32(1)<<uint(width) - 1) << uint(start)
	return (reg &^ mask) | ((value << uint(start)) & mask)
}

// TestLookupIMCPlatform checks the verified CPU families are selected and that
// an unknown model stays unsupported instead of using Haswell offsets.
func TestLookupIMCPlatform(t *testing.T) {
	if got := lookupIMCPlatform("GenuineIntel", 6, 60); got != haswellIMC {
		t.Errorf("Haswell model 60 selected %v", got)
	}
	if got := lookupIMCPlatform("GenuineIntel", 6, 58); got != sandyBridgeIMC {
		t.Errorf("Ivy Bridge model 58 selected %v", got)
	}
	if got := lookupIMCPlatform("GenuineIntel", 6, 78); got != nil {
		t.Errorf("unverified Skylake model 78 selected %v", got)
	}
	if got := lookupIMCPlatform("AuthenticAMD", 6, 60); got != nil {
		t.Errorf("non-Intel vendor selected %v", got)
	}
}

// TestDecodeHaswellLayout checks the Haswell bitfields used by TC_BANK and the
// TC_BANK_RANK registers.
func TestDecodeHaswellLayout(t *testing.T) {
	dbp := uint32(0)
	dbp = setBits(dbp, 0, 5, 16)  // tRCD
	dbp = setBits(dbp, 5, 5, 18)  // tRP
	dbp = setBits(dbp, 10, 6, 38) // tRAS
	dbp = setBits(dbp, 16, 4, 6)  // tRDPRE
	dbp = setBits(dbp, 20, 6, 20) // tWRPRE
	dbp = setBits(dbp, 26, 4, 6)  // tRRD

	rap := uint32(0)
	rap = setBits(rap, 0, 4, 6)  // tCKE
	rap = setBits(rap, 4, 8, 24) // tFAW
	rap = setBits(rap, 12, 3, 4) // tRDRD_sr
	rap = setBits(rap, 15, 4, 5) // tRDRD_dr
	rap = setBits(rap, 19, 4, 6) // tRDRD_dd

	rwp := uint32(0)
	rwp = setBits(rwp, 0, 6, 8)  // tWRRD_sr
	rwp = setBits(rwp, 6, 4, 4)  // tWRRD_dr
	rwp = setBits(rwp, 10, 4, 5) // tWRRD_dd
	rwp = setBits(rwp, 14, 3, 4) // tWRWR_sr
	rwp = setBits(rwp, 17, 4, 5) // tWRWR_dr
	rwp = setBits(rwp, 21, 4, 6) // tWRWR_dd

	othp := uint32(0)
	othp = setBits(othp, 14, 5, 7) // tRDWR_sr
	othp = setBits(othp, 19, 5, 8) // tRDWR_dr
	othp = setBits(othp, 24, 5, 9) // tRDWR_dd

	bankD := uint32(0)
	bankD = setBits(bankD, 0, 5, 16) // tAA
	bankD = setBits(bankD, 5, 5, 9)  // tCWL

	rftp := uint32(0)
	rftp = setBits(rftp, 0, 16, 7800) // tREFI
	rftp = setBits(rftp, 16, 9, 350)  // tRFC

	timings := decodeIMC(haswellIMC, 0, map[string]uint32{
		"DBP": dbp, "RAP": rap, "RWP": rwp, "OTHP": othp,
		"BANK_D": bankD, "RFTP": rftp,
	})
	if timings.TRCD != 16 || timings.TRP != 18 || timings.TRAS != 38 {
		t.Errorf("primary = tRCD %d tRP %d tRAS %d", timings.TRCD, timings.TRP, timings.TRAS)
	}
	if timings.TRDPRE != 6 || timings.TWRPRE != 20 || timings.TRRD != 6 {
		t.Errorf("bank = tRDPRE %d tWRPRE %d tRRD %d", timings.TRDPRE, timings.TWRPRE, timings.TRRD)
	}
	if timings.TCKE != 6 || timings.TFAW != 24 || timings.TRDRDsg != 4 || timings.TRDRDdg != 5 || timings.TRDRDdd != 6 {
		t.Errorf("rank A = %+v", timings)
	}
	if timings.TWRRDsg != 8 || timings.TWRRDdg != 4 || timings.TWRRDdd != 5 {
		t.Errorf("tWRRD = %d/%d/%d", timings.TWRRDsg, timings.TWRRDdg, timings.TWRRDdd)
	}
	if timings.TWRWRsg != 4 || timings.TWRWRdg != 5 || timings.TWRWRdd != 6 {
		t.Errorf("tWRWR = %d/%d/%d", timings.TWRWRsg, timings.TWRWRdg, timings.TWRWRdd)
	}
	if timings.TRDWRsg != 7 || timings.TRDWRdg != 8 || timings.TRDWRdd != 9 {
		t.Errorf("tRDWR = %d/%d/%d", timings.TRDWRsg, timings.TRDWRdg, timings.TRDWRdd)
	}
	if timings.TAA != 16 || timings.TCWL != 9 || timings.TREFI != 7800 || timings.TRFC != 350 {
		t.Errorf("bank D/refresh = tAA %d tCWL %d tREFI %d tRFC %d",
			timings.TAA, timings.TCWL, timings.TREFI, timings.TRFC)
	}
}

// TestDecodeSandyBridgeLayout checks the TC_DBP/TC_RAP bitfields used by
// Sandy Bridge and Ivy Bridge.
func TestDecodeSandyBridgeLayout(t *testing.T) {
	dbp := uint32(0)
	dbp = setBits(dbp, 0, 4, 9)   // tRCD
	dbp = setBits(dbp, 4, 4, 10)  // tRP
	dbp = setBits(dbp, 8, 4, 11)  // tAA
	dbp = setBits(dbp, 12, 4, 8)  // tCWL
	dbp = setBits(dbp, 16, 8, 28) // tRAS

	rap := uint32(0)
	rap = setBits(rap, 0, 4, 4)   // tRRD
	rap = setBits(rap, 4, 4, 5)   // tRTP
	rap = setBits(rap, 8, 4, 6)   // tCKE
	rap = setBits(rap, 12, 4, 7)  // tWTR
	rap = setBits(rap, 16, 8, 20) // tFAW
	rap = setBits(rap, 24, 5, 12) // tWR

	rwp := uint32(0)
	rwp = setBits(rwp, 0, 3, 4)  // tRRDR
	rwp = setBits(rwp, 4, 3, 5)  // tRRDD
	rwp = setBits(rwp, 8, 3, 6)  // tWWDR
	rwp = setBits(rwp, 12, 3, 7) // tWWDD
	rwp = setBits(rwp, 16, 3, 4) // tRWDRDD
	rwp = setBits(rwp, 20, 3, 5) // tWRDRDD
	rwp = setBits(rwp, 24, 3, 6) // tRWSR

	rftp := uint32(0)
	rftp = setBits(rftp, 0, 16, 6240) // tREFI
	rftp = setBits(rftp, 16, 9, 208)  // tRFC

	timings := decodeIMC(sandyBridgeIMC, 1, map[string]uint32{
		"DBP": dbp, "RAP": rap, "RWP": rwp, "RFTP": rftp,
	})
	if timings.Channel != 1 || timings.Platform != sandyBridgeIMC.name {
		t.Errorf("identity = %+v", timings)
	}
	if timings.TRCD != 9 || timings.TRP != 10 || timings.TAA != 11 || timings.TCWL != 8 || timings.TRAS != 28 {
		t.Errorf("primary = %+v", timings)
	}
	if timings.TRRD != 4 || timings.TRTP != 5 || timings.TCKE != 6 || timings.TWTR != 7 || timings.TFAW != 20 || timings.TWR != 12 {
		t.Errorf("secondary = %+v", timings)
	}
	if timings.TRRDR != 4 || timings.TRRDD != 5 || timings.TWWDR != 6 || timings.TWWDD != 7 {
		t.Errorf("turnaround = %+v", timings)
	}
	if timings.TRWDRDD != 4 || timings.TWRDRDD != 5 || timings.TRWSR != 6 {
		t.Errorf("cross-rank turnaround = %+v", timings)
	}
	if timings.TREFI != 6240 || timings.TRFC != 208 {
		t.Errorf("refresh = tREFI %d tRFC %d", timings.TREFI, timings.TRFC)
	}
}
