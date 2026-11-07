// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// TestTheJournalFoldsAreReadBackUntilAFactLands (rpg-project#879 S3b): the
// holdings fold, a member's known facts and an observer's graph state are
// folded once per journal length. The journal is append-only, so the same
// length is the same fold; a new fact refolds. Mutants: a memo keyed on
// nothing (the second fold after the fact is stale: the new fact is missing);
// a known-facts memo that hands out its own slice (a caller's edit leaks).
func TestTheJournalFoldsAreReadBackUntilAFactLands(t *testing.T) {
	enc := sweepScene(t)
	props1, records1 := enc.holdings.fold()
	props2, records2 := enc.holdings.fold()
	require.Equal(t, len(props1), len(props2))
	require.Equal(t, len(records1), len(records2))
	require.Equal(t, enc.world.log.Len(), enc.holdings.foldedAt, "the fold is stamped with the journal length")

	state1 := enc.world.stateFor(journal.EntityID(sweepWatchA))
	require.Same(t, state1, enc.world.stateFor(journal.EntityID(sweepWatchA)), "the same length reads the same state back")

	known := enc.world.knownFacts(sweepWatchA)
	known = append(known, "a caller's own edit")
	require.NotContains(t, enc.world.knownFacts(sweepWatchA), "a caller's own edit", "the memo hands out a copy")

	before := enc.world.log.Len()
	_, err := enc.world.log.Append(journal.Fact{
		Kind: factKnownKind("guards-turned"), Actor: journal.EntityID(sweepWatchA), Subject: journal.EntityID(sweepWatchA),
		Audience: []journal.EntityID{journal.EntityID(sweepWatchA)},
	})
	require.NoError(t, err)
	require.Greater(t, enc.world.log.Len(), before, "precondition: a fact landed")
	require.Contains(t, enc.world.knownFacts(sweepWatchA), "guards-turned", "a new fact refolds")
	require.NotSame(t, state1, enc.world.stateFor(journal.EntityID(sweepWatchA)), "and refolds the graph state")
	enc.holdings.fold()
	require.Equal(t, enc.world.log.Len(), enc.holdings.foldedAt, "and restamps the holdings fold")
}

// TestAWakePassAssessesParticipationOnce (rpg-project#879 S3b): inside a wake
// pass the first ask is kept and every later ask reads it; outside a pass
// nothing is kept; a noticed down (and, in the wake, any intent other than a
// Pass) drops it so the pass assesses afresh.
// Mutant: a memo that outlives the pass (a later verb reads a stale roster).
func TestAWakePassAssessesParticipationOnce(t *testing.T) {
	enc := sweepScene(t)
	require.Nil(t, enc.wakeParticipation)
	outside, err := enc.participationNow()
	require.NoError(t, err)
	require.Nil(t, enc.wakeParticipation, "outside a wake pass nothing is kept")

	enc.worldThinking = true
	first, err := enc.participationNow()
	require.NoError(t, err)
	require.NotSame(t, outside, first)
	second, err := enc.participationNow()
	require.NoError(t, err)
	require.Same(t, first, second, "inside the pass the first ask serves")

	_, _, err = enc.noticeDown()
	require.NoError(t, err)
	require.NotSame(t, first, enc.wakeParticipation, "a noticed down drops it and the pass assesses afresh")

	third, err := enc.participationNow()
	require.NoError(t, err)
	require.NotSame(t, first, third)
	require.Same(t, enc.wakeParticipation, third, "and the fresh assessment serves the rest of the pass")
	enc.worldThinking = false
	enc.wakeParticipation = nil
}
