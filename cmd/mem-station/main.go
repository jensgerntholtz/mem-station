// Command mem-station runs the MemStation memory timing workspace TUI and its
// headless info/bench/burnin subcommands.
package main

import (
	"os"

	"mem-station/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.DefaultDeps()))
}
