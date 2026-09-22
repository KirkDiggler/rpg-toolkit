// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// bothways_internal_test.go is THE ORDER THE AGGRESSION LAW RUNS IN
// (rpg-project#493, R3), which is the one claim about it that cannot be made
// from outside: the pair has to turn BEFORE the deed lands, or the very pick
// the swing provokes reads `enemy: none`.
//
// A creature's own facts are the projection its table is picked against
// ([Encounter.factsFor]), and they are unexported because a host never asks
// for them — a Driver is handed a view and nothing else. Reading them here is
// reading exactly what the goblin's `time` table will read a moment later.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestTheProvokedCampReadsAnEnemyAndADeedInOnePick is the whole reason
// [Encounter.aggression] sits ahead of the deed rather than behind it.
//
// The goblin's shipped table has two rows that matter here — "attacked within
// 3 -> attack: attacker" and "enemy: reach -> attack: enemy" — and only the
// first of them used to hold after a party attacked a neutral camp. One
// aggrieved goblin, no initiative and no friends, is what the design's
// opening paragraph describes; both facts being true in the same projection
// is what replaces it.
func TestTheProvokedCampReadsAnEnemyAndADeedInOnePick(t *testing.T) {
	const camp = "goblins"
	scimitar := ActionView{
		Ref:  core.Ref{Module: "dnd5e", Type: "weapons", ID: "scimitar"},
		Name: "Scimitar", RangeFeet: 5, Kind: "melee",
	}

	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas:   CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions:  []RegionInput{rectRegion("yard", 0, 0, 6, 6)},
			Factions: []FactionInput{{ID: camp}},
			Dispositions: []DispositionInput{{
				Between: [2]FactionID{camp, FactionParty}, Stance: StanceNeutral,
			}},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30, SightFeet: 60},
			{
				ID: "watcher", Kind: KindMonster, Faction: camp, Position: spatial.Position{X: 2, Y: 1},
				SpeedFeet: 30, SightFeet: 60, Actions: []ActionView{scimitar},
			},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	before, err := enc.factsFor("watcher")
	require.NoError(t, err)
	require.False(t, before.EnemyInReach, "precondition: a civil camp reads no enemy at all")
	require.False(t, before.EnemySeen)
	require.Empty(t, before.Deeds, "precondition: nothing has been done to it")

	_, err = enc.Record(&RecordInput{
		Kind: OutcomeStruck, Actor: "alice", Targets: []MemberID{"watcher"},
		Values: map[OutcomeValue]int{ValueAmount: 7},
	})
	require.NoError(t, err)

	after, err := enc.factsFor("watcher")
	require.NoError(t, err)

	require.True(t, after.EnemyInReach,
		"the pair turned before the deed landed, so the row that says `enemy: reach` can fire")

	var attacked bool
	for _, held := range after.Deeds {
		if held.Kind == DeedAttack && held.Actor == "alice" {
			attacked = true
		}
	}
	require.True(t, attacked, "and the deed the swing landed is held against alice by name")
}
