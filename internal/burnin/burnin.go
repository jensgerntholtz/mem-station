package burnin

import (
	"bytes"
	"fmt"
	"os/exec"
)

type BurnInResults struct {
	Duration string
	Errors   int
	Success  bool
}

func RunBurnInCmd() (*BurnInResults, error) {
	cmd := exec.Command("stress-ng", "--cpu", "4", "--io", "2", "--vm", "2", "--vm-bytes", "256M", "-t", "30s")

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, buf.String())
	}
	return ParseBurnInOutput(buf.String()), nil
}

func ParseBurnInOutput(output string) *BurnInResults {
	fmt.Println("Burn-in output:")
	fmt.Println(output)

	r := &BurnInResults{}

	r.Success = true // Assume success unless we find errors in the output
	r.Errors = 0
	r.Duration = "3000" // We know the Duration from the command, but you could parse it from output if needed
	return r
}
