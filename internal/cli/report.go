package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"mem-station/internal/report"
	"mem-station/internal/store"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// defaultReportStyle is documented in the CLI help. "auto" means: plain text
// when stdout is not a terminal or NO_COLOR is set, otherwise the dark theme.
const defaultReportStyle = "auto"

// reportOptions are the flags of the report command.
type reportOptions struct {
	asJSON     bool
	asMarkdown bool
	withIMC    bool
	history    int
	style      string
}

// newReportCmd builds the report subcommand.
func newReportCmd(deps Deps) *cobra.Command {
	var opts reportOptions
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render a hardware, profile and run-history report",
		Long: `Render one report of detected hardware, the applied profile and recent runs.

The report is Markdown rendered for the terminal. When stdout is not a
terminal the Markdown source is printed unchanged, so redirecting it to a .md
file produces a valid document; --style forces a specific rendering instead.
--json prints the same content as one JSON document, reusing the schemas that
info, the profile store and history.jsonl already define.

Detection needs root for dmidecode/decode-dimms; raw IMC registers are opt-in
with --imc and may need iomem=relaxed on the kernel command line.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return runReport(deps, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print one JSON document on stdout")
	cmd.Flags().BoolVar(&opts.asMarkdown, "markdown", false, "render Markdown for the terminal (the default)")
	cmd.Flags().BoolVar(&opts.withIMC, "imc", false, "include raw IMC registers (root; may need iomem=relaxed)")
	cmd.Flags().IntVar(&opts.history, "history", 10, "recent runs to include")
	cmd.Flags().StringVar(&opts.style, "style", defaultReportStyle,
		"style: auto, raw, dark, light, notty, ascii, pink, dracula, tokyo-night")
	return cmd
}

// runReport builds the document and prints it as JSON or rendered Markdown.
func runReport(deps Deps, opts reportOptions) error {
	if opts.asJSON && opts.asMarkdown {
		return usageErr(errors.New("--json and --markdown are mutually exclusive"))
	}
	if opts.history < 1 {
		return usageErr(errors.New("--history must be at least 1"))
	}

	doc, err := buildReport(deps, opts)
	if err != nil {
		return fail(err)
	}
	if opts.asJSON {
		if err := writeJSON(deps.Stdout, doc); err != nil {
			return fail(err)
		}
		return nil
	}

	markdown := report.Markdown(doc)
	style := resolveReportStyle(deps, opts.style)
	if style == "" {
		// Piped output is the Markdown source, so `report > report.md` is a
		// valid document for GitHub, pandoc, or a diff.
		fmt.Fprint(deps.Stdout, markdown)
		return nil
	}
	rendered, err := renderReport(markdown, style)
	if err != nil {
		return fail(err)
	}
	fmt.Fprint(deps.Stdout, rendered)
	return nil
}

// buildReport gathers the same data the headless commands and the TUI read.
// Detection and history failures are recorded in the document rather than
// failing the command: a report without a profile is still a useful report.
func buildReport(deps Deps, opts reportOptions) (report.Document, error) {
	values, msg, spd := deps.Detect()
	doc := report.Document{
		GeneratedAt: time.Now().UTC(),
		Version:     deps.Version,
		Hardware:    report.Hardware{Values: values, Message: msg, SPD: spd},
		History:     []store.Record{},
	}
	deps.Log.Debug("report detection finished", "values", len(values), "spd", spd != nil)

	if opts.withIMC {
		timings, err := deps.ReadIMC()
		doc.Hardware.IMC = timings
		if err != nil {
			doc.Hardware.IMCError = err.Error()
		}
	} else {
		doc.Hardware.IMCError = "not requested (pass --imc to read raw registers; needs root and may need iomem=relaxed)"
	}

	profile, ok, err := store.LoadActiveProfile()
	if err != nil {
		deps.Log.Warn("applied profile unavailable", "err", err)
	} else if ok {
		doc.Profile = &profile
	}

	if opts.history > 0 {
		records, err := store.RecentHistory("", opts.history)
		if err != nil {
			deps.Log.Warn("run history unavailable", "err", err)
		} else if records != nil {
			doc.History = records
		}
	}
	return doc, nil
}

// renderReport renders Markdown for the terminal with the given glamour style.
func renderReport(markdown, style string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(80),
		glamour.WithTableWrap(true),
	)
	if err != nil {
		return "", fmt.Errorf("report style %q: %w", style, err)
	}
	out, err := renderer.Render(markdown)
	if err != nil {
		return "", fmt.Errorf("render report: %w", err)
	}
	return out, nil
}

// resolveReportStyle turns the --style value into a glamour style name. It
// returns "" for "emit the Markdown source unchanged".
//
// auto: raw Markdown when stdout is a pipe or a file (so the output is a real
// Markdown document), the dark theme on a terminal, and the plain notty style
// when NO_COLOR is set on a terminal.
func resolveReportStyle(deps Deps, styleName string) string {
	style := strings.ToLower(strings.TrimSpace(styleName))
	switch style {
	case "", "auto":
		if !stdoutIsTerminal(deps.Stdout) {
			return ""
		}
		if os.Getenv("NO_COLOR") != "" {
			return styles.NoTTYStyle
		}
		return styles.DarkStyle
	case "markdown", "raw", "source":
		return ""
	case "plain", "plaintext":
		return styles.NoTTYStyle
	default:
		return style
	}
}

// stdoutIsTerminal reports whether w is an interactive terminal. An injected
// buffer is never a terminal, which is what makes piped output plain.
func stdoutIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(f.Fd())
}
