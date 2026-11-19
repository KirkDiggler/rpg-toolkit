// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// TestAStepsGuardWordsAreDrawn: every guard word the machine and the words
// slices touched (rpg-project#1000, #1005) is said in plain words and
// connects to what it reads, so the studio draws it from connections alone
// — the verdict a `heard.claim` asks is the connection's id.
func TestAStepsGuardWordsAreDrawn(t *testing.T) {
	leaf := func(name string, op machine.Op, value any) *machine.Guard {
		return &machine.Guard{Fact: &machine.FactLeaf{Name: machine.FactName(name), Op: op, Value: value}}
	}
	for _, tc := range []struct {
		name     string
		guard    *machine.Guard
		sentence string
		conn     Connection
	}{
		{"noted, eq true", leaf("noted:admitted", machine.OpEq, true),
			"X has noted `admitted` about the one X heard (X knows who only by seeing them)", Connection{Kind: "topic", ID: "admitted", Role: "asks"}},
		{"noted, eq false", leaf("noted:admitted", machine.OpEq, false),
			"X has not noted `admitted` about the one X heard (X knows who only by seeing them)", Connection{Kind: "topic", ID: "admitted", Role: "asks"}},
		{"noted.anyone, eq true", leaf("topic:admitted", machine.OpEq, true),
			"X has noted `admitted` about anyone", Connection{Kind: "topic", ID: "admitted", Role: "wakes-on"}},
		{"noted.anyone, eq false", leaf("topic:admitted", machine.OpEq, false),
			"X has not noted `admitted` about anyone", Connection{Kind: "topic", ID: "admitted", Role: "wakes-on"}},
		{"heard.claim, eq failed", leaf("heard.claim", machine.OpEq, "failed"),
			"what X heard was a claim that failed", Connection{Kind: ConnClaim, ID: "failed", Role: "asks"}},
		{"heard.claim under a not", &machine.Guard{Not: leaf("heard.claim", machine.OpEq, "held")},
			"NOT (what X heard was a claim that held)", Connection{Kind: ConnClaim, ID: "held", Role: "asks"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, conns := guardWords(tc.guard, "X", "heard")
			require.Equal(t, tc.sentence, s)
			require.Equal(t, []Connection{tc.conn}, conns)
		})
	}
}
