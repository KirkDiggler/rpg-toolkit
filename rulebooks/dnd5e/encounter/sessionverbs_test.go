// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
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
	// Every member this suite places or joins walks 30 feet.
	sheets := sheetFacts{}
	for _, id := range []encounter.MemberID{alice, bob, billy, goblin, denLurker} {
		sheets[id] = encounter.SheetFacts{SpeedFeet: 30}
	}
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

	out, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{
		{Member: alice, ResourcesRefilled: []string{"dnd5e:features:second_wind"}},
	}})
	s.Require().NoError(err)
	s.Require().Len(out.Rested, 1)
	s.Equal(alice, out.Rested[0].Member)
	s.ElementsMatch([]encounter.MemberID{alice, bob}, out.Rested[0].Audience)

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
		_, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{{Member: member}}})
		s.Require().ErrorIs(err, encounter.ErrInBubble, "%s is in the fight", member)
	}
	// ONE MEMBER IN A FIGHT REFUSES THE WHOLE REST: billy is free, and his
	// beat is not written either.
	_, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{
		{Member: billy}, {Member: active},
	}})
	s.Require().ErrorIs(err, encounter.ErrInBubble)
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

	out, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{{Member: billy}}})
	s.Require().NoError(err)

	after := enc.ToData()
	s.Equal(before.Clock.HighWater+encounter.RoundsPerHour, after.Clock.HighWater, "one hour on the world clock")
	s.Equal(uint64(after.Clock.HighWater), out.Clock)
	s.Equal(600, encounter.RoundsPerHour, "an hour is six hundred six-second rounds")
	nextSeq, err := enc.NextStorySeq()
	s.Require().NoError(err)
	s.Equal(firstSeq+1, nextSeq, "the rest beat and nothing else: no tick, no creature's pick")
	s.Require().Len(out.Rested, 1)
	s.Equal(firstSeq, out.Rested[0].Seq)
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

// A rest is the party's act: two members resting in ONE call is one hour,
// each told their own beat; two calls are two hours.
func (s *SessionVerbsSuite) TestMembersRestingTogetherShareOneHour() {
	enc := s.freeRoam()
	before := s.highWater(enc)

	out, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{
		{Member: alice}, {Member: billy},
	}})
	s.Require().NoError(err)
	s.Equal(before+encounter.RoundsPerHour, s.highWater(enc), "one rest, one hour")
	s.Require().Len(out.Rested, 2)
	s.Equal(alice, out.Rested[0].Member)
	s.Equal(billy, out.Rested[1].Member)
	s.ElementsMatch([]encounter.MemberID{alice, bob}, out.Rested[0].Audience, "alice's beat to alice's witnesses")
	s.ElementsMatch([]encounter.MemberID{billy}, out.Rested[1].Audience, "billy's beat to his own, behind the wall")
	s.Len(s.beatsOfKind(enc, bob, encounter.BeatRested), 1, "bob sees alice rest, not billy")
	s.Len(s.beatsOfKind(enc, billy, encounter.BeatRested), 1, "billy is told his own rest")

	_, err = enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{{Member: bob}}})
	s.Require().NoError(err)
	s.Equal(before+2*encounter.RoundsPerHour, s.highWater(enc), "a second rest is a second hour")
}

// A member who joins after a rest arrives at the world's own time, not an
// hour behind it: their walking paces the clock from their first step, so
// the sixth cell of bob's first walk raises the clock past the rest's hour.
func (s *SessionVerbsSuite) TestAMemberWhoJoinsAfterARestWalksOnTheWorldsTime() {
	enc := s.scene(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
	)
	_, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{{Member: alice}}})
	s.Require().NoError(err)
	rested := s.highWater(enc)
	s.Require().Equal(encounter.RoundsPerHour, rested)

	_, err = enc.Join(&encounter.JoinInput{Member: bob, Kind: encounter.KindPlayer, Cell: cellAt(6, 4)})
	s.Require().NoError(err)
	s.Equal(rested, s.highWater(enc), "joining moves no time")

	walk := []spatial.Position{cellAt(6, 3), cellAt(6, 4), cellAt(6, 3), cellAt(6, 4), cellAt(6, 3)}
	for _, to := range walk {
		_, err := enc.Step(&encounter.StepInput{Member: bob, To: to})
		s.Require().NoError(err)
	}
	s.Equal(rested, s.highWater(enc), "five cells of bob's pace pay nothing yet")
	_, err = enc.Step(&encounter.StepInput{Member: bob, To: cellAt(6, 4)})
	s.Require().NoError(err)
	s.Equal(rested+1, s.highWater(enc), "the sixth cell pays a round, an hour after nobody")
}

// A creature that arrives from reserve after a rest arrives at the world's
// own time too: the reserve door seats it at the clock's reading, not an
// hour behind it.
func (s *SessionVerbsSuite) TestAReserveArrivalAfterARestSeatsAtTheWorldsTime() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: encounter.UnobservedEquipment{}, Sheets: sheetFacts{goblin: {SpeedFeet: 30}, "straggler": {SpeedFeet: 30}, alice: {SpeedFeet: 30}},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: tableDriver(), Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Roller: rollsLowest{},
		Field: encounter.FieldInput{
			Canvas:   openAir(),
			Regions:  []encounter.RegionInput{rectRegion("front", 0, 0, 12, 6)},
			Factions: []encounter.FactionInput{{ID: campFaction, Mind: core.EntityID(goblin)}},
			// NEUTRAL, so nobody fights and alice may rest.
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{campFaction, encounter.FactionParty}, Stance: encounter.StanceNeutral,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Faction: campFaction,
				Table: encounter.Table{encounter.AnswerIntimidated: {{Weight: 1, Say: "Fine.", Fact: campFact}}}},
			{ID: "straggler", Kind: encounter.KindMonster, Position: spatial.Position{X: 9, Y: 3}, Faction: campFaction,
				Arrives: encounter.TriggerFact{Fact: campFact}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{{Member: alice}}})
	s.Require().NoError(err)
	rested := s.highWater(enc)
	s.Require().Equal(encounter.RoundsPerHour, rested)

	// The threat plants the fact the straggler waits on, and it arrives.
	_, err = enc.Intimidate(context.Background(), &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: "straggler"})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clock.Kind, "precondition: it arrived, on the world clock")

	data := enc.ToData()
	progress := data.Clock.DriverProgress["straggler"]
	s.GreaterOrEqual(progress, rested, "seated at the reading it arrived into, not at zero")
	s.LessOrEqual(progress, data.Clock.HighWater)
}

// A rest that names nobody, or somebody twice, is refused and writes
// nothing.
func (s *SessionVerbsSuite) TestARestNamingNobodyOrSomebodyTwiceIsRefused() {
	enc := s.freeRoam()
	before := s.highWater(enc)

	_, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort})
	s.ErrorIs(err, encounter.ErrNoMember, "nobody rested")
	_, err = enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{
		{Member: alice}, {Member: bob}, {Member: alice},
	}})
	s.ErrorIs(err, encounter.ErrInvalidData, "alice twice")
	_, err = enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{
		{Member: alice}, {Member: "nobody"},
	}})
	s.ErrorIs(err, encounter.ErrNotMember)

	s.Equal(before, s.highWater(enc))
	s.Empty(s.beatsOfKind(enc, alice, encounter.BeatRested), "no refusal wrote alice's beat")
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

	_, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{{
		Member: alice, HitDiceSpent: 2, HitPointsRestored: 12, Calculation: calc,
		HitPoints: 20, HitDiceRemaining: 1,
	}}})
	s.Require().NoError(err)
	beats := s.beatsOfKind(enc, alice, encounter.BeatRested)
	s.Require().Len(beats, 1)
	s.Equal(float64(2), beats[0]["hit_dice_spent"])
	s.Equal(float64(12), beats[0]["hit_points_restored"], "capped by the rulebook, carried as told")
	s.Equal(float64(20), beats[0]["hit_points"])
	s.Equal(float64(0), beats[0]["hit_dice_returned"], "a short rest returns none, and says so")
	s.Equal(float64(1), beats[0]["hit_dice_remaining"])
	s.NotNil(beats[0]["calculation"])

	short := func(m encounter.RestingMember) *encounter.RecordRestInput {
		m.Member = alice
		return &encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{m}}
	}
	refused := map[string]*encounter.RecordRestInput{
		"no kind":                     {Members: []encounter.RestingMember{{Member: alice}}},
		"a long rest":                 {Kind: "long", Members: []encounter.RestingMember{{Member: alice}}},
		"negative dice":               short(encounter.RestingMember{HitDiceSpent: -1}),
		"negative hit points":         short(encounter.RestingMember{HitPointsRestored: -1}),
		"dice with no arithmetic":     short(encounter.RestingMember{HitDiceSpent: 1}),
		"arithmetic with no dice":     short(encounter.RestingMember{Calculation: calc}),
		"an unnamed refill":           short(encounter.RestingMember{ResourcesRefilled: []string{""}}),
		"negative hit points now":     short(encounter.RestingMember{HitPoints: -1}),
		"negative dice returned":      short(encounter.RestingMember{HitDiceReturned: -1}),
		"a short rest returning dice": short(encounter.RestingMember{HitDiceReturned: 3}),
		"negative dice remaining":     short(encounter.RestingMember{HitDiceRemaining: -1}),
	}
	for name, in := range refused {
		_, err := enc.RecordRest(in)
		s.ErrorIs(err, encounter.ErrInvalidData, name)
	}
	s.Len(s.beatsOfKind(enc, alice, encounter.BeatRested), 1, "no refusal wrote a beat")
}

// What a rest ENDED rides on the member's own rested beat: the concentration
// it broke, with the conditions that spell was holding, and every other
// condition or effect that came off — omitted when the rest ended nothing.
func (s *SessionVerbsSuite) TestARestCarriesWhatItEnded() {
	enc := s.freeRoam()
	bless := encounter.ConcentrationBreak{
		Caster: alice,
		Spell:  encounter.SpellIdentity{Ref: "dnd5e:spells:bless", Name: "Bless"},
		Reason: "duration",
		Removed: []encounter.ActivationResult{{
			Kind:    encounter.ResultConditionRemoved,
			Address: &encounter.ConditionAddress{MemberID: bob, ConditionKey: encounter.ConditionKey{ConditionRef: "dnd5e:conditions:blessed"}},
			Name:    "Blessed", Reason: "concentration ended",
		}},
	}
	prone := &core.Ref{Module: "dnd5e", Type: "conditions", ID: "prone"}

	_, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{
		{Member: alice, ConcentrationBreaks: []encounter.ConcentrationBreak{bless}, Ended: []*core.Ref{prone}},
		{Member: bob},
	}})
	s.Require().NoError(err)

	var aliceBeat, bobBeat map[string]any
	for _, beat := range s.beatsOfKind(enc, bob, encounter.BeatRested) {
		switch beat["member"] {
		case string(alice):
			aliceBeat = beat
		case string(bob):
			bobBeat = beat
		}
	}
	s.Require().NotNil(aliceBeat)
	s.Require().NotNil(bobBeat)

	ended, ok := aliceBeat["concentration_ended"].([]any)
	s.Require().True(ok, "alice's Bless ended with the rest")
	s.Require().Len(ended, 1)
	first := ended[0].(map[string]any)
	s.Equal(string(alice), first["caster"])
	s.Equal("duration", first["reason"])
	s.Equal("Bless", first["spell"].(map[string]any)["name"])
	s.Len(first["removed"], 1, "bob's blessing came off with it")
	s.Equal([]any{"dnd5e:conditions:prone"}, aliceBeat["ended"], "and alice is no longer prone")

	s.NotContains(bobBeat, "concentration_ended", "a rest that ended nothing says nothing")
	s.NotContains(bobBeat, "ended")

	refused := map[string]encounter.RestingMember{
		"somebody else's concentration": {Member: bob, ConcentrationBreaks: []encounter.ConcentrationBreak{bless}},
		"a break with a save":           {Member: alice, ConcentrationBreaks: []encounter.ConcentrationBreak{withSave(bless)}},
		"a break with no reason":        {Member: alice, ConcentrationBreaks: []encounter.ConcentrationBreak{withoutReason(bless)}},
		"a nil ended ref":               {Member: alice, Ended: []*core.Ref{nil}},
		"an invalid ended ref":          {Member: alice, Ended: []*core.Ref{{Module: "dnd5e"}}},
	}
	before, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	for name, m := range refused {
		_, err := enc.RecordRest(&encounter.RecordRestInput{Kind: encounter.RestShort, Members: []encounter.RestingMember{m}})
		s.ErrorIs(err, encounter.ErrInvalidData, name)
	}
	after, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	s.Equal(string(before), string(after), "no refusal wrote anything")
}

func withSave(b encounter.ConcentrationBreak) encounter.ConcentrationBreak {
	b.Save = &encounter.CastSave{}
	return b
}

func withoutReason(b encounter.ConcentrationBreak) encounter.ConcentrationBreak {
	b.Reason = ""
	return b
}

func paceOf(data encounter.EncounterData, member encounter.MemberID) int {
	for _, m := range data.Members {
		if m.ID == member {
			return m.Pace
		}
	}
	return 0
}
