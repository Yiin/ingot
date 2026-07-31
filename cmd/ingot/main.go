// Command ingot is the Ingot desktop app entry point.
package main

import (
	"os"

	"github.com/Yiin/ingot/internal/cli"
)

func main() {
	cli.Run(os.Args)
}
