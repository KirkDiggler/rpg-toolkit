// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// TestCompiledMoveIDFailsBeforeMove pins the workbench's host-side declaration
// handoff: an absent or blocked Move is explained before an empty selector ever
// reaches the mutating verb.
func TestCompiledMoveIDFailsBeforeMove(t *testing.T) {
	tests := []struct {
		name         string
		declarations []session.Declaration
		wantID       string
		wantErr      string
	}{
		{
			name: "compiled Move",
			declarations: []session.Declaration{
				{Verb: session.VerbAttack, ID: "attack"},
				{Verb: session.VerbMove, ID: "move"},
			},
			wantID: "move",
		},
		{
			name:    "Move absent",
			wantErr: "no compiled Move declaration",
		},
		{
			name: "Move blocked without prose",
			declarations: []session.Declaration{
				{Verb: session.VerbMove},
			},
			wantErr: "no compiled Move declaration",
		},
		{
			name: "Move blocker explains why",
			declarations: []session.Declaration{
				{Verb: session.VerbMove, Why: &session.Shortfall{Text: "member is unreadable"}},
			},
			wantErr: "member is unreadable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := compiledMoveID(tt.declarations)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("compiledMoveID() error = %v, want text %q", err, tt.wantErr)
				}
				if id != "" {
					t.Fatalf("compiledMoveID() id = %q on failure", id)
				}
				return
			}
			if err != nil {
				t.Fatalf("compiledMoveID() error = %v", err)
			}
			if id != tt.wantID {
				t.Fatalf("compiledMoveID() id = %q, want %q", id, tt.wantID)
			}
		})
	}
}

// TestWorkbenchRuns exists because of a lesson this repo learned the expensive
// way: the encounter's workbench was dead on startup for six review passes,
// invisible the whole time because nothing in CI ever executed the binary. A
// demonstration nobody runs is not a demonstration.
//
// It drives the real thing end to end and asserts on the beats that would
// disappear if the SDK regressed — the doorway crossing in particular, since
// that is the step most likely to break and the least likely to be noticed by a
// test that only checks the process exited zero.
func TestWorkbenchRuns(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out); err != nil {
		t.Fatalf("workbench failed: %v", err)
	}
	got := out.String()

	for _, want := range []string{
		"the party enters the crypt",
		// W3 holds in absolute space, and the map that reports it no longer
		// names the rooms on either side (rpg-toolkit#1042). Row 1 is one of
		// the two authored rows whose axial cells read the same as their
		// offsets under pointy-top, which is why the gate is legible here.
		"gate: (5,1) kisses (6,1)",
		"one hex map: 72 cells",
		"alice enters (5,1)",           // the walk reached the doorway
		"alice steps through to (6,1)", // and crossed it — a doorway is a step

		// A fight that starts itself, end to end. Each of these is a different
		// claim, and any one regressing would leave the others still true:
		"a fight starts, in order [alice skel-1 wight]",       // contact at the doorway starts it, unasked
		"she walks 2 step(s) into the vault, on her own turn", // the active member still walks (rpg-toolkit#1169)
		"bob walks on regardless, 1 step(s)",                  // while everyone not in it carries on

		`ended by "withdraw"`,
		"alice at (6,2)", // authored [7,2] on the map: where her own turn's walk left her, and it persisted
	} {
		if !strings.Contains(got, want) {
			t.Errorf("workbench output missing %q\nfull output:\n%s", want, got)
		}
	}
}
