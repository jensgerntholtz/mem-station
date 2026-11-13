// Command seed writes the deterministic state the demo tapes start from: one
// applied profile plus a few named profiles for the picker.
//
// It is a demo helper, not part of mem-station: it only touches the state
// directory named by MEM_STATION_STATE_DIR.
package main

import (
	"fmt"
	"os"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/store"
)

// seeded is the fixed timestamp of every profile, so a tape recorded twice
// looks the same.
var seeded = time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC)

func values(freq, cl, rcd, rp, ras, cr, rfc, voltage string) map[string]string {
	return map[string]string{
		config.KeyChannels: "2",
		config.KeyRatio:    "16",
		config.KeyFreq:     freq,
		config.KeyCL:       cl,
		config.KeyRCD:      rcd,
		config.KeyRP:       rp,
		config.KeyRAS:      ras,
		config.KeyCR:       cr,
		config.KeyRFC:      rfc,
		config.KeyVoltage:  voltage,
	}
}

func main() {
	applied := config.Profile{
		Name:      "active",
		Source:    "applied",
		UpdatedAt: seeded,
		Values:    values("3200", "16", "18", "18", "38", "1T", "350", "1.35"),
	}
	if err := store.SaveActiveProfile(applied); err != nil {
		fmt.Fprintln(os.Stderr, "seed: active profile:", err)
		os.Exit(1)
	}

	named := []config.Profile{
		{Name: "daily-3200", Source: "draft", UpdatedAt: seeded, Values: values("3200", "16", "18", "18", "38", "1T", "350", "1.35")},
		{Name: "undervolt", Source: "draft", UpdatedAt: seeded, Values: values("2933", "16", "18", "18", "38", "1T", "350", "1.20")},
		{Name: "stock", Source: "draft", UpdatedAt: seeded, Values: values("2133", "15", "15", "15", "36", "1T", "350", "1.20")},
	}
	for _, p := range named {
		if err := store.SaveNamedProfile(p); err != nil {
			fmt.Fprintln(os.Stderr, "seed: named profile:", err)
			os.Exit(1)
		}
	}

	dir, err := store.Dir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Printf("seeded %s\n", dir)
}
