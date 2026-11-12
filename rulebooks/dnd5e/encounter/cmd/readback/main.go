// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Command readback says one content file back in plain sentences, each
// followed by where in the file it is set (rpg-project#941).
//
// Run from the module directory:
//
//	go run ./cmd/readback dungeonspec/testdata/world-builder-v4-castle.yaml
//	go run ./cmd/readback --full dungeonspec/testdata/world-builder-v4-castle.yaml
//	go run ./cmd/readback --json dungeonspec/testdata/hail-site.yaml
//
// By default it prints the folded text: each line said once, things that
// differ only by state or by creature said once, topics and facts as tables
// of places. --full prints one sentence per statement, each followed by its
// path. --json prints the structured statements. A file the
// compiler refuses prints "REFUSED:" and the compiler's sentences and exits
// 1; a bad flag, a missing argument or an unreadable file exits 2.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the command with its arguments and outputs, for the tests.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("readback", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print the structured statements as JSON")
	full := fs.Bool("full", false, "print one sentence per statement instead of the folded text")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: readback [--json] <file>")
		return 2
	}
	raw, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "readback: %v\n", err)
		return 2
	}
	rb, err := readback.Of(raw)
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "REFUSED: %v\n", err)
		return 1
	}
	if *asJSON {
		out, err := json.MarshalIndent(rb, "", "  ")
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "readback: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	if *full {
		_, _ = io.WriteString(stdout, rb.Text())
		return 0
	}
	_, _ = io.WriteString(stdout, rb.FoldedText())
	return 0
}
