package memory

// IMCTimings holds the timing values decoded from one memory channel's
// integrated memory controller registers. The same result type is used for
// every supported platform; Platform names the register layout that produced
// the values.
type IMCTimings struct {
	Platform string         `json:"platform,omitempty"`
	Channel  int            `json:"channel"`
	Values   map[string]int `json:"values,omitempty"`

	RawDBP  uint32 `json:"raw_dbp,omitempty"`
	RawRAP  uint32 `json:"raw_rap,omitempty"`
	RawRWP  uint32 `json:"raw_rwp,omitempty"`
	RawOTH  uint32 `json:"raw_oth,omitempty"`
	RawOTH2 uint32 `json:"raw_oth2,omitempty"`
	RawRFTP uint32 `json:"raw_rftp,omitempty"`

	// Primary timings.
	TRAS   int `json:"tras"`
	TRP    int `json:"trp"`
	TRCD   int `json:"trcd"`
	TRDPRE int `json:"trdpre"`
	TWRPRE int `json:"twrpre"`
	TRRD   int `json:"trrd"`
	TFAW   int `json:"tfaw"`
	TAA    int `json:"taa"`
	TCWL   int `json:"tcwl"`
	TWR    int `json:"twr"`
	TCMD   int `json:"tcmd"`

	// Secondary timings.
	TRRDsg int `json:"trrd_sg"`
	TRRDdg int `json:"trrd_dg"`
	TRTP   int `json:"trtp"`
	TCKE   int `json:"tcke"`
	TWTR   int `json:"twtr"`

	// Turnaround timings. Platform-specific layouts use different rank
	// combinations, so the raw decoded values are also retained in Values.
	TRDRDsg int `json:"trdrd_sg"`
	TRDRDdg int `json:"trdrd_dg"`
	TRDRDdr int `json:"trdrd_dr"`
	TRDRDdd int `json:"trdrd_dd"`
	TWRWRsg int `json:"twrwr_sg"`
	TWRWRdg int `json:"twrwr_dg"`
	TWRWRdr int `json:"twrwr_dr"`
	TWRWRdd int `json:"twrwr_dd"`

	TWRRDsg int `json:"twrrd_sg"`
	TWRRDdg int `json:"twrrd_dg"`
	TWRRDdr int `json:"twrrd_dr"`
	TWRRDdd int `json:"twrrd_dd"`
	TRDWRsg int `json:"trdwr_sg"`
	TRDWRdg int `json:"trdwr_dg"`
	TRDWRdr int `json:"trdwr_dr"`
	TRDWRdd int `json:"trdwr_dd"`

	// Sandy/Ivy Bridge expose these individually in TC_RWP.
	TRRDR   int `json:"trrdr"`
	TRRDD   int `json:"trrdd"`
	TWWDR   int `json:"twwdr"`
	TWWDD   int `json:"twwdd"`
	TRWDRDD int `json:"trwdrdd"`
	TWRDRDD int `json:"twrdrdd"`
	TRWSR   int `json:"trwsr"`

	TREFI int `json:"trefi"`
	TRFC  int `json:"trfc"`
}

// imcField maps one named timing to a bitfield in a register.
type imcField struct {
	key      string
	register string
	start    int
	width    int
}

// imcPlatform describes the MCHBAR register layout for one CPU family. All
// register offsets are relative to MCHBAR and are repeated per channel with
// channelStride.
type imcPlatform struct {
	name          string
	vendor        string
	family        int
	models        []int
	channels      int
	channelStride uint64
	registers     map[string]uint64
	fields        []imcField
}

// Sandy Bridge and Ivy Bridge share the TC_DBP/TC_RAP/TC_RWP/TC_OTHP layout.
// Offsets and bitfields follow the public Intel MCHBAR documentation as used by
// coreboot's native raminit.
var sandyBridgeIMC = &imcPlatform{
	name:          "Intel Sandy Bridge / Ivy Bridge",
	vendor:        "GenuineIntel",
	family:        6,
	models:        []int{42, 45, 58, 62},
	channels:      2,
	channelStride: 0x400,
	registers: map[string]uint64{
		"DBP":  0x4000,
		"RAP":  0x4004,
		"RWP":  0x4008,
		"OTHP": 0x400c,
		"RFTP": 0x4298,
	},
	fields: []imcField{
		{"tRCD", "DBP", 0, 4},
		{"tRP", "DBP", 4, 4},
		{"tAA", "DBP", 8, 4},
		{"tCWL", "DBP", 12, 4},
		{"tRAS", "DBP", 16, 8},

		{"tRRD", "RAP", 0, 4},
		{"tRTP", "RAP", 4, 4},
		{"tCKE", "RAP", 8, 4},
		{"tWTR", "RAP", 12, 4},
		{"tFAW", "RAP", 16, 8},
		{"tWR", "RAP", 24, 5},
		{"tCMD", "RAP", 30, 2},

		{"tRRDR", "RWP", 0, 3},
		{"tRRDD", "RWP", 4, 3},
		{"tWWDR", "RWP", 8, 3},
		{"tWWDD", "RWP", 12, 3},
		{"tRWDRDD", "RWP", 16, 3},
		{"tWRDRDD", "RWP", 20, 3},
		{"tRWSR", "RWP", 24, 3},

		{"tXPDLL", "OTHP", 0, 5},
		{"tXP", "OTHP", 5, 3},
		{"tAONPD", "OTHP", 8, 4},

		{"tREFI", "RFTP", 0, 16},
		{"tRFC", "RFTP", 16, 9},
	},
}

// Haswell and Broadwell moved the timings into TC_BANK and TC_BANK_RANK_A-D.
// The offsets stay in the same MCHBAR window but the bitfields differ.
var haswellIMC = &imcPlatform{
	name:          "Intel Haswell / Broadwell",
	vendor:        "GenuineIntel",
	family:        6,
	models:        []int{60, 61, 63, 69, 70, 71, 79, 86},
	channels:      2,
	channelStride: 0x400,
	registers: map[string]uint64{
		"DBP":    0x4000,
		"RAP":    0x4004,
		"RWP":    0x4008,
		"OTHP":   0x400c,
		"BANK_D": 0x4014,
		"RFTP":   0x4298,
	},
	fields: []imcField{
		{"tRCD", "DBP", 0, 5},
		{"tRP", "DBP", 5, 5},
		{"tRAS", "DBP", 10, 6},
		{"tRDPRE", "DBP", 16, 4},
		{"tWRPRE", "DBP", 20, 6},
		{"tRRD", "DBP", 26, 4},

		{"tCKE", "RAP", 0, 4},
		{"tFAW", "RAP", 4, 8},
		{"tRDRD_sr", "RAP", 12, 3},
		{"tRDRD_dr", "RAP", 15, 4},
		{"tRDRD_dd", "RAP", 19, 4},

		{"tWRRD_sr", "RWP", 0, 6},
		{"tWRRD_dr", "RWP", 6, 4},
		{"tWRRD_dd", "RWP", 10, 4},
		{"tWRWR_sr", "RWP", 14, 3},
		{"tWRWR_dr", "RWP", 17, 4},
		{"tWRWR_dd", "RWP", 21, 4},

		{"tXPDLL", "OTHP", 0, 6},
		{"tXP", "OTHP", 6, 4},
		{"tAONPD", "OTHP", 10, 4},
		{"tRDWR_sr", "OTHP", 14, 5},
		{"tRDWR_dr", "OTHP", 19, 5},
		{"tRDWR_dd", "OTHP", 24, 5},

		{"tAA", "BANK_D", 0, 5},
		{"tCWL", "BANK_D", 5, 5},

		{"tREFI", "RFTP", 0, 16},
		{"tRFC", "RFTP", 16, 9},
	},
}

// lookupIMCPlatform returns the layout for a CPU identity, or nil when the
// platform has not been verified. Unknown platforms still get SPD/DMI timing
// detection; only the direct MCHBAR register view is unavailable.
func lookupIMCPlatform(vendor string, family, model int) *imcPlatform {
	for _, p := range []*imcPlatform{sandyBridgeIMC, haswellIMC} {
		if p.vendor != vendor || p.family != family {
			continue
		}
		for _, m := range p.models {
			if m == model {
				return p
			}
		}
	}
	return nil
}

// decodeIMC converts raw register values into one channel's timing struct.
func decodeIMC(p *imcPlatform, channel int, raw map[string]uint32) IMCTimings {
	t := IMCTimings{
		Platform: p.name,
		Channel:  channel,
		Values:   make(map[string]int),
		RawDBP:   raw["DBP"],
		RawRAP:   raw["RAP"],
		RawRWP:   raw["RWP"],
		RawOTH:   raw["OTHP"],
		RawOTH2:  raw["BANK_D"],
		RawRFTP:  raw["RFTP"],
	}
	for _, f := range p.fields {
		value := extractBits(raw[f.register], f.start, f.width)
		t.Values[f.key] = value
		applyIMCField(&t, f.key, value)
	}
	return t
}

// applyIMCField copies a named decoded value into the typed fields used by the
// UI and JSON exports.
func applyIMCField(t *IMCTimings, key string, value int) {
	switch key {
	case "tRAS":
		t.TRAS = value
	case "tRP":
		t.TRP = value
	case "tRCD":
		t.TRCD = value
	case "tRDPRE":
		t.TRDPRE = value
	case "tWRPRE":
		t.TWRPRE = value
	case "tRRD":
		t.TRRD = value
	case "tFAW":
		t.TFAW = value
	case "tAA":
		t.TAA = value
	case "tCWL":
		t.TCWL = value
	case "tWR":
		t.TWR = value
	case "tCMD":
		t.TCMD = value

	case "tRTP":
		t.TRTP = value
	case "tCKE":
		t.TCKE = value
	case "tWTR":
		t.TWTR = value

	case "tRDRD_sr":
		t.TRDRDsg = value
	case "tRDRD_dr":
		t.TRDRDdg = value
	case "tRDRD_dd":
		t.TRDRDdd = value
	case "tWRWR_sr":
		t.TWRWRsg = value
	case "tWRWR_dr":
		t.TWRWRdg = value
	case "tWRWR_dd":
		t.TWRWRdd = value
	case "tWRRD_sr":
		t.TWRRDsg = value
	case "tWRRD_dr":
		t.TWRRDdg = value
	case "tWRRD_dd":
		t.TWRRDdd = value
	case "tRDWR_sr":
		t.TRDWRsg = value
	case "tRDWR_dr":
		t.TRDWRdg = value
	case "tRDWR_dd":
		t.TRDWRdd = value

	case "tRRDR":
		t.TRRDR = value
		t.TRDRDsg = value
	case "tRRDD":
		t.TRRDD = value
		t.TRDRDdg = value
	case "tWWDR":
		t.TWWDR = value
		t.TWRWRsg = value
	case "tWWDD":
		t.TWWDD = value
		t.TWRWRdg = value
	case "tRWDRDD":
		t.TRWDRDD = value
		t.TRDWRdg = value
	case "tWRDRDD":
		t.TWRDRDD = value
		t.TWRRDdg = value
	case "tRWSR":
		t.TRWSR = value

	case "tREFI":
		t.TREFI = value
	case "tRFC":
		t.TRFC = value
	}
}

// extractBits returns width bits starting at start.
func extractBits(val uint32, start, count int) int {
	return int((val >> uint(start)) & ((1 << uint(count)) - 1))
}
