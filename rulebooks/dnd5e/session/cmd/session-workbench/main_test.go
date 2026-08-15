// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"
)

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
		"gate: antechamber (5,1) kisses vault (6,1)", // W3 holds in absolute space
		"alice enters (5,1)",                         // the walk reached the doorway
		"antechamber (5,1) → vault (0,1)",            // and crossed it

		// A fight that starts itself, end to end. Each of these is a different
		// claim, and any one regressing would leave the others still true:
		"a fight starts, in order [alice skel-1 wight]", // contact at the doorway starts it, unasked
		"free roam refused for a fight member: true",    // and a fighter stops free-roaming
		"bob walks on regardless, 1 step(s)",            // while everyone not in it carries on

		`ended by "withdraw"`,
		"alice in vault at (0,1)", // she is where the fight stopped her, and it persisted
	} {
		if !strings.Contains(got, want) {
			t.Errorf("workbench output missing %q\nfull output:\n%s", want, got)
		}
	}
}
