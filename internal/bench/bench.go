// Package bench parses stress-ng memrate benchmark output. It does not execute
// jobs; execution is handled by internal/runner.
package bench

import (
	"regexp"
	"strconv"
)

// BenchMetric is one parsed memrate sub-metric.
type BenchMetric struct {
	Label string
	Value float64
}

// BenchResults is the parsed outcome of a memrate benchmark run.
type BenchResults struct {
	ReadRate  float64
	WriteRate float64
	Metrics   []BenchMetric
	Duration  string
}

var (
	readRateRe  = regexp.MustCompile(`read rate\s+([\d.]+)\s+MB per sec`)
	writeRateRe = regexp.MustCompile(`write rate\s+([\d.]+)\s+MB per sec`)
	metricRe    = regexp.MustCompile(`memrate\s+([\d.]+)\s+(\S+)\s+MB per sec`)
	durationRe  = regexp.MustCompile(`completed in\s+([\d.]+\s+secs)`)
)

// ParseBenchOutput parses stress-ng memrate output.
func ParseBenchOutput(output string) *BenchResults {
	r := &BenchResults{}
	if m := readRateRe.FindStringSubmatch(output); len(m) > 1 {
		r.ReadRate, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := writeRateRe.FindStringSubmatch(output); len(m) > 1 {
		r.WriteRate, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := durationRe.FindStringSubmatch(output); len(m) > 1 {
		r.Duration = m[1]
	}
	interesting := map[string]bool{
		"read64": true, "read128": true, "read256": true,
		"write64": true, "write128": true, "write64stoq": true,
		"read64pf": true, "read128pf": true,
		"memset": true,
	}
	for _, match := range metricRe.FindAllStringSubmatch(output, -1) {
		if len(match) < 3 {
			continue
		}
		label := match[2]
		if !interesting[label] {
			continue
		}
		val, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		r.Metrics = append(r.Metrics, BenchMetric{Label: label, Value: val})
	}
	return r
}
