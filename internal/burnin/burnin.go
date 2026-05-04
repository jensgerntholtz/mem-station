package burnin

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type BurnInResults struct {
	Duration string
	Errors   int
	Success  bool
}

func RunBurnInCmd() (*BurnInResults, error) {
	cmd := exec.Command("stress-ng", "--job", "burnin-memory.job", "--log-file", "burnin.log")

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

	r.Success = true
	r.Errors = 0
	r.Duration = "30s"
	return r
}

// StreamBurnInCmd runs the burn-in command and streams output lines via a callback.
func StreamBurnInCmd(ctx context.Context, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, "stress-ng", "--job", "burnin-memory.job", "--log-file", "burnin.log")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Read both stdout and stderr
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			onLine(scanner.Text())
		}
	}()
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		onLine(scanner.Text())
	}
	err = cmd.Wait()
	return err
}
