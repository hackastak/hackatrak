// Command hackatrak is a local-first, automatic work log.
//
// This is a bare starting point. The full design and build plan live in the
// Obsidian vault under "1. Projects/Hackatrak".
package main

import (
	"fmt"

	// Pinned until the Phase 0 skeleton imports them directly.
	_ "github.com/BurntSushi/toml"
	_ "github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

func main() {
	fmt.Println("hackatrak: bare scaffold — ready to build out")
}
