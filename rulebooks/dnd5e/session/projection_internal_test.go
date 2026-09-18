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

// The view a host's driver reads has to cross the boundary intact.
//
// A driver written against this package's own interface is handed a view that
// has been flattened out of the composition's. A field the flattening drops
// does not fail: the driver simply decides with less than the encounter knew,
// and the first anybody hears of it is a monster that stands still, or shoots
// the wrong player, on a board where nothing errored.
//
// So the projection is asserted as a whole value rather than field by field.
// A new field added to the twin and forgotten in the projection fails here,
// which is the only place it can fail cheaply.
//
// THIS USED TO BE A ROUND TRIP, and the half that is gone says something. The
// reference driver and the minded one were written against the composition's
// own view, so this package rebuilt one to hand them; both are deleted
// (rpg-project#465) and the one driver that still reads the composition's view
// takes it directly rather than through a rebuild (see [tableDriver]). What is
// left is one direction, which is the only direction a host's driver has ever
// travelled in.
func TestMonsterViewCarriesEveryFieldAHostsDriverReads(t *testing.T) {
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

	require.Equal(t, MonsterView{
		Self:      "skeleton",
		Position:  spatial.Position{X: 6, Y: 4},
		Targeting: "closest",
		Actions: []ActionView{
			{Ref: blade.String(), Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
			{Ref: bow.String(), Name: "Shortbow", RangeFeet: 80, Kind: "ranged"},
		},
		Holdings: []Holding{
			{
				Subject: "alice", Payload: sight, Channel: string(perception.Sight),
				Observed: 1, Confirmed: 7, CurrentVia: []string{string(perception.Sight)},
			},
			{
				Subject: "deeds|alice", Payload: []byte(`{"kind":"deed","verb":"attack"}`),
				Channel: "deeds", Observed: 7, Confirmed: 7,
			},
		},
		At: 7,
		Seen: []SeenMember{{
			ID: "alice", Kind: KindPlayer, Standing: true,
			Position: aliceAt, DistanceCells: 4,
			InReach:  map[string]bool{bow.String(): true, blade.String(): false},
			Path:     []spatial.Position{{X: 6, Y: 3}},
			AwayPath: []spatial.Position{{X: 6, Y: 5}},
		}},
		Remembered: []RememberedMember{{
			ID: "bob", Kind: KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, DistanceCells: 6,
			Path: []spatial.Position{{X: 5, Y: 4}},
		}},
		Budget: TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Round:  3,
	}, projectMonsterView(original))
}

// The creature's table does NOT cross this boundary, and that is the whole
// argument for [tableDriver] existing (rpg-project#465).
//
// A host's driver is handed a view with no table, no temperament and no deeds
// in it. So a table driver projected through this twin would roll an empty
// table and hold, silently, every turn — which is why the one driver that rolls
// takes the composition's own view instead of coming through here.
//
// This test is what makes that argument checkable rather than a claim in a
// comment: if somebody twins the table onto this view, the reason [tableDriver]
// gives for existing has lapsed and this test says so.
func TestTheTableDoesNotCrossToAHostsDriver(t *testing.T) {
	projected := projectMonsterView(encounter.MonsterView{
		Self:   "goblin-1",
		Table:  encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}},
		Temper: encounter.Temper{Word: "coward", Profile: encounter.TemperProfile{Away: 300}},
		Deeds: []encounter.HeldDeed{
			{Kind: encounter.DeedAttack, Actor: "alice", At: 3},
		},
	})

	require.Equal(t, MonsterView{
		Self:       "goblin-1",
		Actions:    []ActionView{},
		Seen:       []SeenMember{},
		Remembered: []RememberedMember{},
	}, projected,
		"a host's driver is told what it can act on, and a table is not one of those things")
}

// The fields a driver reads testimony through get their own assertions on this
// package's own twin.
//
// The whole-value check above would fail if any of them were dropped, but it
// would fail the same way for every other field too. These say what the view
// is FOR: every holding as values, the clock's high-water, and a step away.
func TestTheTestimonyFieldsCrossOnTheWayIn(t *testing.T) {
	aliceAt := spatial.Position{X: 6, Y: 0}
	view := projectMonsterView(encounter.MonsterView{
		Self: "skeleton",
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
