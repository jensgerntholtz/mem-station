package app

import (
	"testing"

	"mem-station/internal/config"
	"mem-station/internal/memory"
	"mem-station/internal/store"
	"mem-station/internal/uitest"

	tea "charm.land/bubbletea/v2"
)

// detectedMsg is the deterministic detection result every golden view starts
// from, so header summary, status line, and timing values never vary.
func detectedMsg() detectResultMsg {
	return detectResultMsg{
		values: map[string]string{
			"Channels":     "2",
			"DRAM Ratio":   "16",
			"DRAM Freq":    "3200",
			"CAS (tCL)":    "16",
			"tRCD":         "18",
			"tRP":          "18",
			"tRAS":         "38",
			"Command Rate": "1T",
			"tRFC":         "350",
			"DRAM Voltage": "1.35V",
		},
		msg: "Detected 2 channels at 3200 MT/s",
		spd: &memory.SPDInfo{
			PartNumber:   "F4-3200C16-16GVK",
			SizeMB:       "16384",
			ModuleSpeed:  "3200 MT/s",
			Ranks:        "2",
			TimingString: "16-18-18-38",
		},
	}
}

// goldenModel builds the shell at w x h with a completed, deterministic
// detection so the golden views cover the detected-values layout.
func goldenModel(t *testing.T, w, h int) *Model {
	t.Helper()
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.Update(detectedMsg())
	return m
}

// TestGoldenShellViews pins the shell chrome and the Timings workspace at the
// minimum and a wide terminal, plus the help overlay and the apply review.
func TestGoldenShellViews(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		run  func(*Model)
	}{
		{name: "timings_78x21", w: 80, h: 24},
		{name: "timings_118x37", w: 120, h: 40},
		{name: "help_78x21", w: 80, h: 24, run: func(m *Model) {
			m.Update(uitest.Key("?"))
		}},
		{name: "help_118x37", w: 120, h: 40, run: func(m *Model) {
			m.Update(uitest.Key("?"))
		}},
		{name: "apply_review_118x37", w: 120, h: 40, run: func(m *Model) {
			// Edit the focused field (Channels), then open the review modal.
			m.Update(uitest.Code(tea.KeyEnter))
			m.Update(uitest.Key("4"))
			m.Update(uitest.Code(tea.KeyEnter))
			m.Update(uitest.Ctrl('s'))
		}},
		{name: "profile_prompt_118x37", w: 120, h: 40, run: func(m *Model) {
			m.Update(uitest.Ctrl('p'))
		}},
		{name: "profile_picker_118x37", w: 120, h: 40, run: func(m *Model) {
			for _, name := range []string{"daily-3200", "undervolt", "stock"} {
				if err := store.SaveNamedProfile(config.Profile{Name: name}); err != nil {
					t.Fatalf("SaveNamedProfile(%q): %v", name, err)
				}
			}
			m.Update(uitest.Ctrl('l'))
		}},
		{name: "toosmall_68x18", w: 70, h: 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := goldenModel(t, tc.w, tc.h)
			if tc.run != nil {
				tc.run(m)
			}
			uitest.GoldenView(t, "testdata", tc.name+".golden", tc.w, tc.h, m.View().Content)
		})
	}
}
