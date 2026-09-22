// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// placedfailclosed_internal_test.go is THE GUARD ON A DERIVED FIELD
// (rpg-api-protos#351, PR #1866 review finding 1).
//
// [AtlasPlacedProp.Cells] is filled in exactly one place — [Encounter.Atlas]
// — and [field.placedCells] is never empty for a compiled field, so the
// concealment filter cannot meet an unfilled one today. The guard exists
// because of WHICH WAY the zero value falls: an empty cell list reads as
// "stands on no hidden cell", so a placement that forgot to say where it
// stands would be PRESENTED to a recipient who cannot see its floor, and the
// rectangle would mark the secret the concealment was keeping.
//
// That is a silent leak arriving through a second construction site nobody
// re-read this filter for — rpg-api maps this exact type on the wire half of
// the same issue. So the absent value is made to mean what its author meant:
// "I cannot say where this stands", which is a reason to withhold it.
//
// Internal because [Encounter.placedTouchesHidden] is the decision, and the
// leak it guards cannot be staged through [Encounter.AtlasFor] — the whole
// point of the finding is that no reachable path produces the input yet.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// failClosedEncounter is a plain field: the filter's decision is about the
// placement and the hidden set it is handed, and needs no secret of its own
// to be asked.
func failClosedEncounter(t *testing.T) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("hall", 0, 0, 6, 6)},
		},
		Members: []MemberInput{{ID: "walker", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 0}}},
		Endings: []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	return enc
}

// TestAPlacementThatNamesNoCellIsWithheldRatherThanPresented.
//
// Three asks of one decision, because the guard is only honest if it fires on
// exactly the case it is for: a placement standing on visible floor is still
// presented, a placement that names no cell at all is withheld, and a field
// hiding nothing still withholds nothing from either.
func TestAPlacementThatNamesNoCellIsWithheldRatherThanPresented(t *testing.T) {
	enc := failClosedEncounter(t)
	secret := spatial.Position{X: 2, Y: 2}
	hidden := map[spatial.Position]bool{secret: true}

	standingInTheOpen := AtlasPlacedProp{ID: "table", Cells: []spatial.Position{{X: 0, Y: 0}}}
	require.False(t, enc.placedTouchesHidden(standingInTheOpen, hidden),
		"a placement standing on floor this recipient can see is presented")

	standingOnTheSecret := AtlasPlacedProp{ID: "table", Cells: []spatial.Position{secret}}
	require.True(t, enc.placedTouchesHidden(standingOnTheSecret, hidden),
		"and one standing on hidden floor is withheld, which is the rule this guards")

	saysNothing := AtlasPlacedProp{ID: "table"}
	require.True(t, enc.placedTouchesHidden(saysNothing, hidden),
		"a placement that names no cell cannot be shown to be safe, so it is withheld — "+
			"the empty list must not read as \"stands on no hidden cell\"")

	require.False(t, enc.placedTouchesHidden(saysNothing, nil),
		"but a field that hides nothing withholds nothing, whatever a placement says about itself")
}
