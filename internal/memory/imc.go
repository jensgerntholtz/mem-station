package memory

// IMCTimings holds IMC register timing values for one memory channel.
// Register layout is Haswell-specific (4th Gen Core, MCHBAR offsets).
type IMCTimings struct {
	Channel int
	RawDBP  uint32
	RawRAP  uint32
	RawRWP  uint32
	RawOTH  uint32
	RawOTH2 uint32
	RawRFTP uint32

	// TC_DBP (0x4000) — primary timings for cross-reference with SPD
	TRAS   int
	TRP    int
	TRCD   int
	TRDPRE int // related to tRTP
	TWRPRE int // = tCWL + BL/2 + tWR

	// TC_RAP (0x4004) — secondary from IMC
	TRRDsg int
	TRRDdg int
	TRTP   int
	TCKE   int
	TWTR   int

	// TC_RWPDEN (0x4008) — turnaround read-to-read, write-to-write
	TRDRDsg int
	TRDRDdg int
	TRDRDdr int
	TRDRDdd int
	TWRWRsg int
	TWRWRdg int

	// TC_OTHP (0x400C) — turnaround write-to-write cross, write-to-read
	TWRWRdr int
	TWRWRdd int
	TWRRDsg int
	TWRRDdg int
	TWRRDdr int
	TWRRDdd int

	// TC_OTHP2 (0x4010) — turnaround read-to-write
	TRDWRsg int
	TRDWRdg int
	TRDWRdr int
	TRDWRdd int

	// TC_RFTP (0x423C) — refresh
	TREFI int
	TRFC  int
}
