// Command hackatrak is a local-first, automatic work log.
//
// Cobra wiring lands in the next Phase 0 step. This file only keeps the
// binary building from cmd/hackatrak.
package main

import (
	"fmt"

	_ "github.com/spf13/cobra" // pinned until the root command lands
)

func main() {
	fmt.Println("hackatrak: bare scaffold — ready to build out")
}
