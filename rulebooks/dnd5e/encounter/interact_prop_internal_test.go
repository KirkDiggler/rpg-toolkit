// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestAHeldDoerIsRefusedTheThingWhole (rpg-project#959): on the escape site,
// the prisoner the guards hold does a thing whose row would set them down in
// the cell, and it is refused whole — the guards' sentence, nothing written,
// nobody moved.
func TestAHeldDoerIsRefusedTheThingWhole(t *testing.T) {
	no := false
	hatch := PropInput{
		ID: "hatch", Ref: "dnd5e:props:test:hatch", At: spatial.Position{X: 3, Y: 2},
		BlocksMovement: &no, BlocksLineOfSight: &no,
		Options: []Option{{
			ID: "drop", Do: "You drop through the hatch.", Success: []Answer{{Weight: 1}},
			Grants: []RowGrant{{Consequence: Consequence{Relocate: &RelocateGrant{To: escCell, Words: "Down you go."}}}},
		}},
	}
	enc := escapeSiteWith(t, CustodyHeld, spatial.Position{X: 3, Y: 1}, []PropInput{hatch}, nil)
	where := func() spatial.Position {
		at, ok := enc.canvas.GetEntityPosition(string(escPris))
		require.True(t, ok)
		return at
	}
	before := where()
	story, err := enc.Story(&StoryInput{Audience: escPris})
	require.NoError(t, err)

	_, err = enc.Interact(&InteractInput{Actor: escPris, Prop: "hatch", OptionID: "drop", Range: 3, Roller: lowestDie{}})
	require.ErrorIs(t, err, ErrInCustody)
	require.Equal(t, before, where(), "nobody moved")
	after, err := enc.Story(&StoryInput{Audience: escPris})
	require.NoError(t, err)
	require.Len(t, after, len(story), "the thing wrote nothing")
}

// TestAWatcherTheSneakerBeatLearnsNothing (rpg-project#959): the friend,
// who knows the hatch's secret, sneaks with a won Stealth verdict against
// the guard and the captain — the record a beaten roll leaves, which every
// sight reach reads — and opens the hidden hatch in the yard, in plain line
// of both and of the hatch. Neither is a witness: neither learns the secret,
// and neither is told any version of the deed.
func TestAWatcherTheSneakerBeatLearnsNothing(t *testing.T) {
	no := false
	hatch := PropInput{
		ID: "hatch", Ref: "dnd5e:props:test:hatch", At: spatial.Position{X: 3, Y: 2},
		BlocksMovement: &no, BlocksLineOfSight: &no,
		Options: []Option{{ID: "open", Do: "You ease the hatch open.", Success: []Answer{{Weight: 1}}}},
	}
	secret := ConcealmentInput{ID: "hatchway", Checks: []CheckApproach{{Ability: "investigation", DC: 30}}, Props: []PropID{"hatch"}}
	enc := escapeSiteWith(t, CustodyHeld, spatial.Position{X: 1, Y: 1}, []PropInput{hatch}, []ConcealmentInput{secret})
	c := enc.field.concealmentOf("hatchway")
	require.NoError(t, enc.revealConcealmentTo(escFriend, c, "test", 0))
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	hatchCell := enc.field.cellAt(hatch.At)
	for _, w := range []MemberID{escGuard, escCaptain} {
		require.True(t, reach.Reaches(perception.Sight, w, escFriend), "precondition: %s sees the friend", w)
		require.True(t, reach.reachesCell(w, hatchCell), "precondition: %s sees the hatch's cell", w)
	}

	enc.members[escFriend].Sneaking = true
	enc.sneakVerdicts = map[MemberID]map[MemberID]SneakVerdictData{escFriend: {
		escGuard: {Beaten: true, InZone: true}, escCaptain: {Beaten: true, InZone: true},
	}}
	reach, err = enc.placedSightReach()
	require.NoError(t, err)
	require.False(t, reach.Reaches(perception.Sight, escGuard, escFriend), "precondition: the sneak's verdict hides them")

	out, err := enc.Interact(&InteractInput{Actor: escFriend, Prop: "hatch", OptionID: "open", Range: 3, Roller: lowestDie{}})
	require.NoError(t, err)
	require.Equal(t, []MemberID{escFriend}, out.Witnesses, "nobody saw the sneaker")
	for _, w := range []MemberID{escGuard, escCaptain} {
		require.False(t, enc.world.knowsConcealment(w, "hatchway"), "%s did not see it used", w)
		story, err := enc.Story(&StoryInput{Audience: w})
		require.NoError(t, err)
		for _, entry := range story {
			require.NotContains(t, string(entry.Payload), `"interacted"`, "%s is told no version of it", w)
		}
	}
}
