// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// SessionVerbsSuite is the composition's half of the session verbs
// (rpg-project#542, slice 2): an equip told to whoever sees the actor and
// refused off their turn in a fight, and a short rest that is an hour on the
// world clock and nothing else.
type SessionVerbsSuite struct {
	suite.Suite
}

func TestSessionVerbsSuite(t *testing.T) {
	suite.Run(t, new(SessionVerbsSuite))
}

// scene is one yard split by a wall row at y=6: whoever stands above it
// sees each other, billy below it sees none of them. A den beyond the void
// holds whatever the scene hides there. Every member walks 30
// feet, so a step accrues pace.
func (s *SessionVerbsSuite) scene(members ...encounter.MemberInput) *encounter.Encounter {
	sheets := sheetFacts{}
	for _, m := range members {
		sheets[m.ID] = encounter.SheetFacts{SpeedFeet: 30}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: encounter.UnobservedEquipment{}, Sheets: sheets, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: tableDriver(), Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			// The den is cut off from the yard by void, which is opaque: a
			// creature in it sees nobody and nobody sees it.
			Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12), rectRegion("den", 20, 0, 4, 3)},
			Props:   wallRow(6, 4, 8),
		},
		Members:   members,
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	return enc
}

// freeRoam is alice and bob in sight of each other above the wall and billy
// below it: three players, so no fight.
func (s *SessionVerbsSuite) freeRoam() *encounter.Encounter {
	return s.scene(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 3}},
		encounter.MemberInput{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
	)
}

// fight is alice and bob in a fight with the goblin above the wall, and
// billy below it, free.
func (s *SessionVerbsSuite) fight() (enc *encounter.Encounter, active, waiting encounter.MemberID) {
	enc = s.scene(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 3}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}},
		encounter.MemberInput{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
	)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, clock.Kind, "a player and a monster sharing sight is a fight")
	billyClock, err := enc.ClockOf(&encounter.ClockOfInput{Member: billy})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, billyClock.Kind, "billy, behind the wall, is not in it")

	// WHO is active is trigger detection's order, not this scene's to
	// assume; the scene asks, then names the other player as waiting.
	switch clock.Active {
	case alice:
		return enc, alice, bob
	case bob:
		return enc, bob, alice
	default:
		// The goblin is up first: pass its turn so a player is active.
		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: clock.Active})
		s.Require().NoError(err)
		now, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
		s.Require().NoError(err)
		if now.Active == alice {
			return enc, alice, bob
		}
		s.Require().Equal(bob, now.Active)
		return enc, bob, alice
	}
}

// beatsOfKind is every beat of one kind in one member's story.
func (s *SessionVerbsSuite) beatsOfKind(enc *encounter.Encounter, member core.EntityID, kind string) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}
	return out
}

func (s *SessionVerbsSuite) highWater(enc *encounter.Encounter) int {
	return enc.ToData().Clock.HighWater
}

// equipEntries is every equipment-changed entry in one member's story, with
// its correlation.
func (s *SessionVerbsSuite) equipEntries(enc *encounter.Encounter, member core.EntityID) []record.Entry {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]record.Entry, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatEquipmentChanged {
			out = append(out, entry)
		}
	}
	return out
}

// An equip beat reaches every member that perceives the actor and no member
// that does not: alice and bob see each other, billy is behind the wall.
func (s *SessionVerbsSuite) TestAnEquipIsToldToWhoeverSeesTheActor() {
	enc := s.freeRoam()

	out, err := enc.RecordEquip(&encounter.RecordEquipInput{
		Member: alice, Slot: "main_hand", Drawn: "dnd5e:weapons:longsword",
	})
	s.Require().NoError(err)
	s.ElementsMatch([]encounter.MemberID{alice, bob}, out.Audience)

	for _, watcher := range []encounter.MemberID{alice, bob} {
		beats := s.beatsOfKind(enc, watcher, encounter.BeatEquipmentChanged)
		s.Require().Len(beats, 1, "%s sees alice draw, once", watcher)
		s.Equal(string(alice), beats[0]["member"])
		s.Equal("main_hand", beats[0]["slot"])
		s.Equal("dnd5e:weapons:longsword", beats[0]["item"])
		s.Equal(string(encounter.EquipDraw), beats[0]["change"])
	}
	s.Empty(s.beatsOfKind(enc, billy, encounter.BeatEquipmentChanged), "billy cannot see alice and is not told")
}

// The equip's re-look DECLARES the actor changed, so a watcher who already
// sees them is told to look again — a `sighted` beat naming the actor as
// changed — with no second Recheck call. A member who cannot see the actor
// is told nothing.
func (s *SessionVerbsSuite) TestAnEquipTellsWatchersToLookAgain() {
	enc := s.freeRoam()
	changedSeen := func(member encounter.MemberID) int {
		n := 0
		for _, beat := range s.beatsOfKind(enc, member, encounter.BeatSighted) {
			changed, _ := beat["changed"].([]any)
			for _, who := range changed {
				if who == string(alice) {
					n++
				}
			}
		}
		return n
	}
	s.Require().Zero(changedSeen(bob), "precondition: nothing has changed about alice yet")

	_, err := enc.RecordEquip(&encounter.RecordEquipInput{
		Member: alice, Slot: "main_hand", Drawn: "dnd5e:weapons:longsword",
	})
	s.Require().NoError(err)

	s.Equal(1, changedSeen(bob), "bob sees alice and is told her appearance changed")
	s.Zero(changedSeen(billy), "billy cannot see alice")
}

// A swap is two beats — the stow, then the draw — on one correlation, and an
// equip outside a fight costs nothing on the world clock (R1: the economy is
// a fight's).
func (s *SessionVerbsSuite) TestASwapIsAStowThenADrawOnOneCorrelation() {
	enc := s.freeRoam()
	before := s.highWater(enc)

	out, err := enc.RecordEquip(&encounter.RecordEquipInput{
		Member: bob, Slot: "main_hand", Stowed: "dnd5e:weapons:longsword", Drawn: "dnd5e:weapons:warhammer",
	})
	s.Require().NoError(err)
	s.NotEmpty(out.Correlation)

	entries := s.equipEntries(enc, alice)
	s.Require().Len(entries, 2, "a stow and a draw")
	s.Equal(out.Seqs, []uint64{entries[0].Seq, entries[1].Seq})
	beats := s.beatsOfKind(enc, alice, encounter.BeatEquipmentChanged)
	s.Equal(string(encounter.EquipStow), beats[0]["change"], "the stow is told first")
	s.Equal("dnd5e:weapons:longsword", beats[0]["item"])
	s.Equal(string(encounter.EquipDraw), beats[1]["change"])
	s.Equal("dnd5e:weapons:warhammer", beats[1]["item"])
	for _, entry := range entries {
		s.Equal(out.Correlation, entry.Correlation, "both halves are one act")
	}
	s.Equal(before, s.highWater(enc), "a free-roam equip is free, in time as well")

	// A second equip is a second act, on a correlation of its own.
	again, err := enc.RecordEquip(&encounter.RecordEquipInput{
		Member: bob, Slot: "main_hand", Stowed: "dnd5e:weapons:warhammer",
	})
	s.Require().NoError(err)
	s.NotEqual(out.Correlation, again.Correlation)
	s.Require().Len(again.Seqs, 1, "a stow alone is one beat")
}

// Recording an equip for a member in a fight on another member's turn
// refuses with the not-your-turn sentinel and writes no beat; the member
// whose turn it is may.
func (s *SessionVerbsSuite) TestAnEquipOffTurnInAFightIsRefused() {
	enc, active, waiting := s.fight()

	_, err := enc.RecordEquip(&encounter.RecordEquipInput{Member: waiting, Slot: "main_hand", Drawn: "dnd5e:weapons:longsword"})
	s.Require().ErrorIs(err, encounter.ErrNotActive)
	s.Contains(err.Error(), "record equip:", "the refusal names the verb")
	for _, member := range []encounter.MemberID{alice, bob, goblin, billy} {
		s.Empty(s.beatsOfKind(enc, member, encounter.BeatEquipmentChanged), "nothing written for %s", member)
	}

	_, err = enc.RecordEquip(&encounter.RecordEquipInput{Member: active, Slot: "main_hand", Drawn: "dnd5e:weapons:longsword"})
	s.Require().NoError(err, "on their own turn")
	s.Len(s.beatsOfKind(enc, active, encounter.BeatEquipmentChanged), 1)
}

// A change the verb cannot record is refused before anything is written.
func (s *SessionVerbsSuite) TestAnEquipThatSaysNothingIsRefused() {
	enc := s.freeRoam()

	cases := map[string]struct {
		in   *encounter.RecordEquipInput
		want error
	}{
		"nil":          {nil, encounter.ErrNilInput},
		"no member":    {&encounter.RecordEquipInput{Slot: "main_hand", Drawn: "dnd5e:weapons:dagger"}, encounter.ErrNoMember},
		"not a member": {&encounter.RecordEquipInput{Member: "nobody", Slot: "main_hand", Drawn: "dnd5e:weapons:dagger"}, encounter.ErrNotMember},
		"no slot":      {&encounter.RecordEquipInput{Member: alice, Drawn: "dnd5e:weapons:dagger"}, encounter.ErrInvalidData},
		"no item":      {&encounter.RecordEquipInput{Member: alice, Slot: "main_hand"}, encounter.ErrInvalidData},
		"the same item both ways": {&encounter.RecordEquipInput{
			Member: alice, Slot: "main_hand", Stowed: "dnd5e:weapons:dagger", Drawn: "dnd5e:weapons:dagger",
		}, encounter.ErrInvalidData},
	}
	for name, tc := range cases {
		_, err := enc.RecordEquip(tc.in)
		s.ErrorIs(err, tc.want, name)
	}
	s.Empty(s.beatsOfKind(enc, alice, encounter.BeatEquipmentChanged))
}

// Recording a rest in free roam writes one beat naming the member and the
// kind, told to whoever sees the rester.
func (s *SessionVerbsSuite) TestARestInFreeRoamTellsOneBeat() {
	enc := s.freeRoam()

	out, err := enc.RecordRest(&encounter.RecordRestInput{
		Member: alice, Kind: encounter.RestShort,
		ResourcesRefilled: []string{"dnd5e:features:second_wind"},
	})
	s.Require().NoError(err)
	s.ElementsMatch([]encounter.MemberID{alice, bob}, out.Audience)

	beats := s.beatsOfKind(enc, bob, encounter.BeatRested)
	s.Require().Len(beats, 1)
	s.Equal(string(alice), beats[0]["member"])
	s.Equal(string(encounter.RestShort), beats[0]["kind"])
	s.Equal([]any{"dnd5e:features:second_wind"}, beats[0]["resources_refilled"])
	s.Empty(s.beatsOfKind(enc, billy, encounter.BeatRested), "billy cannot see alice")
}

// Recording a rest for a member in a fight refuses, and writes nothing: no
// beat and no time.
func (s *SessionVerbsSuite) TestARestInAFightIsRefused() {
	enc, active, waiting := s.fight()
	before := s.highWater(enc)

	for _, member := range []encounter.MemberID{active, waiting} {
		_, err := enc.RecordRest(&encounter.RecordRestInput{Member: member, Kind: encounter.RestShort})
		s.Require().ErrorIs(err, encounter.ErrInBubble, "%s is in the fight", member)
	}
	for _, member := range []encounter.MemberID{alice, bob, goblin, billy} {
		s.Empty(s.beatsOfKind(enc, member, encounter.BeatRested))
	}
	s.Equal(before, s.highWater(enc))
}

// denLurker is a creature with orders, alone in the den on the world clock: if
// the hour were driven rather than jumped it would spend six hundred rounds
// walking to the far end of the den.
const denLurker encounter.MemberID = "denLurker"

// A short rest moves the world clock by one hour and nothing else in the
// run moves: billy rests below the wall while the fight above it waits on
// its turn, with a pace remainder on billy, a sight area standing and a
// creature with orders in the den. Exactly one beat is written, nobody acts,
// and nothing is owed to the next step.
func (s *SessionVerbsSuite) TestAShortRestIsAnHourAndNothingElse() {
	enc := s.scene(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 3}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}},
		encounter.MemberInput{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
		encounter.MemberInput{ID: denLurker, Kind: encounter.KindMonster, Position: spatial.Position{X: 20, Y: 1}, Table: walksTo(cellAt(23, 1))},
	)
	for member, want := range map[encounter.MemberID]encounter.ClockKind{
		alice: encounter.ClockTurn, billy: encounter.ClockWorld, denLurker: encounter.ClockWorld,
	} {
		clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: member})
		s.Require().NoError(err)
		s.Require().Equal(want, clock.Kind, "%s's clock", member)
	}

	// A step accrues one cell of pace on billy, which the rest must keep.
	_, err := enc.Step(&encounter.StepInput{Member: billy, To: cellAt(6, 11)})
	s.Require().NoError(err)
	s.Require().NoError(enc.AddSightArea(&encounter.SightAreaInput{
		ID: "fog", SourceID: "caster", Center: cellAt(1, 10), RadiusFeet: 5,
	}))
	before := enc.ToData()
	s.Require().Equal(1, paceOf(before, billy), "the step accrued pace")
	firstSeq, err := enc.NextStorySeq()
	s.Require().NoError(err)

	out, err := enc.RecordRest(&encounter.RecordRestInput{Member: billy, Kind: encounter.RestShort})
	s.Require().NoError(err)

	after := enc.ToData()
	s.Equal(before.Clock.HighWater+encounter.RoundsPerHour, after.Clock.HighWater, "one hour on the world clock")
	s.Equal(uint64(after.Clock.HighWater), out.Clock)
	s.Equal(600, encounter.RoundsPerHour, "an hour is six hundred six-second rounds")
	nextSeq, err := enc.NextStorySeq()
	s.Require().NoError(err)
	s.Equal(firstSeq+1, nextSeq, "the rest beat and nothing else: no tick, no creature's pick")
	s.Equal(out.Seq, firstSeq)
	s.Nil(out.Formed, "nobody moved, so nothing formed")

	s.Equal(before.Members, after.Members, "nobody moved — not the denLurker, and billy's pace remainder is as it was")
	s.Equal(before.Clock.Budgets, after.Clock.Budgets, "the jump grants nothing, so nothing is owed")
	s.Equal(before.Bubbles, after.Bubbles, "the fight's turn did not move")
	s.Equal(before.SightAreas, after.SightAreas, "the sight area stands")
	s.Equal(before.Doors, after.Doors)

	// AND THE NEXT WALK PACES NORMALLY: five more cells finish billy's
	// thirty-foot pace (six cells) and pay exactly one round, as they would
	// have with no rest between.
	cells := []spatial.Position{cellAt(6, 10), cellAt(6, 11), cellAt(6, 10), cellAt(6, 11)}
	for _, to := range cells {
		_, err := enc.Step(&encounter.StepInput{Member: billy, To: to})
		s.Require().NoError(err)
	}
	s.Equal(after.Clock.HighWater, s.highWater(enc), "five cells of pace pay nothing yet")
	_, err = enc.Step(&encounter.StepInput{Member: billy, To: cellAt(6, 10)})
	s.Require().NoError(err)
	s.Equal(after.Clock.HighWater+1, s.highWater(enc), "the sixth cell pays the round")
}

// The hour is a jump, so each rest is its own hour: two members resting one
// after the other is two hours on the clock.
func (s *SessionVerbsSuite) TestEachRestIsItsOwnHour() {
	enc := s.freeRoam()
	before := s.highWater(enc)

	for _, member := range []encounter.MemberID{alice, bob} {
		_, err := enc.RecordRest(&encounter.RecordRestInput{Member: member, Kind: encounter.RestShort})
		s.Require().NoError(err)
	}

	s.Equal(before+2*encounter.RoundsPerHour, s.highWater(enc))
	s.Len(s.beatsOfKind(enc, alice, encounter.BeatRested), 2, "each rest is told")
}

// What a rest restored is carried as told, and refused when it cannot have
// happened as told.
func (s *SessionVerbsSuite) TestARestCarriesWhatItRestored() {
	enc := s.freeRoam()
	modifier := 4
	calc := &encounter.RollCalculation{
		Components: []encounter.RollComponent{
			{
				Source: encounter.RollSource{Ref: "dnd5e:classes:fighter", Name: "Hit Dice", SourceID: string(alice)},
				Dice: &encounter.DiceTrace{
					Notation: "2d10", DieSize: 10, OriginalRolls: []int{3, 7}, FinalRolls: []int{3, 7}, Subtotal: 10,
				},
			},
			{Source: encounter.RollSource{Ref: "dnd5e:abilities:con", Name: "Constitution"}, Modifier: &modifier},
		},
		Total: 14,
	}

	_, err := enc.RecordRest(&encounter.RecordRestInput{
		Member: alice, Kind: encounter.RestShort, HitDiceSpent: 2, HitPointsRestored: 12, Calculation: calc,
		HitPoints: 20, HitDiceRemaining: 1,
	})
	s.Require().NoError(err)
	beats := s.beatsOfKind(enc, alice, encounter.BeatRested)
	s.Require().Len(beats, 1)
	s.Equal(float64(2), beats[0]["hit_dice_spent"])
	s.Equal(float64(12), beats[0]["hit_points_restored"], "capped by the rulebook, carried as told")
	s.Equal(float64(20), beats[0]["hit_points"])
	s.Equal(float64(0), beats[0]["hit_dice_returned"], "a short rest returns none, and says so")
	s.Equal(float64(1), beats[0]["hit_dice_remaining"])
	s.NotNil(beats[0]["calculation"])

	refused := map[string]*encounter.RecordRestInput{
		"no kind":                 {Member: alice},
		"a long rest":             {Member: alice, Kind: "long"},
		"negative dice":           {Member: alice, Kind: encounter.RestShort, HitDiceSpent: -1},
		"negative hit points":     {Member: alice, Kind: encounter.RestShort, HitPointsRestored: -1},
		"dice with no arithmetic": {Member: alice, Kind: encounter.RestShort, HitDiceSpent: 1},
		"arithmetic with no dice": {Member: alice, Kind: encounter.RestShort, Calculation: calc},
		"an unnamed refill":       {Member: alice, Kind: encounter.RestShort, ResourcesRefilled: []string{""}},
		"negative hit points now": {Member: alice, Kind: encounter.RestShort, HitPoints: -1},
		"negative dice returned":  {Member: alice, Kind: encounter.RestShort, HitDiceReturned: -1},
		"negative dice remaining": {Member: alice, Kind: encounter.RestShort, HitDiceRemaining: -1},
	}
	for name, in := range refused {
		_, err := enc.RecordRest(in)
		s.ErrorIs(err, encounter.ErrInvalidData, name)
	}
	s.Len(s.beatsOfKind(enc, alice, encounter.BeatRested), 1, "no refusal wrote a beat")
}

func paceOf(data encounter.EncounterData, member encounter.MemberID) int {
	for _, m := range data.Members {
		if m.ID == member {
			return m.Pace
		}
	}
	return 0
}
