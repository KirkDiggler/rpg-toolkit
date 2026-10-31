// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
)

// TestMain_WritesAndReports: the command writes a site the loader accepts,
// prints the report, and records itself in the header so the same options
// regenerate the same bytes.
func TestMain_WritesAndReports(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bench-keep.yaml")
	var stdout, stderr bytes.Buffer
	args := []string{"-hexes", "10000", "-key", "bench-keep", "-out", out}
	require.Equal(t, 0, run(args, &stdout, &stderr), stderr.String())
	require.Contains(t, stdout.String(), "hexes=10000 scale=1 regions=32")
	require.Contains(t, stdout.String(), "→ "+out)

	doc, err := os.ReadFile(out)
	require.NoError(t, err)
	_, err = dungeonspec.Load(doc)
	require.NoError(t, err)
	opts, err := keepgen.OptionsFromHeader(doc)
	require.NoError(t, err)
	require.Equal(t, "keepgen -hexes 10000 -key bench-keep -out "+out, opts.CommandLine)
	again, _, err := keepgen.Generate(keepgen.DefaultPlan(), opts)
	require.NoError(t, err)
	require.Equal(t, doc, again)
}

// TestMain_RefusesBelowPlan: fewer hexes than the plan's floor is refused,
// exit 1, the reason on stderr, and no file.
func TestMain_RefusesBelowPlan(t *testing.T) {
	out := filepath.Join(t.TempDir(), "k.yaml")
	var stdout, stderr bytes.Buffer
	require.Equal(t, 1, run([]string{"-hexes", "500", "-out", out}, &stdout, &stderr))
	require.Contains(t, stderr.String(), "the plan needs at least 10000 hexes")
	require.Empty(t, stdout.String())
	_, err := os.Stat(out)
	require.True(t, os.IsNotExist(err), "no file is written")
}

// TestMain_RefusesUnreachablePlan: a local plan whose doors leave a room
// shut off is the proof's refusal, and no file.
func TestMain_RefusesUnreachablePlan(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join("..", "..", "dungeonspec", "keepgen", "plan", "keep.yaml"))
	require.NoError(t, err)
	edited := bytes.Replace(raw, []byte("  - [treasury, hidden-stash]\n"), nil, 1)
	require.NotEqual(t, raw, edited)
	plan := filepath.Join(dir, "plan.yaml")
	require.NoError(t, os.WriteFile(plan, edited, 0o600))
	out := filepath.Join(dir, "k.yaml")
	var stdout, stderr bytes.Buffer
	require.Equal(t, 1, run([]string{"-plan", plan, "-out", out}, &stdout, &stderr))
	require.Contains(t, stderr.String(), `region "hidden-stash" is unreachable`)
	_, err = os.Stat(out)
	require.True(t, os.IsNotExist(err))
}

// TestMain_RequiresOut: no -out, exit 2.
func TestMain_RequiresOut(t *testing.T) {
	var stdout, stderr bytes.Buffer
	require.Equal(t, 2, run(nil, &stdout, &stderr))
	require.Contains(t, stderr.String(), "-out is required")
}
