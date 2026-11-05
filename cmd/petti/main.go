// Command petti is the Petti container runtime CLI.
package main

import (
	"os"

	"github.com/r4r00t-sh/petti/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:], cli.DefaultIO()))
}
