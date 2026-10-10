// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestEachCrossingTellsItsOwnConcentration: one advance ends A's turn and
// starts B's, and B's concentration lapses at B's start. The break is B's, so
// it is told with B as actor — never with whichever crossing came first or
// last — and a check C saved at C's start stays C's.
func TestEachCrossingTellsItsOwnConcentration(t *testing.T) {
	crossed := []encounter.Boundary{
		{Kind: encounter.TurnEnded, Subject: "alice", Round: 1},
		{Kind: encounter.TurnStarted, Subject: "bob", Round: 1},
		{Kind: encounter.TurnStarted, Subject: "carol", Round: 1},
	}
	carolCheck := encounter.ConcentrationCheck{Save: encounter.CastSave{Saver: "carol"}}
	bobBreak := encounter.ConcentrationBreak{Caster: "bob"}
	strayBreak := encounter.ConcentrationBreak{Caster: "dave"}

	tells := boundaryConcentration(crossed, concentration{
		Checks: []encounter.ConcentrationCheck{carolCheck},
		Breaks: []encounter.ConcentrationBreak{bobBreak, strayBreak},
	})
	require.Equal(t, []concentrationTell{
		{actor: "bob", told: concentration{Breaks: []encounter.ConcentrationBreak{bobBreak}}},
		{actor: "carol", told: concentration{Checks: []encounter.ConcentrationCheck{carolCheck}}},
		{actor: "dave", told: concentration{Breaks: []encounter.ConcentrationBreak{strayBreak}}},
	}, tells)

	require.Nil(t, boundaryConcentration(crossed, concentration{}), "nothing to tell, nobody told")
}
