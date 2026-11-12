// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Command contentcheck compiles one content file with the dialect's own
// loader and says whether it compiles (rpg-project#941).
//
// Run from the module directory:
//
//	go run ./cmd/contentcheck dungeonspec/testdata/world-builder-v4-castle.yaml
//
// It prints "OK: the file compiles" and exits 0, or "REFUSED:" and the
// compiler's own sentences, each at the path it refused, and exits 1. A
// missing argument or an unreadable file exits 2.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the command with its arguments and outputs, for the tests.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: contentcheck <file>")
		return 2
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "contentcheck: %v\n", err)
		return 2
	}
	if _, err := dungeonspec.Load(raw); err != nil {
		_, _ = fmt.Fprintf(stdout, "REFUSED: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "OK: the file compiles")
	return 0
}
