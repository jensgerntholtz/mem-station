package cli

import (
	"fmt"
	"sort"
	"strings"

	"mem-station/internal/memory"
	"mem-station/internal/ui"
)

// infoDocument is the stable JSON document `info -json` prints.
type infoDocument struct {
	Values   map[string]string   `json:"values"`
	Message  string              `json:"message"`
	SPD      *memory.SPDInfo     `json:"spd,omitempty"`
	IMC      []memory.IMCTimings `json:"imc,omitempty"`
	IMCError string              `json:"imc_error,omitempty"`
}

// infoKeys is the display order of the detected values. Keys that are not listed
// (future detection fields) are printed afterwards in alphabetical order so
// nothing is ever dropped from the output.
var infoKeys = []string{
	"Channels",
	"DRAM Ratio",
	"DRAM Freq",
	"CAS (tCL)",
	"tRCD",
	"tRP",
	"tRAS",
	"Command Rate",
	"tRFC",
	"DRAM Voltage",
}

// runInfo prints detected hardware values, SPD details, and the IMC dump.
func runInfo(deps Deps, args []string) int {
	fs := newFlagSet("info", deps)
	asJSON := fs.Bool("json", false, "print one JSON object on stdout")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(deps.Stderr, "mem-station info: unexpected argument %q\n", fs.Arg(0))
		return ExitUsage
	}

	values, msg, spd := deps.Detect()
	timings, imcErr := deps.ReadIMC()

	doc := infoDocument{Values: values, Message: msg, SPD: spd, IMC: timings}
	if imcErr != nil {
		doc.IMCError = imcErr.Error()
	}
	if *asJSON {
		if err := writeJSON(deps.Stdout, doc); err != nil {
			fmt.Fprintln(deps.Stderr, "mem-station info:", err)
			return ExitFailed
		}
		return ExitOK
	}

	printInfoHuman(deps, doc)
	return ExitOK
}

// printInfoHuman renders the detection result for a terminal.
func printInfoHuman(deps Deps, doc infoDocument) {
	fmt.Fprintln(deps.Stdout, "Hardware")
	fmt.Fprintf(deps.Stdout, "  %s\n", doc.Message)
	if len(doc.Values) == 0 {
		fmt.Fprintln(deps.Stdout, "  no values detected (try running with sudo)")
	}
	for _, key := range orderedKeys(doc.Values) {
		if v := strings.TrimSpace(doc.Values[key]); v != "" {
			fmt.Fprintf(deps.Stdout, "  %-16s %s\n", key, v)
		}
	}

	if doc.SPD == nil {
		fmt.Fprintln(deps.Stdout, "\nSPD\n  unavailable (needs dmidecode as root, or decode-dimms)")
	} else {
		fmt.Fprintln(deps.Stdout, "\nSPD")
		printSPD(deps, doc.SPD)
	}

	if len(doc.IMC) == 0 {
		reason := doc.IMCError
		if reason == "" {
			reason = "no channels readable (needs root and iomem=relaxed)"
		}
		fmt.Fprintf(deps.Stdout, "\nIMC\n  unavailable: %s\n", reason)
		return
	}
	fmt.Fprintln(deps.Stdout, "\nIMC")
	for _, t := range doc.IMC {
		fmt.Fprintf(deps.Stdout, "  channel %d: tRAS %d tRP %d tRCD %d tRFC %d tREFI %d\n",
			t.Channel, t.TRAS, t.TRP, t.TRCD, t.TRFC, t.TREFI)
	}
}

// printSPD prints the SPD fields that are present, never inventing blanks.
func printSPD(deps Deps, spd *memory.SPDInfo) {
	fields := []struct{ label, value string }{
		{"Part number", spd.PartNumber},
		{"Module manufacturer", spd.ModuleManufacturer},
		{"DRAM manufacturer", spd.DRAMManufacturer},
		{"Module type", spd.ModuleType},
		{"Memory type", spd.MemoryType},
		{"Speed", spd.ModuleSpeed},
		{"Size", spd.SizeLabel()},
		{"Ranks", spd.Ranks},
		{"Bus width", spd.BusWidth},
		{"Device width", spd.DeviceWidth},
		{"Supported CL", spd.SupportedCL},
		{"Voltage", spd.Voltage},
		{"Timings", spd.TimingString},
	}
	for _, f := range fields {
		if v := strings.TrimSpace(f.value); v != "" {
			fmt.Fprintf(deps.Stdout, "  %-20s %s\n", f.label, v)
		}
	}
	for _, t := range spd.TimingsBySpeed {
		if v := strings.TrimSpace(t); v != "" {
			fmt.Fprintf(deps.Stdout, "  %-20s %s\n", "JEDEC profile", ui.Truncate(v, 60, false))
		}
	}
}

// orderedKeys returns the known keys first, then any extra key alphabetically.
func orderedKeys(values map[string]string) []string {
	seen := make(map[string]bool, len(values))
	keys := make([]string, 0, len(values))
	for _, k := range infoKeys {
		if _, ok := values[k]; ok {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	extra := make([]string, 0, len(values))
	for k := range values {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return append(keys, extra...)
}
