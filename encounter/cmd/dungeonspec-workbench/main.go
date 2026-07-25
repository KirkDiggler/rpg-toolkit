// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Command dungeonspec-workbench is the fast dungeon-authoring loop
// (design.md's observability addendum, rpg-toolkit#842): edit a spec YAML
// file, run this against it, and see the compiled layout -- verdict,
// regions, spawn plan, placed props, and an ASCII floor plan -- in
// seconds, with no server, no rpg-api, no Discord activity involved. All
// the actual work happens in dungeonspec.WorkbenchReport; this file is
// just flag parsing and printing.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
)

func main() {
	file := flag.String("file", "", "path to a dungeon spec YAML file (required)")
	seed := flag.Int64("seed", 1, "seed to compile the layout at")
	n := flag.Int("n", 1, "sweep this many consecutive seeds starting at -seed, printing one report per seed")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: dungeonspec-workbench -file <spec.yaml> [-seed N] [-n SWEEP]")
		os.Exit(2)
	}
	if *n < 1 {
		fmt.Fprintln(os.Stderr, "-n must be at least 1")
		os.Exit(2)
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *file, err)
		os.Exit(1)
	}

	// A -n sweep prints every seed's report regardless of individual
	// failures (an author sweeping seeds wants to see ALL of them, not
	// have the run abort partway through) -- the exit code just reflects
	// whether ANY seed in the sweep came back INVALID.
	sawInvalid := false
	for i := 0; i < *n; i++ {
		report, err := dungeonspec.WorkbenchReport(raw, *seed+int64(i))
		fmt.Println(report)
		if err != nil {
			sawInvalid = true
		}
	}
	if sawInvalid {
		os.Exit(1)
	}
}
