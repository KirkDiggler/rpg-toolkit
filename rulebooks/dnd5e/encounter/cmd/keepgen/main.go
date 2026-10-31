// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Command keepgen writes the benchmark keep (rpg-project#740): FadedPez's
// keep from the embedded plan (or a local one), at a hex count from keep
// scale to the 30,000-hex ceiling, propped and garrisoned at the requested
// densities, as a version-5 site the dialect's loader has already accepted.
//
// Run from the module directory:
//
//	go run ./cmd/keepgen -hexes 30000 -key bench-keep-30k -out bench-keep-30k.yaml
//
// Flags:
//
//	-plan          embedded (default), or a path to a plan file
//	-hexes         the floor's exact cell count (default 10000)
//	-seed          the placement seed (default 1)
//	-props         props per 100 room hexes (default 12.5)
//	-grounds-props props per 100 courtyard and grounds hexes (default 2.5)
//	-monsters      monsters per 100 floor hexes (default 1)
//	-key           the site's key, equal to its shelf file's name (default bench-keep)
//	-out           the file to write (required)
//
// It prints the report as one line. A site the loader refuses, that seats
// fewer than four at the start, or that leaves a room unreachable is not
// written: the reason goes to stderr and the exit status is 1. The file's
// header records the command line, so the same command makes the same bytes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
)

const embedded = "embedded"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the command with its arguments and outputs, for the tests: 0 on a
// written file, 1 on a refusal, 2 on a bad flag.
func run(args []string, stdout, stderr io.Writer) int {
	def := keepgen.DefaultOptions()
	fs := flag.NewFlagSet("keepgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	planPath := fs.String("plan", embedded, "embedded, or a path to a plan file")
	hexes := fs.Int("hexes", def.Hexes, "the floor's exact cell count")
	seed := fs.Int64("seed", def.Seed, "the placement seed")
	props := fs.Float64("props", def.PropDensity, "props per 100 room hexes")
	grounds := fs.Float64("grounds-props", def.GroundsDensity, "props per 100 courtyard and grounds hexes")
	monsters := fs.Float64("monsters", def.MonsterDensity, "monsters per 100 floor hexes")
	key := fs.String("key", def.Key, "the site's key, equal to its shelf file's name")
	out := fs.String("out", "", "the file to write (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *out == "" {
		_, _ = fmt.Fprintln(stderr, "keepgen: -out is required")
		return 2
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "keepgen: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	plan, err := loadPlan(*planPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	doc, report, err := keepgen.Generate(plan, keepgen.Options{
		Hexes: *hexes, Seed: *seed, PropDensity: *props, GroundsDensity: *grounds, MonsterDensity: *monsters,
		Key: *key, CommandLine: strings.Join(append([]string{"keepgen"}, args...), " "),
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.WriteFile(*out, doc, 0o600); err != nil {
		_, _ = fmt.Fprintf(stderr, "keepgen: write %s: %v\n", *out, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "%s → %s\n", report, *out)
	return 0
}

func loadPlan(path string) (*keepgen.Plan, error) {
	if path == embedded {
		return keepgen.DefaultPlan(), nil
	}
	f, err := os.Open(path) // #nosec G304 -- the operator names the plan file
	if err != nil {
		return nil, fmt.Errorf("keepgen: plan: %w", err)
	}
	p, err := keepgen.LoadPlan(f)
	return p, errors.Join(err, f.Close())
}
