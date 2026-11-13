package report

import (
	"strings"
	"testing"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/memory"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/uitest"
)

// fixed is the timestamp every fixture uses, so the Markdown is deterministic.
var fixed = time.Date(2026, 11, 13, 20, 4, 0, 0, time.UTC)

// fullDocument is a report with every section populated.
func fullDocument() Document {
	return Document{
		GeneratedAt: fixed,
		Version:     "0.2.0-test",
		Hardware: Hardware{
			Values: map[string]string{
				"Channels":     "2",
				"DRAM Freq":    "3200",
				"DRAM Voltage": "1.35V",
				"Extra Field":  "kept",
			},
			Message: "Detected 4 params from hardware.",
			SPD: &memory.SPDInfo{
				PartNumber:   "F4-3200C16-16GVK",
				SizeMB:       "16384",
				ModuleSpeed:  "3200 MT/s",
				Ranks:        "2",
				TimingString: "16-18-18-38",
				TimingsBySpeed: []string{
					"DDR4-3200 16-18-18-38",
				},
			},
			IMC: []memory.IMCTimings{{Channel: 0, TRAS: 38, TRP: 18, TRCD: 18, TRFC: 350, TREFI: 7800}},
		},
		Profile: &config.Profile{
			Name:      "active",
			UpdatedAt: fixed.Add(-time.Hour),
			Source:    "applied",
			Values: map[string]string{
				"tCL":  "16",
				"tRCD": "18",
				"freq": "3200",
			},
		},
		History: []store.Record{
			{
				ID:         "1",
				Kind:       runner.KindBurnIn,
				StartedAt:  fixed.Add(-2 * time.Hour),
				DurationMS: 30 * 60 * 1000,
				ExitCode:   0,
			},
			{
				ID:         "2",
				Kind:       runner.KindBenchmark,
				StartedAt:  fixed.Add(-time.Hour),
				DurationMS: 15000,
				ExitCode:   2,
				Error:      "stress-ng exited with status 2",
			},
		},
	}
}

// TestMarkdownGolden pins the whole document, including the section order and
// the table layout glamour will consume.
func TestMarkdownGolden(t *testing.T) {
	uitest.Golden(t, "testdata", "report_full.md.golden", Markdown(fullDocument()))
}

// TestMarkdownEmptyDocument checks a machine with no detection, no profile and
// no runs still produces a complete, readable report.
func TestMarkdownEmptyDocument(t *testing.T) {
	doc := Document{GeneratedAt: fixed, Version: "0.2.0-test"}
	out := Markdown(doc)
	for _, want := range []string{
		"# MemStation report",
		"## Hardware",
		"### SPD",
		"### IMC registers",
		"## Applied profile",
		"## Recent runs",
		"No profile has been applied yet",
		"No runs recorded yet",
		"Profile only",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("empty report missing %q:\n%s", want, out)
		}
	}
}

// TestMarkdownEscapesPipes keeps a value from breaking the table layout.
func TestMarkdownEscapesPipes(t *testing.T) {
	doc := Document{
		GeneratedAt: fixed,
		Version:     "test",
		Hardware: Hardware{
			Values: map[string]string{"Parts": "16GB|2 sticks"},
		},
	}
	out := Markdown(doc)
	if !strings.Contains(out, `16GB\|2 sticks`) {
		t.Errorf("pipe in a value was not escaped:\n%s", out)
	}
}

// TestOrderedValues checks known keys come first and unknown keys follow
// alphabetically, so a new detection field can never be dropped.
func TestOrderedValues(t *testing.T) {
	got := OrderedValues(map[string]string{
		"Zeta":         "1",
		"Alpha":        "2",
		"DRAM Freq":    "3200",
		"Channels":     "2",
		"tRAS":         "38",
		"New Detector": "x",
	})
	want := []string{"Channels", "DRAM Freq", "tRAS", "Alpha", "New Detector", "Zeta"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("OrderedValues = %v, want %v", got, want)
	}
}
