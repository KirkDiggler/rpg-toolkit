// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// pursuit_internal_test.go is the table driver's and the projection's half of
// pursuit (rpg-project#668, memory.go): `toward: enemy` losing a memory it has
// walked to the end of, `search` walking a lost trail, and `enemy: lost` as a
// band of its own.

// TestTowardARememberedEnemyWithNowhereLeftLosesIt is R3 at the driver: a
// remembered enemy whose route is empty — the creature stands on the cell, or
// cannot reach it — goes out as a silent Pass that loses the memory. A route
// still to walk is walked, and an enemy in SIGHT with no route loses nothing.
func (s *TableSuite) TestTowardARememberedEnemyWithNowhereLeftLosesIt() {
	d := TableDriver{}
	row := Answer{Toward: &Selector{Word: SelectorEnemy}}
	budget := TurnBudget{MovementFeet: 30}

	nowhere := MonsterView{Budget: budget, Remembered: []RememberedMember{{
		ID: "bard", Opposed: true, Position: spatialOrigin,
	}}}
	s.Equal(Pass{Loses: "bard"}, d.towardIntent(nowhere, row), "at the remembered cell, the memory is lost")

	walking := nowhere
	walking.Remembered = []RememberedMember{{
		ID: "bard", Opposed: true, Position: spatialOrigin, Path: []spatial.Position{{X: 1, Y: 0}, spatialOrigin},
	}}
	s.Equal(Move{Path: []spatial.Position{{X: 1, Y: 0}, spatialOrigin}, Target: "bard"},
		d.towardIntent(walking, row), "a route still to walk is walked")

	inSight := MonsterView{Budget: budget, Seen: []SeenMember{{
		ID: "bard", Opposed: true, Standing: true, DistanceCells: 4,
	}}}
	s.Equal(Pass{}, d.towardIntent(inSight, row), "a seen enemy with no route is still seen: nothing is lost")
}

// TestASearchWalksToTheTrailsNthCell is R4 at the driver: the nearest opposed
// lost enemy's trail, walked to its Nth cell — or its last when it is shorter,
// or the lost cell itself when it has none — as an exact-cell routed walk that
// names whom it gives up on.
func (s *TableSuite) TestASearchWalksToTheTrailsNthCell() {
	d := TableDriver{}
	trail := []spatial.Position{{X: 1}, {X: 2}, {X: 3}, {X: 4}, {X: 5}, {X: 6}, {X: 7}, {X: 8}}
	lost := LostMember{ID: "bard", Opposed: true, Position: spatial.Position{}, DistanceCells: 2, Trail: trail}
	view := MonsterView{Budget: TurnBudget{MovementFeet: 30}, Lost: []LostMember{lost}}

	sixth := trail[5]
	s.Equal(Routed{Policy: MoveToward, AnchorAt: &sixth, Cause: tableCauseSearch, GivesUp: "bard"},
		d.searchIntent(view, Search{Distance: 6}), "six on is the trail's sixth cell")

	short := view
	short.Lost = []LostMember{{ID: "bard", Opposed: true, Trail: trail[:3]}}
	third := trail[2]
	s.Equal(&third, d.searchIntent(short, Search{Distance: 6}).(Routed).AnchorAt, "a trail cut short ends at its last cell")

	blocked := view
	blocked.Lost = []LostMember{{ID: "bard", Opposed: true, Position: spatialOrigin}}
	s.Equal(spatialOrigin, *d.searchIntent(blocked, Search{Distance: 6}).(Routed).AnchorAt,
		"no trail at all searches the lost cell")

	nearer := view
	nearer.Lost = []LostMember{
		{ID: "far", Opposed: true, DistanceCells: 9, Trail: trail},
		{ID: "friend", Opposed: false, DistanceCells: 1, Trail: trail},
		{ID: "near", Opposed: true, DistanceCells: 3, Trail: trail},
	}
	s.Equal(MemberID("near"), d.searchIntent(nearer, Search{Distance: 6}).(Routed).GivesUp,
		"the nearest opposed one; a lost friend is not searched for")

	s.Equal(Pass{}, d.searchIntent(MonsterView{}, Search{Distance: 6}), "nobody lost is nothing to search for")
}

// TestLostIsABandOfItsOwn is R3 at the projection: a lost opposed sighting
// holds `enemy: lost`, and every nearer band clears it, so the five bands stay
// exclusive. A lost friend holds nothing.
func (s *TableSuite) TestLostIsABandOfItsOwn() {
	lost := factSighting{opposed: true, lost: true}
	only := projectFacts(factsInput{sightings: []factSighting{lost}})
	s.True(only.EnemyLost)
	s.False(only.EnemyRemembered, "lost is not remembered")

	withMemory := projectFacts(factsInput{sightings: []factSighting{lost, {opposed: true}}})
	s.True(withMemory.EnemyRemembered, "a memory still worth walking to is nearer")
	s.False(withMemory.EnemyLost)

	withSight := projectFacts(factsInput{sightings: []factSighting{lost, {opposed: true, current: true, standing: true}}})
	s.True(withSight.EnemySeen)
	s.False(withSight.EnemyLost)

	inReach := projectFacts(factsInput{canAttack: true, sightings: []factSighting{
		lost, {opposed: true, current: true, standing: true, inReach: true},
	}})
	s.True(inReach.EnemyInReach)
	s.False(inReach.EnemyLost)

	friend := projectFacts(factsInput{sightings: []factSighting{{lost: true}}})
	s.False(friend.EnemyLost, "a lost friend is no enemy")
}

// TestTheViewsLostFeedTheLostBand is the driver's door into the band: a view's
// Lost entries are lost sightings, never remembered ones.
func (s *TableSuite) TestTheViewsLostFeedTheLostBand() {
	facts := factsFromView(MonsterView{Lost: []LostMember{{ID: "bard", Opposed: true}}})
	s.True(facts.EnemyLost)
	s.False(facts.EnemyRemembered)
}

// TestASearchIsATimeWordAtTheCompositionsDoor: `search` is what a creature
// does with time, so a verdict's row carrying one is refused at the
// composition's own door, for every caller the dialect does not front.
func (s *TableSuite) TestASearchIsATimeWordAtTheCompositionsDoor() {
	row := Answer{Weight: 1, Search: &Search{Distance: 6}}
	s.NoError(validateEntry(AnswerTime, 0, row))
	s.ErrorIs(validateEntry(AnswerSuccess, 0, row), ErrBadAnswer)
	s.Equal("search", answerWord(row))
}

// TestACloneDoesNotShareASearch: a cloned table's search is its own, the rule
// every pointer on a row keeps.
func (s *TableSuite) TestACloneDoesNotShareASearch() {
	table := Table{AnswerTime: {{Weight: 1, Search: &Search{Distance: 6}}}}
	clone := cloneTable(table)
	table[AnswerTime][0].Search.Distance = 2
	s.Equal(6, clone[AnswerTime][0].Search.Distance)
}

// TestACurrentSightingIsNeverRewritten: a holding sustained on sight right now
// is not a memory, and rewriteSight leaves it before asking what to change —
// it reaches neither the change nor the store.
func (s *TableSuite) TestACurrentSightingIsNeverRewritten() {
	payload, err := EncodeSightTestimony(SightTestimony{State: LocationKnown, Position: spatialOrigin})
	s.Require().NoError(err)
	current := perception.Holding{
		Subject: "bard", Channel: perception.Sight, Payload: payload,
		CurrentVia: []perception.Channel{perception.Sight},
	}
	asked := false
	var e *Encounter
	s.Require().NoError(e.rewriteSight("goblin", "bard", []perception.Holding{current}, func(*SightTestimony) bool {
		asked = true

		return true
	}))
	s.False(asked, "a sighting being made is not revised")
}
