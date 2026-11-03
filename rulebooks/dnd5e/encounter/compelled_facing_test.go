// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// compelled_facing_test.go is rpg-project#829 (design R2) on the castle:
// FACING FOLLOWS THE LEGS. A Dissonant Whispers flee is walked by the
// creature's own legs, so it turns to face its flight and stays facing away;
// a Thunderwave shove moves a body without its legs, so it keeps the facing
// it had. A Command flee is a Routed walk and already turned
// (TestARoutedWalkIsTheWalkersOwn, unchanged).

var cfWhispers = core.Ref{Module: "dnd5e", Type: "spells", ID: "dissonant-whispers"}

// TestAWhisperedGuardTurnsToFaceItsFlightOnTheCastle: the captain fails the
// bard's Dissonant Whispers and runs its speed away from her. It ends facing
// its last step, the bard sees it facing that way, and nothing turns it back.
func TestAWhisperedGuardTurnsToFaceItsFlightOnTheCastle(t *testing.T) {
	enc := openTheGatehouseForTheBard(t)
	const guard = encounter.MemberID("captain")
	seated := memberOf(t, enc, guard).Facing

	route, err := enc.Route(encounter.RouteInput{
		Mover: guard, Policy: encounter.MoveAway, Anchor: whereIs(t, enc, clBard), Budget: 6,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(route.Path), 2, "the captain has somewhere to flee")

	out, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: guard, Cause: cfWhispers, Route: route.Path, Kind: encounter.MoveCompelled,
	})
	require.NoError(t, err)
	require.Equal(t, len(route.Path), out.Moved, "the whole flight is walked")

	from, to := route.Path[len(route.Path)-2], route.Path[len(route.Path)-1]
	flight := stepBearing(t, from, to)
	require.NotEqual(t, seated, flight, "precondition: the flight is not the way it already faced")

	assert.InDelta(t, flight, memberOf(t, enc, guard).Facing, 1e-9, "it ends facing its last step, away")
	if seen := heldFacingOf(t, enc, clBard, guard); assert.NotNil(t, seen, "the bard sees which way it faces") {
		assert.InDelta(t, flight, *seen, 1e-9, "and what she sees is the flight, with no snap back")
	}
}

// TestAThunderwavedGuardKeepsItsFacingOnTheCastle: the same captain, shoved
// two cells away from the bard by Thunderwave, keeps the facing it was seated
// with. The shove is walked through Direct with the displaced kind, which is
// the seam RecordCast's pushes walk through (walkCastPushes); calling it
// straight keeps the fight a hostile cast forms, and the captain's own first
// turn inside it, out of the reading.
func TestAThunderwavedGuardKeepsItsFacingOnTheCastle(t *testing.T) {
	enc := openTheGatehouseForTheBard(t)
	const guard = encounter.MemberID("captain")
	seated := memberOf(t, enc, guard).Facing

	_, push := thunderwaveOn(t, enc, guard)
	require.Len(t, push.Route, 2, "precondition: the shove is two cells")
	shove := stepBearing(t, push.Route[0], push.Route[1])
	require.NotEqual(t, seated, shove, "precondition: a step of its own along the shove would have turned it")

	out, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: guard, Cause: core.Ref{Module: "dnd5e", Type: "spells", ID: "thunderwave"},
		Route: push.Route, Kind: encounter.MoveDisplaced,
	})
	require.NoError(t, err)
	require.Equal(t, len(push.Route), out.Moved, "the whole shove lands")

	assert.InDelta(t, seated, memberOf(t, enc, guard).Facing, 1e-9, "a shoved body keeps the facing it had")
	if seen := heldFacingOf(t, enc, clBard, guard); assert.NotNil(t, seen, "the bard sees which way it faces") {
		assert.InDelta(t, seated, *seen, 1e-9, "and what she sees is the facing it had")
	}
}
