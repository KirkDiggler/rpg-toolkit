// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The view a mind reads has to survive the boundary intact.
//
// Every driver written one layer down — the reference one and the minded one
// alike — is handed a view that has already been flattened into this
// package's own types and then rebuilt into the composition's. A field the
// rebuild drops does not fail: the driver simply decides with less than the
// encounter knew, and the first anybody hears of it is a monster that stands
// still, or shoots the wrong player, on a board where nothing errored.
//
// So the round trip is asserted as a whole value rather than field by field.
// A new field added to either twin and forgotten in one direction fails here,
// which is the only place it can fail cheaply.
func TestMonsterViewSurvivesTheRoundTrip(t *testing.T) {
	bow := core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "shortbow"}
	blade := core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "shortsword"}
	aliceAt := spatial.Position{X: 6, Y: 0}

	sight, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationKnown, Position: aliceAt,
	})
	require.NoError(t, err)

	original := encounter.MonsterView{
		Self:      "skeleton",
		Position:  spatial.Position{X: 6, Y: 4},
		Targeting: "closest",
		Mind:      "retaliator",
		Actions: []encounter.ActionView{
			{Ref: blade, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
			{Ref: bow, Name: "Shortbow", RangeFeet: 80, Kind: "ranged"},
		},
		Holdings: []perception.Holding{
			// Sustained testimony: somebody is looking at alice right now.
			{
				Subject: "alice", Payload: sight, Channel: perception.Sight,
				Observed: 1, Confirmed: 7, CurrentVia: []perception.Channel{perception.Sight},
			},
			// Discrete testimony on another channel, current on nothing —
			// the shape the deeds channel actually lands in, and the one a
			// projection is most likely to quietly normalise.
			{
				Subject: "deeds|alice", Payload: []byte(`{"kind":"deed","verb":"attack"}`),
				Channel: "deeds", Observed: 7, Confirmed: 7,
			},
		},
		At: 7,
		Seen: []encounter.SeenMember{{
			ID: "alice", Kind: encounter.KindPlayer, Standing: true,
			Position: aliceAt, DistanceCells: 4,
			InReach:  map[core.Ref]bool{bow: true, blade: false},
			Path:     []spatial.Position{{X: 6, Y: 3}},
			AwayPath: []spatial.Position{{X: 6, Y: 5}},
		}},
		Remembered: []encounter.RememberedMember{{
			ID: "bob", Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, DistanceCells: 6,
			Path: []spatial.Position{{X: 5, Y: 4}},
		}},
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Round:  3,
	}

	back, err := unprojectMonsterView(projectMonsterView(original))
	require.NoError(t, err)
	require.Equal(t, original, back)
}

// The fields a mind reads are the ones this wave added, so they get their own
// assertions on this package's own twin.
//
// The whole-value check above would fail if any of them were dropped, but it
// would fail the same way for every other field too. These say what the view
// is FOR: the sheet's word, every holding as values, the clock's high-water,
// and a step away.
func TestTheMindsOwnFieldsCrossOnTheWayIn(t *testing.T) {
	aliceAt := spatial.Position{X: 6, Y: 0}
	view := projectMonsterView(encounter.MonsterView{
		Self: "skeleton",
		Mind: "retaliator",
		At:   7,
		Holdings: []perception.Holding{{
			Subject: "deeds|alice", Payload: []byte("payload"), Channel: "deeds",
			Observed: 7, Confirmed: 7,
		}},
		Seen: []encounter.SeenMember{{
			ID: "alice", Position: aliceAt,
			AwayPath: []spatial.Position{{X: 6, Y: 5}},
		}},
	})

	require.Equal(t, "retaliator", view.Mind, "the sheet's word, verbatim")
	require.Equal(t, uint64(7), view.At, "the clock's high-water")
	require.Equal(t, []Holding{{
		Subject: "deeds|alice", Payload: []byte("payload"), Channel: "deeds",
		Observed: 7, Confirmed: 7,
	}}, view.Holdings, "every holding, on every channel, as values")
	require.Equal(t, []spatial.Position{{X: 6, Y: 5}}, view.Seen[0].AwayPath,
		"and the step away the encounter already worked out")
}

// A holding's payload and channel list are COPIES, not the encounter's own
// backing arrays.
//
// The view is data a driver may keep, log or replay (its own doc says so),
// and the encounter goes on to run more passes against the same store. A
// shared array would let one of them rewrite what the other already read.
func TestAHoldingCrossesAsACopy(t *testing.T) {
	payload := []byte("payload")
	via := []perception.Channel{perception.Sight}

	view := projectMonsterView(encounter.MonsterView{
		Holdings: []perception.Holding{{
			Subject: "alice", Payload: payload, Channel: perception.Sight, CurrentVia: via,
		}},
	})

	payload[0] = 'X'
	via[0] = "hearing"

	require.Equal(t, []byte("payload"), view.Holdings[0].Payload)
	require.Equal(t, []string{string(perception.Sight)}, view.Holdings[0].CurrentVia)
}
