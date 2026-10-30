// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// ally_internal_test.go is `ally` at the driver and on the view
// (rpg-project#717, lane D2): the nearest member on a side this creature's
// faction is allied with, through a disposition, in its own sight else its
// memory. Kin of its own faction are not allies, an ally is never in reach,
// and the walk to one ends beside it.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

var allyCrossbow = core.Ref{Type: "weapons", ID: "heavy-crossbow"}

// follower is a companion's table: with no foe, follow; otherwise hold.
// Rolled on the lowest face, the first candidate fires.
func follower() Table {
	return Table{AnswerTime: {
		{Weight: 1, When: []When{{Enemy: behavior.EnemyNone}}, Toward: &Selector{Word: SelectorAlly}},
		{Weight: 1, Hold: true},
	}}
}

// followView is a companion with the turn's movement and nobody opposed.
func followView() MonsterView {
	return MonsterView{
		Self:    "veteran",
		Table:   follower(),
		Actions: []ActionView{{Ref: allyCrossbow, RangeFeet: 100}},
		At:      5,
		Budget:  TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// TestFollowWalksTowardTheNearestAlly: two allies in sight and a fallen one
// nearer still, the nearer standing one is walked toward, as far as the turn's movement goes, and the walk names
// whom it closes on.
func TestFollowWalksTowardTheNearestAlly(t *testing.T) {
	view := followView()
	near := []spatial.Position{{X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}, {X: 4, Y: 0}, {X: 5, Y: 0}, {X: 6, Y: 0}, {X: 7, Y: 0}}
	view.Seen = []SeenMember{
		{ID: "bard", Allied: true, Standing: true, DistanceCells: 12, Path: []spatial.Position{{X: 0, Y: 1}}},
		{ID: "hero", Allied: true, Standing: true, DistanceCells: 8, Path: near},
		// A body is nobody to follow, however near.
		{ID: "fallen", Allied: true, Standing: false, DistanceCells: 1, Path: []spatial.Position{{X: 1, Y: 1}}},
	}
	decision, err := TableDriver{Roller: lowestDie{}}.Act(view)
	require.NoError(t, err)
	require.Equal(t, 0, decision.Pick.Entry, "the follow row fires")
	require.Equal(t, Move{Path: near[:6], Target: "hero"}, decision.Intent)
}

// TestAFollowerBesideItsAllyStaysAndSaysSo: an ally in sight with no route
// left to walk is an ally already beside it; the follower stands, and says
// why, rather than passing in silence.
func TestAFollowerBesideItsAllyStaysAndSaysSo(t *testing.T) {
	view := followView()
	view.Seen = []SeenMember{{ID: "hero", Allied: true, Standing: true, DistanceCells: 1}}
	decision, err := TableDriver{Roller: lowestDie{}}.Act(view)
	require.NoError(t, err)
	require.Equal(t, 0, decision.Pick.Entry)
	require.Equal(t, Pass{Cause: tableCauseToward, Why: towardBesideAlly}, decision.Intent)
}

// TestFollowFollowsAMemoryWhenNoAllyIsInSight: the party walked out of
// sight; the follower walks to where it last saw them.
func TestFollowFollowsAMemoryWhenNoAllyIsInSight(t *testing.T) {
	view := followView()
	trail := []spatial.Position{{X: 0, Y: 1}, {X: 0, Y: 2}}
	view.Remembered = []RememberedMember{{ID: "hero", Allied: true, DistanceCells: 2, Path: trail}}
	decision, err := TableDriver{Roller: lowestDie{}}.Act(view)
	require.NoError(t, err)
	require.Equal(t, 0, decision.Pick.Entry)
	require.Equal(t, Move{Path: trail, Target: "hero"}, decision.Intent)
}

// TestFollowIsAbsentWithNobodyAlliedHeld: sightings that are not allied —
// a neutral stranger, kin, a body — leave the follow row off the table, and
// the hold fires.
func TestFollowIsAbsentWithNobodyAlliedHeld(t *testing.T) {
	view := followView()
	view.Seen = []SeenMember{
		{ID: "stranger", Standing: true, DistanceCells: 3, Path: []spatial.Position{{X: 1, Y: 0}}},
		{ID: "fallen", Allied: true, Standing: false, DistanceCells: 2, Path: []spatial.Position{{X: 1, Y: 0}}},
	}
	decision, err := TableDriver{Roller: lowestDie{}}.Act(view)
	require.NoError(t, err)
	require.Len(t, decision.Pick.Candidates, 1, "only the hold is a candidate")
	require.Equal(t, 1, decision.Pick.Entry)
	require.Equal(t, Pass{}, decision.Intent)
}

// TestAnAllyIsNeverStruck: `attack: ally` is refused at the composition's
// door for every caller, a persisted table included.
func TestAnAllyIsNeverStruck(t *testing.T) {
	err := validateTable(Table{AnswerTime: {{Weight: 1, Attack: &Selector{Word: SelectorAlly}}}})
	require.ErrorIs(t, err, ErrBadAnswer)
	require.ErrorIs(t, err, behavior.ErrBadSelector)
	require.Contains(t, err.Error(), "`attack` cannot take `ally`")
	for _, word := range []string{"toward", "dash", "away"} {
		row := Answer{Weight: 1}
		sel := &Selector{Word: SelectorAlly}
		switch word {
		case "toward":
			row.Toward = sel
		case "dash":
			row.Dash = sel
		case "away":
			row.Away = sel
		}
		require.NoError(t, validateTable(Table{AnswerTime: {row}}), "%s: ally", word)
	}
}

// alliesEnc is a yard with a veteran in `veterans`, his kin beside him, a
// hero in the party, and a stranger in no faction, where the party and the
// veterans stand as the caller says.
func alliesEnc(t *testing.T, party Stance) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas:       CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions:      []RegionInput{rectRegion("yard", 0, 0, 16, 4)},
			Factions:     []FactionInput{{ID: "veterans"}},
			Dispositions: []DispositionInput{{Between: [2]FactionID{FactionParty, "veterans"}, Stance: party}},
		},
		Members: []MemberInput{
			{ID: "veteran", Kind: KindMonster, Faction: "veterans", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "kin", Kind: KindMonster, Faction: "veterans", Position: spatial.Position{X: 2, Y: 1}},
			{ID: "hero", Kind: KindPlayer, Position: spatial.Position{X: 10, Y: 1}},
			{ID: "stranger", Kind: KindMonster, Position: spatial.Position{X: 1, Y: 2}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  UnobservedEquipment{},
			Conditions: UnobservedEquipment{},
			Sheets: sheetFacts{
				"veteran":  {SpeedFeet: 30, Actions: []ActionView{{Ref: allyCrossbow, RangeFeet: 100}}},
				"kin":      {SpeedFeet: 30},
				"hero":     {SpeedFeet: 30},
				"stranger": {SpeedFeet: 30},
			},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)

	return enc
}

// seenOf is one sighting off a view, by id.
func seenOf(t *testing.T, view MonsterView, id MemberID) SeenMember {
	t.Helper()
	for _, s := range view.Seen {
		if s.ID == id {
			return s
		}
	}
	require.FailNowf(t, "not seen", "%s", id)

	return SeenMember{}
}

// TestTheViewNamesAnAllyAcrossADispositionNotKin: with the party allied to
// the veterans, the hero is the veteran's ally; his own kin are not, and the
// stranger in no faction is not. The hero is not in reach though the
// crossbow carries that far, and the route to him ends beside him, not at
// crossbow range. With the party neutral, nobody is.
func TestTheViewNamesAnAllyAcrossADispositionNotKin(t *testing.T) {
	enc := alliesEnc(t, StanceAllied)
	view, err := enc.buildMonsterView(enc.members["veteran"], TurnBudget{AttacksLeft: 1, MovementFeet: 30}, 0)
	require.NoError(t, err)

	hero := seenOf(t, view, "hero")
	require.True(t, hero.Allied, "the party is allied to his side")
	require.False(t, seenOf(t, view, "kin").Allied, "his own faction is kin, not an ally")
	require.False(t, seenOf(t, view, "stranger").Allied, "a member in no faction is nobody's ally")
	require.False(t, hero.InReach[allyCrossbow], "an ally is never in reach")
	require.NotEmpty(t, hero.Path, "nine cells off, inside crossbow range, he still walks")
	require.LessOrEqual(t, enc.Distance(hero.Path[len(hero.Path)-1], hero.Position), 1.0,
		"and the walk ends beside the hero")
	require.True(t, seenOf(t, view, "kin").InReach[allyCrossbow], "a non-ally keeps its reach")

	facts := factsFromView(view)
	require.Equal(t, []core.EntityID{"hero"}, facts.SeenAllies)

	enc = alliesEnc(t, StanceNeutral)
	view, err = enc.buildMonsterView(enc.members["veteran"], TurnBudget{AttacksLeft: 1, MovementFeet: 30}, 0)
	require.NoError(t, err)
	require.False(t, seenOf(t, view, "hero").Allied, "a neutral party is nobody's ally")
	require.Empty(t, factsFromView(view).SeenAllies)
}
