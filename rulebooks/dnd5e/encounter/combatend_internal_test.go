// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
)

// TestCombatEndBoundariesRefusesADissolutionTheClockDidNotReport pins the loud
// half of combatEndBoundaries' contract, which is unreachable from outside:
// bubble.Dissolve always stamps its milestone, so production never gets here.
//
// WHITE-BOX for the reason clocks_internal_test's own subject is — the branch
// exists as a net under an invariant of another package, and the only way to
// stand under it is to hand it the state that package promises never to
// produce.
//
// It matters more than an unreachable branch usually would. The alternative
// implementation — return no boundaries and no error — reads as harmless and
// is not: a fight would end announcing nothing, which is bit-for-bit the state
// combat end was already in before rpg-project#295 and is exactly how it stayed
// there unnoticed for months.
func TestCombatEndBoundariesRefusesADissolutionTheClockDidNotReport(t *testing.T) {
	_, err := combatEndBoundaries(nil, []MemberID{"alice"})
	require.ErrorIs(t, err, ErrNoDissolveMilestone)

	// And a set holding OTHER milestones is equally not a dissolution. A turn
	// ending in the list does not make the list say a fight ended, and reading
	// "the round" off whatever happens to be first would be how that mistake
	// looks in code.
	_, err = combatEndBoundaries(
		[]clock.Milestone{{Kind: clock.TurnEnded, Subject: "alice", Round: 3}},
		[]MemberID{"alice"},
	)
	require.ErrorIs(t, err, ErrNoDissolveMilestone)
}

// TestCombatEndBoundariesReadsItsRoundFromTheDissolution keeps the one number
// in the answer tied to the one milestone that means it.
//
// The decoy is the test. A TurnEnded carrying a DIFFERENT round sits ahead of
// the dissolution in the list, so an implementation that reads the round off
// the first milestone — or off the last — produces 3 or 9 rather than 4 and
// fails here.
func TestCombatEndBoundariesReadsItsRoundFromTheDissolution(t *testing.T) {
	crossed, err := combatEndBoundaries(
		[]clock.Milestone{
			{Kind: clock.TurnEnded, Subject: "alice", Round: 3},
			{Kind: clock.Dissolved, Round: 4},
			{Kind: clock.TurnStarted, Subject: "goblin", Round: 9},
		},
		[]MemberID{"alice", "goblin"},
	)
	require.NoError(t, err)
	require.Equal(t, []Boundary{
		{Kind: CombatEnded, Subject: "alice", Round: 4},
		{Kind: CombatEnded, Subject: "goblin", Round: 4},
	}, crossed, "one boundary per member, all carrying the round the fight ended on")
}

// TestAFightEndingForNobodyAnnouncesNothing — an empty roster is not an error,
// unlike a missing milestone. clock.Turn.Dissolve refuses an already-empty
// clock (ErrIdle) so this cannot arrive from production either, but the two
// absences mean opposite things and the code should say so: no members is a
// fight that ended for nobody, which is nothing to announce; no milestone is
// the clock failing to say what it did.
func TestAFightEndingForNobodyAnnouncesNothing(t *testing.T) {
	crossed, err := combatEndBoundaries([]clock.Milestone{{Kind: clock.Dissolved, Round: 2}}, nil)
	require.NoError(t, err)
	require.Empty(t, crossed)
}
