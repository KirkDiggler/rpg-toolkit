// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// fell_internal_test.go is the fall read back the way a table reads it
// (rpg-project#716): the projection carries who fell, a `fell: { who, on:
// ally }` row holds for the watcher who saw its campmate drop, and the `who`
// survives the persisted shape.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// fellEnc is a yard with a watcher and its campmate, alice who drops the
// campmate, and a standing the test flips.
func fellEnc(t *testing.T, standing *somebodyDown) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{
			Canvas:   CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions:  []RegionInput{rectRegion("yard", 0, 0, 8, 8)},
			Factions: []FactionInput{{ID: "watch"}},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "watcher", Kind: KindMonster, Faction: "watch", Position: spatial.Position{X: 3, Y: 1}},
			{ID: "friend", Kind: KindMonster, Faction: "watch", Position: spatial.Position{X: 4, Y: 1}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     UnobservedEquipment{},
			Conditions:    UnobservedEquipment{},
			Sheets:        sheetFacts{"alice": {SpeedFeet: 30}, "watcher": {SpeedFeet: 30}, "friend": {SpeedFeet: 30}},
			Participation: standing,
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return enc
}

// TestAWatchersFellRowHoldsForTheCampmateItSawFall: alice drops the
// watcher's campmate in its sight. The watcher's ally reading holds a fall
// naming alice and the campmate, and a `fell` row naming the campmate holds
// while one naming somebody else does not.
func TestAWatchersFellRowHoldsForTheCampmateItSawFall(t *testing.T) {
	standing := &somebodyDown{}
	enc := fellEnc(t, standing)
	standing.down = []MemberID{"friend"}
	strike(t, enc, "alice", "friend")

	facts, err := enc.factsFor("watcher")
	require.NoError(t, err)

	var fell []HeldDeed
	for _, d := range facts.AllyDeeds {
		if d.Kind == DeedFell {
			fell = append(fell, d)
		}
	}
	require.Len(t, fell, 1, "the watcher saw its campmate fall")
	require.Equal(t, MemberID("alice"), fell[0].Actor, "the one who dropped it")
	require.Equal(t, MemberID("friend"), fell[0].Target, "and who fell, as the projection carries it")

	require.Equal(t, 0, fellPick(t, facts, "friend"), "`fell: { who: friend, on: ally }` holds for the watcher")
	require.Equal(t, 1, fellPick(t, facts, "alice"), "a fall names who fell, not who dropped them")
}

// fellPick rolls a table whose first row waits on who's fall and whose
// second is a standing hold, on the lowest face: 0 when the fall holds.
func fellPick(t *testing.T, facts Facts, who MemberID) int {
	t.Helper()
	table := Table{AnswerTime: {
		{Weight: 1, Hold: true, When: []When{{Deed: "fell", Within: 3, Who: who, Scope: ScopeAlly}}},
		{Weight: 1, Hold: true},
	}}
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: AnswerTime, Table: table, Facts: facts, Die: lowestDie{},
	})
	require.NoError(t, err)

	return chosen.Entry
}

// TestWhoSurvivesPersistence: a `who` row saved and loaded back still reads
// one member's fall, not every fall.
func TestWhoSurvivesPersistence(t *testing.T) {
	entry := Answer{Weight: 1, Hold: true, When: []When{{Deed: "fell", Within: 3, Who: "king", Scope: ScopeAlly}}}

	data := answerDataFrom(entry)
	require.Equal(t, "king", data.When[0].Who, "the condition writes who")

	back := answerFromData(data)
	require.Equal(t, entry.When, back.When, "and comes back whole")

	unnamed := answerDataFrom(Answer{Weight: 1, Hold: true, When: []When{{Deed: "attacked", Within: 3}}})
	require.Empty(t, unnamed.When[0].Who, "a condition with no who writes no key")
}

// TestAProjectedFallKeepsItsTarget is the projection a driver is handed
// (rpg-toolkit#2149 review, item c): the view a creature's turn is driven
// from carries each reading's fall with its target, the same as factsFor's.
func TestAProjectedFallKeepsItsTarget(t *testing.T) {
	standing := &somebodyDown{}
	enc := fellEnc(t, standing)
	standing.down = []MemberID{"friend"}
	strike(t, enc, "alice", "friend")

	view, err := enc.buildMonsterView(enc.members["watcher"], TurnBudget{}, 0)
	require.NoError(t, err)
	var targets []MemberID
	for _, d := range view.AllyDeeds {
		if d.Kind == DeedFell {
			targets = append(targets, d.Target)
		}
	}
	require.Equal(t, []MemberID{"friend"}, targets, "the watcher's view names who fell")

	own, err := enc.buildMonsterView(enc.members["friend"], TurnBudget{}, 0)
	require.NoError(t, err)
	var mine []MemberID
	for _, d := range own.Deeds {
		if d.Kind == DeedFell {
			mine = append(mine, d.Target)
		}
	}
	require.Equal(t, []MemberID{"friend"}, mine, "and the fallen's own reading names itself")
}
