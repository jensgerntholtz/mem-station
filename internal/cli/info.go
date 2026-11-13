package cli

import (
	"fmt"
	"strings"

	"mem-station/internal/memory"
	"mem-station/internal/report"
	"mem-station/internal/ui"

	"github.com/spf13/cobra"
)

// infoDocument is the stable JSON document `info -json` prints. It is the same
// type the report uses, so the two can never drift apart.
type infoDocument = report.Hardware

// infoOptions are the flags of the info command.
type infoOptions struct {
	asJSON  bool
	withIMC bool
}

// newInfoCmd builds the info subcommand.
func newInfoCmd(deps Deps) *cobra.Command {
	var opts infoOptions
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Print detected hardware values",
		Long: `Print detected hardware values, SPD details, and the IMC register dump.

Detection needs root for dmidecode/decode-dimms; raw IMC registers need root and
may need iomem=relaxed on the kernel command line.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return runInfo(deps, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print one JSON object on stdout")
	cmd.Flags().BoolVar(&opts.withIMC, "imc", false, "read raw IMC registers (root; may need iomem=relaxed)")
	return cmd
}

// runInfo prints detected hardware values, SPD details, and the IMC dump.
func runInfo(deps Deps, opts infoOptions) error {
	values, msg, spd := deps.Detect()
	deps.Log.Debug("detection finished", "values", len(values), "spd", spd != nil)
	doc := infoDocument{Values: values, Message: msg, SPD: spd}
	if opts.withIMC {
		timings, imcErr := deps.ReadIMC()
		doc.IMC = timings
		if imcErr != nil {
			doc.IMCError = imcErr.Error()
		}
	} else {
		doc.IMCError = "not requested (pass -imc to read raw registers; needs root and may need iomem=relaxed)"
	}
	if opts.asJSON {
		if err := writeJSON(deps.Stdout, doc); err != nil {
			return fail(err)
		}
		return nil
	}

	printInfoHuman(deps, doc)
	return nil
}

// printInfoHuman renders the detection result for a terminal.
func printInfoHuman(deps Deps, doc infoDocument) {
	fmt.Fprintln(deps.Stdout, "Hardware")
	fmt.Fprintf(deps.Stdout, "  %s\n", doc.Message)
	if len(doc.Values) == 0 {
		fmt.Fprintln(deps.Stdout, "  no values detected (try running with sudo)")
	}
	for _, key := range report.OrderedValues(doc.Values) {
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
			reason = "no channels readable"
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
