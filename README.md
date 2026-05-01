# MemStation
A Linux-based TUI application for overclocking enthusiasts to verify memory stability and performance.

## Features
- **System Information:**
  - Detailed hardware and software info (similar to CPU-Z)
  - Memory timings, frequency, voltage, and more
  - Theoretical max performance calculation
  - Save system info for later reference
- **Burn-in Testing:**
  - Stress-ng presets for stability testing
  - Memory-only and combined CPU+Memory stress tests
  - Workloads similar to Linpack, Prime95
- **Benchmarks:**
  - FOSS-based synthetic performance measurement
  - Compare theoretical vs actual memory bandwidth

## Quick Start
### Prerequisites
- Go toolchain (1.20+ recommended)
- Linux (tested on recent distributions)

### Run the TUI
```
cd mem-station
# Run the modular TUI app
 go run ./cmd/mem-station
```

### Build
```
go build -o mem-station ./cmd/mem-station
```

### Usage
- Use **Tab/Shift+Tab** to move between fields
- **F1/F2** to switch tabs (System Info / Benchmark)
- **L** to toggle ratio lock
- **Enter** to apply changes or run benchmarks
- **Esc** to cancel changes
- **Q** or **Ctrl+C** to quit

### Notes
- Some features require root privileges (e.g., hardware info via dmidecode)
- For best results, run with `sudo` if hardware info is missing
- Benchmarking uses `stress-ng` (ensure it is installed)

---
For development or troubleshooting, see the modular Go code in `internal/model/`, `cmd/`, and related files.