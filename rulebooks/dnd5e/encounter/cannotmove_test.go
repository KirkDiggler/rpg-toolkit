// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// cannotmove_test.go is the Keep's hold in the composition (rpg-project#990):
// a member whose sheet says it cannot move under its own power
// ([encounter.SheetFacts.CannotMove]) does not step, does not carry itself off
// with its own cast, and — driven — does not walk, willing or compelled; everything
// done TO it still moves it, and attacking, being attacked, sneaking and a
// fight forming are untouched. A fight starting is told for each member in it,
// once, and for nobody outside it.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type CannotMoveSuite struct {
	suite.Suite
}

func TestCannotMoveSuite(t *testing.T) { suite.Run(t, new(CannotMoveSuite)) }

// held is the sheet answer of a member a status named Held holds.
var held = []string{"Held"}

var cmCause = core.Ref{Module: "dnd5e", Type: "consequences", ID: "test-shove"}

// world opens one room (and a far one nobody in it can see) with the given members, every one standing and seen,
// and the given sheets. Two players never fight; a monster beside a player
// forms a fight at once, in the order the members are given.
func (s *CannotMoveSuite) world(
	members []encounter.MemberInput, sheets sheetFacts, driver encounter.Driver, striker encounter.Striker,
	announcer encounter.Announcer,
) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 12, 12), rectRegion("room-far", 30, 0, 5, 5)},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			CheckResolver: findsNothing{},
			Driver:        driver,
			Actors: encounter.Actors{
				Striker:   striker,
				Mover:     quietMover{},
				Announcer: announcer,
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

// twoPlayers is alice and bob on the world clock; alice is held when heldAlice.
func (s *CannotMoveSuite) twoPlayers(heldAlice bool) *encounter.Encounter {
	sheets := sheetFacts{alice: {SpeedFeet: 30}, bob: {SpeedFeet: 30}}
	if heldAlice {
		sheets[alice] = encounter.SheetFacts{SpeedFeet: 30, CannotMove: held}
	}
	return s.world([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 6}},
	}, sheets, passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietAnnouncer{})
}

// fight is alice beside a goblin, a fight formed at once with alice first.
func (s *CannotMoveSuite) fight(
	aliceSheet, goblinSheet encounter.SheetFacts, driver encounter.Driver, striker encounter.Striker,
	announcer encounter.Announcer,
) *encounter.Encounter {
	goblinSheet.SpeedFeet = 30
	goblinSheet.Actions = []encounter.ActionView{{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"}}
	goblinSheet.Targeting = "closest"
	return s.world([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
	}, sheetFacts{alice: aliceSheet, goblin: goblinSheet}, driver, striker, announcer)
}

func (s *CannotMoveSuite) clockOf(enc *encounter.Encounter, who encounter.MemberID) *encounter.ClockOfOutput {
	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: who})
	s.Require().NoError(err)
	return out
}

// Catches a Step that does not ask the hold, asks it for the wrong member,
// or half-does the step before refusing.
func (s *CannotMoveSuite) TestAHeldPlayerCannotStepOnTheWorldClock() {
	enc := s.twoPlayers(true)
	before := whereIs(s.T(), enc, alice)
	story := len(storyBeats(s.T(), enc, alice))

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 3), EndWalk: true})
	s.Require().ErrorIs(err, encounter.ErrCannotMove)
	var refusal *encounter.CannotMoveError
	s.Require().True(errors.As(err, &refusal))
	s.Equal(encounter.MemberID(alice), refusal.Member)
	s.Equal(held, refusal.HeldBy)
	s.Contains(err.Error(), "You are Held; you cannot move.")
	s.Equal(before, whereIs(s.T(), enc, alice), "nothing moved")
	s.Len(storyBeats(s.T(), enc, alice), story, "nothing was told")

	_, err = enc.Step(&encounter.StepInput{Member: bob, To: cellAt(6, 7), EndWalk: true})
	s.Require().NoError(err, "a member nothing holds walks")
}

// Catches a hold that a fight's own turn skips.
func (s *CannotMoveSuite) TestAHeldPlayerCannotStepOnTheirOwnTurn() {
	enc := s.fight(encounter.SheetFacts{SpeedFeet: 30, CannotMove: []string{"Held", "Snared"}}, encounter.SheetFacts{},
		passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietAnnouncer{})
	s.Require().Equal(encounter.MemberID(alice), s.clockOf(enc, alice).Active, "precondition: alice's turn")

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2), EndWalk: true})
	s.Require().ErrorIs(err, encounter.ErrCannotMove)
	s.Contains(err.Error(), "You are Held and Snared; you cannot move.")
	s.Equal(cellAt(2, 2), whereIs(s.T(), enc, alice))
}

// Catches a hold copied onto the moves custody refuses: [encounter.Direct],
// whatever kind its caller names, and a relocation are done TO the held
// member, and move them.
func (s *CannotMoveSuite) TestMovesMadeOnTheHeldStillMoveThem() {
	enc := s.twoPlayers(true)
	for _, kind := range []encounter.MoveKind{encounter.MoveDisplaced, encounter.MoveCompelled} {
		from := whereIs(s.T(), enc, alice)
		to := spatial.Position{X: from.X + 1, Y: from.Y}
		out, err := enc.Direct(context.Background(), encounter.DirectInput{
			Mover: alice, Cause: cmCause, Route: []spatial.Position{to}, Kind: kind,
		})
		s.Require().NoError(err, kind)
		s.Equal(1, out.Moved, kind)
		s.Equal(to, whereIs(s.T(), enc, alice), kind)
	}

	_, err := enc.Relocate(&encounter.RelocateInput{
		Moves: []encounter.Relocation{{Member: alice, To: cellAt(8, 2)}}, Cause: relCause,
	})
	s.Require().NoError(err)
	s.Equal(cellAt(8, 2), whereIs(s.T(), enc, alice), "set down somewhere else")
}

// Catches a cast whose landing carries a held caster off, and a hold that
// refuses a cast that does not move the caster.
func (s *CannotMoveSuite) TestAHeldCasterIsNotCarriedOffByTheirOwnCast() {
	enc := s.twoPlayers(true)
	self := []encounter.CastPush{{Mover: alice, Route: []spatial.Position{cellAt(3, 2)}}}
	_, err := enc.RecordCast(&encounter.RecordCastInput{Actor: alice, Spell: clThunderwave, Pushes: self})
	s.Require().ErrorIs(err, encounter.ErrCannotMove)
	s.Equal(cellAt(2, 2), whereIs(s.T(), enc, alice))

	// The same malformed cast by a caster nothing holds is refused for being
	// malformed, never for a hold.
	_, err = enc.RecordCast(&encounter.RecordCastInput{Actor: bob, Spell: clThunderwave,
		Pushes: []encounter.CastPush{{Mover: bob, Route: []spatial.Position{cellAt(6, 7)}}}})
	s.NotErrorIs(err, encounter.ErrCannotMove)
	// Nor is a held member's cast that moves somebody else.
	_, err = enc.RecordCast(&encounter.RecordCastInput{Actor: alice, Spell: clThunderwave,
		Pushes: []encounter.CastPush{{Mover: bob, Route: []spatial.Position{cellAt(6, 7)}}}})
	s.NotErrorIs(err, encounter.ErrCannotMove)
}

// Catches a hold that refuses sneaking, which moves nobody.
func (s *CannotMoveSuite) TestAHeldMemberStillSneaks() {
	enc := s.twoPlayers(true)
	out, err := enc.SetSneaking(&encounter.SetSneakingInput{Member: alice, Sneaking: true})
	s.Require().NoError(err)
	s.True(out.Sneaking)
}

// Catches a hold that stops a fight forming, a held player attacking, or a
// held monster attacking.
func (s *CannotMoveSuite) TestAHeldMemberStillFightsAndIsFought() {
	j := &journal{}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Attack{Target: alice, Action: testMeleeAction}}}
	enc := s.fight(encounter.SheetFacts{SpeedFeet: 30, CannotMove: held}, encounter.SheetFacts{CannotMove: held},
		driver, journalStriker{j: j, inner: &scriptedStriker{kind: encounter.OutcomeMissed}}, quietAnnouncer{})
	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice).Kind, "the fight formed with both held")

	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	s.Require().NoError(err, "the held player swings")

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Contains(j.entries, "strike:goblin", "the held goblin swings at the held player")
}

// Catches a driven member's own walk taken while held, a refusal that errors
// the round, or one that charges the walk: the goblin stays, the story says
// why, and the fight comes back round to alice.
func (s *CannotMoveSuite) TestAHeldMonstersOwnWalkDoesNotHappenAndTheRoundGoesOn() {
	for name, intent := range map[string]encounter.TurnIntent{
		"move":  encounter.Move{Path: []spatial.Position{cellAt(4, 2)}},
		"run":   encounter.Move{Path: []spatial.Position{cellAt(4, 2)}, Dash: true},
		"route": encounter.Routed{Policy: encounter.MoveToward, AnchorAt: cmCell(cellAt(6, 2)), Cause: cmCause},
	} {
		s.Run(name, func() {
			driver := &scriptedDriver{intents: []encounter.TurnIntent{intent}}
			enc := s.fight(encounter.SheetFacts{SpeedFeet: 30}, encounter.SheetFacts{CannotMove: held},
				driver, &scriptedStriker{kind: encounter.OutcomeMissed}, quietAnnouncer{})

			_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
			s.Require().NoError(err, "a held walk does not error the round")
			s.Equal(cellAt(3, 2), whereIs(s.T(), enc, goblin), "the goblin stays")
			s.Equal(encounter.MemberID(alice), s.clockOf(enc, alice).Active, "the round goes on")
			s.Len(driver.calls, 1, "the turn ended at the refused walk")
			s.Equal(held, driver.calls[0].CannotMove, "the driver was told what holds it")

			stayed := stayedBeats(goblinBeats(s.T(), enc))
			s.Require().NotEmpty(stayed)
			s.Equal("cannot move: Held", stayed[len(stayed)-1]["why"])
		})
	}
}

// Catches a compelled walk let through a hold: ordered to flee, a held
// creature still moves under its own power, so it stays, the story says why,
// and the round goes on.
func (s *CannotMoveSuite) TestAHeldMonsterCompelledToWalkStaysPut() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Routed{
		Policy: encounter.MoveToward, AnchorAt: cmCell(cellAt(6, 2)), Cause: cmCause, Kind: encounter.MoveCompelled,
	}}}
	enc := s.fight(encounter.SheetFacts{SpeedFeet: 30}, encounter.SheetFacts{CannotMove: held},
		driver, &scriptedStriker{kind: encounter.OutcomeMissed}, quietAnnouncer{})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err, "a held walk does not error the round")
	s.Equal(cellAt(3, 2), whereIs(s.T(), enc, goblin), "ordered to walk, it still stays")
	s.Equal(encounter.MemberID(alice), s.clockOf(enc, alice).Active, "the round goes on")
	stayed := stayedBeats(goblinBeats(s.T(), enc))
	s.Require().NotEmpty(stayed)
	s.Equal("cannot move: Held", stayed[len(stayed)-1]["why"])
}

// combatStarts is the subjects of every combat-started boundary in j.
func combatStarts(j *journal) []string {
	var out []string
	for _, e := range j.entries {
		if rest, ok := strings.CutPrefix(e, string(encounter.CombatStarted)+":"); ok {
			out = append(out, rest)
		}
	}
	return out
}

// Catches a start told to nobody, told twice, told to a member outside the
// fight, or told after the fight's first turn has started.
func (s *CannotMoveSuite) TestAFightStartingIsToldOnceToEachMemberInIt() {
	j := &journal{}
	enc := s.world([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
	}, sheetFacts{alice: {SpeedFeet: 30}, goblin: {SpeedFeet: 30, Actions: []encounter.ActionView{
		{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
	}}}, passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, journalAnnouncer{j: j})

	s.ElementsMatch([]string{"alice:r1", "goblin:r1"}, combatStarts(j), "once for each member: %v", j.entries)
	s.Less(j.indexOf("combat_started:alice:r1"), j.indexOf("turn_started:alice:r1"),
		"the start is told before the first turn: %v", j.entries)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.ElementsMatch([]string{"alice:r1", "goblin:r1"}, combatStarts(j), "turns passing start nothing: %v", j.entries)
}

// Catches a join that is not told, or one told to everybody already in the
// fight.
func (s *CannotMoveSuite) TestJoiningAFightUnderWayIsToldOnlyToTheOneWhoJoins() {
	j := &journal{}
	enc := s.world([]encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
		{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 32, Y: 2}},
	}, sheetFacts{alice: {SpeedFeet: 30}, bob: {SpeedFeet: 30}, goblin: {SpeedFeet: 30, Actions: []encounter.ActionView{
		{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
	}}}, passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, journalAnnouncer{j: j})
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, bob).Kind, "precondition: bob is not in the fight")
	before := combatStarts(j)
	s.NotContains(before, "bob:r1", "bob is not in the fight")

	_, err := enc.Transfer(&encounter.TransferInput{Member: bob, To: encounter.ClockTurn})
	s.Require().NoError(err)
	s.ElementsMatch(append(before, "bob:r1"), combatStarts(j), "bob's start, and nobody else's again: %v", j.entries)
}

// cmCell is p, addressable, for an intent's anchor.
func cmCell(p spatial.Position) *spatial.Position { return &p }

// Catches a rename of the fight-started kind: resolution looks it up by this
// very string, and pins the same literal on its side, so a rename on either
// side fails that side's test instead of an unknown kind refusing every fight.
func (s *CannotMoveSuite) TestTheFightStartedSpellingIsPinned() {
	s.Equal(encounter.BoundaryKind("combat_started"), encounter.CombatStarted)
}
