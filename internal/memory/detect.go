package memory

import (
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type SPDInfo struct {
	ModuleManufacturer string
	DRAMManufacturer   string
	PartNumber         string
	ModuleType         string
	MemoryType         string
	ModuleSpeed        string
	SizeMB             string
	Ranks              string
	BankLayout         string
	BusWidth           string
	DeviceWidth        string
	SupportedCL        string
	Voltage            string
	TimingString       string
	TimingsBySpeed     []string
	TWR                string
	TRRD               string
	TRC                string
	TWTR               string
	TRTP               string
	TFAW               string
	TCKmin             string
	DLLOff             string
	TempRange          string
	AutoSR             string
	ModuleHeight       string
	RefCard            string
	NumDIMMs           string
}

// DetectDefaults runs hardware detection and SPD parsing.
func DetectDefaults() (map[string]string, string, *SPDInfo) {
	vals := make(map[string]string)

	out, err := exec.Command("dmidecode", "--type", "17", "--type", "4").CombinedOutput()
	if err != nil {
		return vals, "Auto-detect failed (try: sudo ./mem-station)", nil
	}
	dmi := string(out)

	blocks := strings.Split(dmi, "Memory Device")
	chSet := make(map[string]bool)
	confSpeedRe := regexp.MustCompile(`Configured Memory Speed:\s+(\d+)\s+MT/s`)
	speedRe := regexp.MustCompile(`\bSpeed:\s+(\d+)\s+MT/s`)
	voltRe := regexp.MustCompile(`Configured Voltage:\s+([\d.]+)\s+V`)
	locRe := regexp.MustCompile(`Locator:\s+(\S+)`)

	for _, block := range blocks[1:] {
		if strings.Contains(block, "No Module Installed") {
			continue
		}
		if _, ok := vals["DRAM Freq"]; !ok {
			if m := confSpeedRe.FindStringSubmatch(block); len(m) > 1 {
				vals["DRAM Freq"] = m[1]
			} else if m := speedRe.FindStringSubmatch(block); len(m) > 1 {
				vals["DRAM Freq"] = m[1]
			}
		}
		if _, ok := vals["DRAM Voltage"]; !ok {
			if m := voltRe.FindStringSubmatch(block); len(m) > 1 {
				vals["DRAM Voltage"] = m[1]
			}
		}
		if m := locRe.FindStringSubmatch(block); len(m) > 1 {
			loc := strings.ToUpper(m[1])
			for _, ch := range []string{"A", "B", "C", "D"} {
				if strings.Contains(loc, "CHANNEL"+ch) ||
					strings.Contains(loc, "_"+ch) ||
					strings.HasPrefix(loc, ch) {
					chSet[ch] = true
					break
				}
			}
		}
	}
	if len(chSet) > 0 {
		vals["Channels"] = strconv.Itoa(len(chSet))
	}
	bclkRe := regexp.MustCompile(`External Clock:\s+(\d+)\s+MHz`)
	if m := bclkRe.FindStringSubmatch(dmi); len(m) > 1 {
		bclk, _ := strconv.Atoi(m[1])
		if bclk > 0 {
			if freqStr, ok := vals["DRAM Freq"]; ok {
				freq, _ := strconv.Atoi(freqStr)
				if freq > 0 {
					vals["DRAM Ratio"] = strconv.Itoa(freq / bclk)
				}
			}
		}
	}
	var spd *SPDInfo
	if spdOut, spdErr := exec.Command("decode-dimms").CombinedOutput(); spdErr == nil {
		ParseSPDTimings(string(spdOut), vals)
		spd = ParseSPDInfo(string(spdOut))
	}
	count := len(vals)
	return vals, fmt.Sprintf("Detected %d params from hardware.", count), spd
}

// ParseSPDTimings parses SPD timing data and fills vals.
func ParseSPDTimings(spd string, vals map[string]string) {
	freqStr, ok := vals["DRAM Freq"]
	if !ok {
		return
	}
	freqMT, err := strconv.ParseFloat(freqStr, 64)
	if err != nil || freqMT <= 0 {
		return
	}
	freqMHz := freqMT / 2.0

	timingPatterns := []struct {
		field string
		re    *regexp.Regexp
	}{
		{"CAS (tCL)", regexp.MustCompile(`(?i)(?:CAS Latency|Minimum CAS Latency|tAA)\D+?(\d+\.?\d*)\s*ns`)},
		{"tRCD", regexp.MustCompile(`(?i)(?:RAS.to.CAS|tRCD)\D+?(\d+\.?\d*)\s*ns`)},
		{"tRP", regexp.MustCompile(`(?i)(?:Row.Precharge|tRP)\D+?(\d+\.?\d*)\s*ns`)},
		{"tRAS", regexp.MustCompile(`(?i)(?:Active.to.Precharge|tRAS)\D+?(\d+\.?\d*)\s*ns`)},
		{"tRFC", regexp.MustCompile(`(?i)(?:Refresh.Recovery|tRFC)\D+?(\d+\.?\d*)\s*ns`)},
	}
	for _, tp := range timingPatterns {
		if m := tp.re.FindStringSubmatch(spd); len(m) > 1 {
			ns, err := strconv.ParseFloat(m[1], 64)
			if err != nil || ns <= 0 {
				continue
			}
			cycles := int(math.Ceil(ns * freqMHz / 1000.0))
			vals[tp.field] = strconv.Itoa(cycles)
		}
	}
}

// ParseSPDInfo parses SPD info from decode-dimms output.
func ParseSPDInfo(spd string) *SPDInfo {
	info := &SPDInfo{}
	get := func(pattern string) string {
		re := regexp.MustCompile(pattern)
		if m := re.FindStringSubmatch(spd); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
		return ""
	}
	info.ModuleManufacturer = get(`Module Manufacturer\s+(.+)`)
	info.DRAMManufacturer = get(`DRAM Manufacturer\s+(.+)`)
	info.PartNumber = get(`Part Number\s+(.+)`)
	info.ModuleType = get(`Module Type\s+(.+)`)
	info.MemoryType = get(`Fundamental Memory type\s+(.+)`)
	info.ModuleSpeed = get(`Maximum module speed\s+(.+)`)
	info.SizeMB = get(`Size\s+(\d+\s*MB)`)
	info.Ranks = get(`Ranks\s+(\d+)`)
	info.BankLayout = get(`Banks x Rows x Columns x Bits\s+(.+)`)
	info.BusWidth = get(`Primary Bus Width\s+(.+)`)
	info.DeviceWidth = get(`SDRAM Device Width\s+(.+)`)
	info.SupportedCL = get(`Supported CAS Latencies.*?\s+([\dT,\s]+)`)
	info.Voltage = get(`Operable voltages\s+(.+)`)
	info.TimingString = get(`tCL-tRCD-tRP-tRAS\s+(.+)`)
	info.TWR = get(`Write Recovery.*?\(tWR\)\s+([\d.]+\s*ns)`)
	info.TRRD = get(`Row Active to Row Active.*?\(tRRD\)\s+([\d.]+\s*ns)`)
	info.TRC = get(`Active to Auto-Refresh.*?\(tRC\)\s+([\d.]+\s*ns)`)
	info.TWTR = get(`Write to Read.*?\(tWTR\)\s+([\d.]+\s*ns)`)
	info.TRTP = get(`Read to Pre-charge.*?\(tRTP\)\s+([\d.]+\s*ns)`)
	info.TFAW = get(`Four Activate Window.*?\(tFAW\)\s+([\d.]+\s*ns)`)
	info.TCKmin = get(`Minimum Cycle Time.*?\(tCK\)\s+([\d.]+\s*ns)`)
	info.DLLOff = get(`DLL-Off Mode supported\?\s+(.+)`)
	info.TempRange = get(`Operating temperature range\s+(.+)`)
	info.AutoSR = get(`Auto Self-Refresh\?\s+(.+)`)
	info.ModuleHeight = get(`Module Height\s+(.+)`)
	info.RefCard = get(`Module Reference Card\s+(.+)`)
	info.NumDIMMs = get(`Number of SDRAM DIMMs detected.*?:\s+(\d+)`)
	speedRe := regexp.MustCompile(`tCL-tRCD-tRP-tRAS as (DDR\d+-\d+)\s+([\d-]+)`)
	for _, m := range speedRe.FindAllStringSubmatch(spd, -1) {
		if len(m) > 2 {
			info.TimingsBySpeed = append(info.TimingsBySpeed, fmt.Sprintf("%-12s %s", m[1], m[2]))
		}
	}
	return info
}
