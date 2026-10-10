// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// train_test.go pins RecordTrain (rpg-toolkit#2002): one landing tells all its
// beats, then asks who is standing once.
//
// Every scene here is the landing's world. The sheets were already written
// when the record step runs, so the standing capability reports the target
// down from the very start. That is what makes the order observable: a verb
// that consulted between two units would report the fall ahead of the second
// blow, or close the encounter between two swings.

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// swing is one struck outcome, the unit a multiattack tells twice.
func swing(actor, target encounter.MemberID) *encounter.RecordInput {
	return &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: actor, Targets: []encounter.MemberID{target},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	}
}

// outcomeTrain wraps outcomes as a train in story order.
func outcomeTrain(outcomes ...*encounter.RecordInput) *encounter.RecordTrainInput {
	units := make([]encounter.TrainUnit, 0, len(outcomes))
	for _, outcome := range outcomes {
		units = append(units, encounter.TrainUnit{Outcome: outcome})
	}
	return &encounter.RecordTrainInput{Units: units}
}

// beatsFrom is the kinds of every shared beat one member was told from seq on.
func beatsFrom(t *testing.T, enc *encounter.Encounter, who encounter.MemberID, seq uint64) []string {
	t.Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	require.NoError(t, err)
	kinds := make([]string, 0)
	for _, entry := range story {
		if entry.Seq < seq {
			continue
		}
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		kind := beat["beat"].(string)
		if recipientScopedKinds[kind] {
			continue
		}
		kinds = append(kinds, kind)
	}
	return kinds
}

// seqOfBeat is the seq of the nth beat of a kind, failing when there is none.
func seqOfBeat(t *testing.T, enc *encounter.Encounter, who encounter.MemberID, kind string, nth int) uint64 {
	t.Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	require.NoError(t, err)
	seen := 0
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] != kind {
			continue
		}
		if seen == nth {
			return entry.Seq
		}
		seen++
	}
	require.Failf(t, "beat not found", "no %s beat number %d", kind, nth)
	return 0
}

// requireTold fails unless the story from seq on STARTS with want. What
// follows the last told beat is the world's own consequence (an ending, a
// sight refresh, the world-action price) and is pinned where it matters.
func requireTold(t *testing.T, enc *encounter.Encounter, who encounter.MemberID, seq uint64, want ...string) {
	t.Helper()
	got := beatsFrom(t, enc, who, seq)
	require.GreaterOrEqual(t, len(got), len(want), "told: %v", got)
	require.Equal(t, want, got[:len(want)])
}

func storyLen(t *testing.T, enc *encounter.Encounter, who encounter.MemberID) int {
	t.Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	require.NoError(t, err)
	return len(story)
}

// fall is the landing's world for a party of one: the sheet already reads 0
// when the record step asks, and the rulebook says the party is defeated.
func fall(capability *scriptedParticipation, who encounter.MemberID) {
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		who: {Down: true, Turn: encounter.TurnParticipationWait},
	}
	capability.partyDefeated = true
	capability.keepTurnOrder = true
}

// aloneAgainstTheGoblin is a one-member party, so its fall is the party's.
func aloneAgainstTheGoblin(t *testing.T, capability encounter.StandingWithParticipation) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(participationSetup(capability,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
	))
	require.NoError(t, err)
	return enc
}

// TestATrainTellsEveryBeatBeforeAskingWhoIsStanding: both blows are in the
// story before the one question, so the fall follows the blow that caused it.
func TestATrainTellsEveryBeatBeforeAskingWhoIsStanding(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := participationTrio(t, capability)
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		alice: {Down: true, Turn: encounter.TurnParticipationWait},
	}

	out, err := enc.RecordTrain(outcomeTrain(swing(goblin, alice), swing(goblin, alice)))
	require.NoError(t, err)
	require.Len(t, out.Units, 2)

	require.Equal(t, []string{"struck", "struck", "down"}, beatsFrom(t, enc, bob, out.Units[0].Seq))
	downSeq := seqOfBeat(t, enc, bob, "down", 0)
	require.Less(t, out.Units[0].Seq, out.Units[1].Seq)
	require.Less(t, out.Units[1].Seq, downSeq)
}

// TestATrainThatFellsTheLastStandingClosesAfterItsLastBeat: the party's fall
// closes the encounter after the second blow, not between the two.
func TestATrainThatFellsTheLastStandingClosesAfterItsLastBeat(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := aloneAgainstTheGoblin(t, capability)
	fall(capability, alice)

	out, err := enc.RecordTrain(outcomeTrain(swing(goblin, alice), swing(goblin, alice)))
	require.NoError(t, err)

	require.Equal(t, []string{"struck", "struck", "down", "ended"}, beatsFrom(t, enc, alice, out.Units[0].Seq))
	status, err := enc.Status()
	require.NoError(t, err)
	require.False(t, status.Open)
	require.Equal(t, "party_defeated", status.Outcome.Ending)
}

// TestEachUnitsConcentrationRidesBehindItsOwnBeat: unit one's break is told
// before unit two's blow, and the fall follows both.
func TestEachUnitsConcentrationRidesBehindItsOwnBeat(t *testing.T) {
	capability := &scriptedParticipation{}
	enc, err := encounter.NewEncounter(participationSetup(capability,
		encounter.MemberInput{ID: castBard, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
		encounter.MemberInput{ID: castFighter, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 2}},
		encounter.MemberInput{ID: castSkeleton, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10}},
	))
	require.NoError(t, err)
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		castBard: {Down: true, Turn: encounter.TurnParticipationWait},
	}

	broken := brokenByDamage()
	broken.Removed = nil
	out, err := enc.RecordTrain(outcomeTrain(theSkeletonHits(broken), theSkeletonHits()))
	require.NoError(t, err)

	requireTold(t, enc, castFighter, out.Units[0].Seq, "struck", "saved", "concentration_ended", "struck", "down")
	require.Len(t, out.Units[0].FollowUpSeqs, 2, "the check and the break ride behind unit one")
	require.Empty(t, out.Units[1].FollowUpSeqs)
	require.Less(t, out.Units[0].FollowUpSeqs[1], out.Units[1].Seq)
}

// TestABadUnitAnywhereRefusesTheWholeTrain: unit two is malformed, so unit
// one neither lands its beat nor its deed.
func (s *BothWaysSuite) TestABadUnitAnywhereRefusesTheWholeTrain() {
	enc := s.camp()
	before := storyLen(s.T(), enc, alice)

	bad := swing(alice, bwScout)
	bad.Attack = &encounter.AttackIdentity{Ref: "", Name: "Scimitar"}
	_, err := enc.RecordTrain(outcomeTrain(swing(alice, bwScout), bad))

	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Equal(before, storyLen(s.T(), enc, alice), "nothing was appended")
	s.Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty),
		"and unit one's deed never landed")
	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice))
}

// TestEachStruckUnitLandsItsDeedBehindItsOwnBeats: the camp turns beside the
// blow that turned it, before the next blow is told.
func (s *BothWaysSuite) TestEachStruckUnitLandsItsDeedBehindItsOwnBeats() {
	enc := s.camp()

	out, err := enc.RecordTrain(outcomeTrain(swing(alice, bwScout), swing(alice, bwScout)))
	s.Require().NoError(err)

	stanceSeq := seqOfBeat(s.T(), enc, alice, "stance", 0)
	s.Less(out.Units[0].Seq, stanceSeq, "after unit one's beat")
	s.Less(stanceSeq, out.Units[1].Seq, "and before unit two's")
}

// warCamp is a neutral goblin camp with an ending on the camp turning hostile.
// The first blow at the camp turns it, so the deed asks for that ending.
func (s *BothWaysSuite) warCamp(
	driver encounter.Driver, striker encounter.Striker, standing encounter.StandingWithParticipation,
	members ...encounter.MemberInput,
) *encounter.Encounter {
	if len(members) == 0 {
		members = []encounter.MemberInput{
			player(alice, 0, 1),
			monster(bwScout, bwGoblins, 4, 1),
			monster(bwChief, bwGoblins, 5, 5),
		}
	}
	return s.openDriven(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins}},
			[]encounter.DispositionInput{{
				Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
			false,
		),
		members,
		driver, striker, standing,
		withdrawn(),
		encounter.EndingInput{Key: "war", Trigger: encounter.TriggerStance{
			Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
			Stance:  encounter.StanceHostile,
		}},
	)
}

// TestAStanceEndingWaitsForTheLastBeatOfTheTrain is the gate's probe (T5): a
// camp that is civil until the party strikes it, with an ending on the camp
// turning hostile. The first swing's deed turns the camp and asks for that
// ending. Both swings are still told, then the stance, then the ending; the
// train neither closes between the units nor refuses the second.
func (s *BothWaysSuite) TestAStanceEndingWaitsForTheLastBeatOfTheTrain() {
	enc := s.warCamp(passDriver{}, passStriker{}, s.standing)

	out, err := enc.RecordTrain(outcomeTrain(swing(alice, bwScout), swing(alice, bwScout)))
	s.Require().NoError(err, "the second swing is told, not refused")

	status, err := enc.Status()
	s.Require().NoError(err)
	s.False(status.Open)
	s.Equal("war", status.Outcome.Ending)

	stanceSeq := seqOfBeat(s.T(), enc, alice, "stance", 0)
	endedSeq := seqOfBeat(s.T(), enc, alice, "ended", 0)
	s.Less(out.Units[0].Seq, stanceSeq, "the stance is told behind the blow that turned it")
	s.Less(stanceSeq, out.Units[1].Seq, "and before the next blow")
	s.Less(out.Units[1].Seq, endedSeq, "the ending follows the last beat")
	kinds := beatsFrom(s.T(), enc, alice, out.Units[0].Seq)
	s.Equal("ended", kinds[len(kinds)-1], "and nothing is told after it")
}

// TestAFightFormedMidTrainDoesNotAskWhoIsStandingUntilTheEnd: swing one forms
// the fight (the stance and the bubble are told in place), swing two fells
// the target. The fall is told once, after the second swing.
func (s *BothWaysSuite) TestAFightFormedMidTrainDoesNotAskWhoIsStandingUntilTheEnd() {
	const (
		ant  = core.EntityID("ant")
		zed  = core.EntityID("zed")
		boss = core.EntityID("boss")
	)
	enc := s.warCamp(passDriver{}, passStriker{}, s.standing,
		monster(ant, bwGoblins, 4, 1), monster(boss, bwGoblins, 5, 5), player(zed, 0, 1))
	s.standing.down = []encounter.MemberID{ant}

	out, err := enc.RecordTrain(outcomeTrain(swing(zed, ant), swing(zed, ant)))
	s.Require().NoError(err)

	kinds := beatsFrom(s.T(), enc, zed, out.Units[0].Seq)
	s.Equal([]string{"struck", "stance", "bubble-formed", "struck", "down", "ended"}, kinds,
		"the fight forms in place and drives nothing; the fall waits for the last swing")
}

// TestFormationMidTrainDrivesNothingUntilTheEnd: the ant, who falls to the
// second swing, holds the first slot of the fight the first swing forms. The
// boss behind it would be driven the moment the fight formed, and its strike
// would tell the ant's fall before the second swing.
// Nothing is driven mid-train: the post-train pass removes the ant, then
// drives the boss, and the boss's turn follows the whole story.
func (s *BothWaysSuite) TestFormationMidTrainDrivesNothingUntilTheEnd() {
	const (
		ant  = core.EntityID("ant")
		zed  = core.EntityID("zed")
		boss = core.EntityID("boss")
	)
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Attack{Target: zed, Action: testMeleeAction},
	}}
	enc := s.openDriven(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins}},
			[]encounter.DispositionInput{{
				Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
			false,
		),
		[]encounter.MemberInput{
			monster(ant, bwGoblins, 4, 1), monster(boss, bwGoblins, 5, 5), player(zed, 0, 1),
		},
		driver, passStriker{}, s.standing, withdrawn(),
	)
	s.standing.down = []encounter.MemberID{ant}

	out, err := enc.RecordTrain(outcomeTrain(swing(zed, ant), swing(zed, ant)))
	s.Require().NoError(err)

	kinds := beatsFrom(s.T(), enc, zed, out.Units[0].Seq)
	second := 3 // struck, stance, bubble-formed, then the second swing
	s.Require().Greater(len(kinds), second)
	s.Equal([]string{"struck", "stance", "bubble-formed", "struck", "down"}, kinds[:5],
		"nothing is told between the two swings but the fight forming: %v", kinds)
	s.Equal(1, len(driver.calls), "and the boss is driven after the story, once")
	s.Equal("turn-ended", kinds[len(kinds)-1], "its turn follows the whole story: %v", kinds)
}

// TestAClosedEncounterRefusesATrain: a closed story takes nothing but an
// experience grant.
func TestAClosedEncounterRefusesATrain(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := aloneAgainstTheGoblin(t, capability)
	fall(capability, alice)
	_, err := enc.RecordTrain(outcomeTrain(swing(goblin, alice)))
	require.NoError(t, err)
	status, err := enc.Status()
	require.NoError(t, err)
	require.False(t, status.Open, "precondition: the encounter is closed")
	before := storyLen(t, enc, alice)

	_, err = enc.RecordTrain(outcomeTrain(swing(goblin, alice)))
	require.ErrorIs(t, err, encounter.ErrClosed)
	require.Equal(t, before, storyLen(t, enc, alice), "nothing was appended")

	_, err = enc.RecordTrain(outcomeTrain(&encounter.RecordInput{
		Kind: encounter.OutcomeExperienceGained, Actor: goblin, Experience: aGrant(),
	}))
	require.NoError(t, err, "an experience grant is the one train a closed encounter takes")
	require.Equal(t, before+1, storyLen(t, enc, alice))

	_, err = enc.RecordTrain(outcomeTrain(
		&encounter.RecordInput{Kind: encounter.OutcomeExperienceGained, Actor: goblin, Experience: aGrant()},
		swing(goblin, alice),
	))
	require.ErrorIs(t, err, encounter.ErrClosed, "one other unit refuses the whole train")
	require.Equal(t, before+1, storyLen(t, enc, alice), "and the experience unit before it was not appended")
}

// TestAnEmptyOrMalformedTrainIsRefused pins the train's own shape refusals.
func TestAnEmptyOrMalformedTrainIsRefused(t *testing.T) {
	enc := participationTrio(t, &scriptedParticipation{})
	before := storyLen(t, enc, alice)

	_, err := enc.RecordTrain(nil)
	require.ErrorIs(t, err, encounter.ErrNilInput)

	_, err = enc.RecordTrain(&encounter.RecordTrainInput{})
	require.ErrorIs(t, err, encounter.ErrInvalidData, "zero units")

	_, err = enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{
		Outcome:    swing(goblin, alice),
		Activation: &encounter.RecordActivationInput{Actor: alice},
	}}})
	require.ErrorIs(t, err, encounter.ErrInvalidData, "both fields")

	_, err = enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{}}})
	require.ErrorIs(t, err, encounter.ErrInvalidData, "neither field")

	require.Equal(t, before, storyLen(t, enc, alice))
}

// retaliation is alice answering the goblin's blow: a failed save, then damage
// to the striker.
func retaliation() *encounter.RecordActivationInput {
	wrath := encounter.SpellIdentity{Ref: "dnd5e:features:wrath_of_the_storm", Name: "Wrath of the Storm"}
	return &encounter.RecordActivationInput{
		Actor: alice, Target: goblin,
		Ability: encounter.ActivationIdentity(wrath),
		Save: &encounter.CastSave{
			Saver: goblin, Ability: "dexterity", Roll: 4, Total: 6, DC: 12, Succeeded: false,
			Calculation: saveCalculation(wrath, "dexterity", 4, 6),
		},
		Results: []encounter.ActivationResult{{
			Kind: encounter.ResultDamageApplied, Target: goblin, Ref: wrath.Ref, Name: wrath.Name,
			Amount: 3, Requested: 3, Before: 3, After: 0, DamageType: "lightning",
			Calculation: &encounter.RollCalculation{
				Components: []encounter.RollComponent{{
					Source: encounter.RollSource{Ref: wrath.Ref, Name: wrath.Name, SourceID: "hero-1"},
					Dice: &encounter.DiceTrace{
						Notation: "1d4", DieSize: 4, OriginalRolls: []int{3}, FinalRolls: []int{3}, Subtotal: 3,
					},
				}},
				Total: 3,
			},
		}},
	}
}

// TestARetaliationUnitIsToldInsideTheTrain: the striker's fall follows the
// reaction that felled it.
func TestARetaliationUnitIsToldInsideTheTrain(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := aloneAgainstTheGoblin(t, capability)
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		goblin: {Down: true, Turn: encounter.TurnParticipationWait},
	}

	out, err := enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{
		{Outcome: swing(goblin, alice)},
		{Activation: retaliation()},
	}})
	require.NoError(t, err)

	requireTold(t, enc, alice, out.Units[0].Seq, "struck", "activated", "saved", "activation-result", "down")
	require.Equal(t, seqOfBeat(t, enc, alice, "activated", 0), out.Units[1].Seq,
		"the unit's own beat is the activated beat")
	require.Len(t, out.Units[1].FollowUpSeqs, 2, "its save and its result")
}

// TestAnActivationIsATrainOfOne: RecordActivation is one unit through the one
// path, so it gains the one standing consult and the observed-standing refresh.
func TestAnActivationIsATrainOfOne(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := aloneAgainstTheGoblin(t, capability)
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		goblin: {Down: true, Turn: encounter.TurnParticipationWait},
	}

	viaVerb, err := enc.RecordActivation(retaliation())
	require.NoError(t, err)
	require.Len(t, viaVerb.Seqs, 3)

	requireTold(t, enc, alice, viaVerb.Seqs[0], "activated", "saved", "activation-result", "down")
	told := beatsFrom(t, enc, alice, viaVerb.Seqs[0])
	require.Equal(t, "tick", told[len(told)-1], "the world-action price is paid after the consult")

	otherCapability := &scriptedParticipation{}
	other := aloneAgainstTheGoblin(t, otherCapability)
	otherCapability.members = capability.members
	viaTrain, err := other.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{
		{Activation: retaliation()},
	}})
	require.NoError(t, err)
	require.Equal(t, viaVerb.Seqs, append([]uint64{viaTrain.Units[0].Seq}, viaTrain.Units[0].FollowUpSeqs...))
	require.Equal(t, beatsFrom(t, other, alice, viaTrain.Units[0].Seq), beatsFrom(t, enc, alice, viaVerb.Seqs[0]))
}

// TestASwingNamesItsSequence pins the T4 marker on the beat.
func TestASwingNamesItsSequence(t *testing.T) {
	sequence := &encounter.SequenceIdentity{Ref: "dnd5e:monster-actions:goblin-boss:multiattack", Name: "Multiattack"}

	t.Run("a swing carries it", func(t *testing.T) {
		enc := participationTrio(t, &scriptedParticipation{})
		hit := swing(goblin, alice)
		hit.Sequence = sequence
		out, err := enc.RecordTrain(outcomeTrain(hit))
		require.NoError(t, err)

		story, err := enc.Story(&encounter.StoryInput{Audience: alice})
		require.NoError(t, err)
		for _, entry := range story {
			if entry.Seq != out.Units[0].Seq {
				continue
			}
			var beat map[string]any
			require.NoError(t, json.Unmarshal(entry.Payload, &beat))
			require.Equal(t, map[string]any{"ref": sequence.Ref, "name": sequence.Name}, beat["sequence"])
			return
		}
		require.Fail(t, "the unit's beat is not in the story")
	})

	t.Run("a lone swing writes no key", func(t *testing.T) {
		enc := participationTrio(t, &scriptedParticipation{})
		out, err := enc.RecordTrain(outcomeTrain(swing(goblin, alice)))
		require.NoError(t, err)

		story, err := enc.Story(&encounter.StoryInput{Audience: alice})
		require.NoError(t, err)
		for _, entry := range story {
			if entry.Seq == out.Units[0].Seq {
				require.NotContains(t, string(entry.Payload), `"sequence"`)
			}
		}
	})

	t.Run("a miss and a ward may carry it", func(t *testing.T) {
		enc := participationTrio(t, &scriptedParticipation{})
		miss := swing(goblin, alice)
		miss.Kind = encounter.OutcomeMissed
		miss.Sequence = sequence
		_, err := enc.RecordTrain(outcomeTrain(miss))
		require.NoError(t, err)

		// A ward is the caster's own failed save against somebody's
		// Sanctuary: the saver is the actor, and the save failed.
		wardSpell := encounter.SpellIdentity{Ref: "dnd5e:spells:sanctuary", Name: "Sanctuary"}
		ward := &encounter.RecordInput{
			Kind: encounter.OutcomeWarded, Actor: goblin, Targets: []encounter.MemberID{alice},
			Warded: &encounter.WardedDetail{
				Source: alice,
				Save: encounter.CastSave{
					Saver: goblin, Ability: "wisdom", Roll: 4, Total: 6, DC: 12, Succeeded: false,
					Calculation: saveCalculation(wardSpell, "wisdom", 4, 6),
				},
			},
			Sequence: sequence,
		}
		_, err = enc.RecordTrain(outcomeTrain(ward))
		require.NoError(t, err, "a warded swing may carry the marker")
	})

	refused := map[string]*encounter.RecordInput{
		"death save": {
			Kind: encounter.OutcomeDeathSave, Actor: alice,
			DeathSave: &encounter.DeathSaveDetail{
				Roll: 12, Outcome: "success", SuccessesNeeded: 2, FailuresRemaining: 3,
				Continuation: "end_turn", PresentationID: "death-save",
			},
		},
		"trade": {
			Kind: encounter.OutcomeBought, Actor: alice, Targets: []encounter.MemberID{goblin},
			Trade: &encounter.TradeDetail{ItemType: "weapon", ItemID: "longsword", Quantity: 1},
		},
	}
	for name, in := range refused {
		t.Run(name+" refuses it", func(t *testing.T) {
			enc := participationTrio(t, &scriptedParticipation{})
			control := *in
			_, err := enc.RecordTrain(outcomeTrain(&control))
			require.NoError(t, err, "control: the same outcome is valid without a sequence")

			with := *in
			with.Sequence = sequence
			_, err = enc.RecordTrain(outcomeTrain(&with))
			require.ErrorIs(t, err, encounter.ErrInvalidData)
		})
	}

	t.Run("an empty ref or name is refused", func(t *testing.T) {
		enc := participationTrio(t, &scriptedParticipation{})
		for _, bad := range []*encounter.SequenceIdentity{
			{Ref: "", Name: "Multiattack"}, {Ref: sequence.Ref, Name: ""},
		} {
			hit := swing(goblin, alice)
			hit.Sequence = bad
			_, err := enc.RecordTrain(outcomeTrain(hit))
			require.ErrorIs(t, err, encounter.ErrInvalidData)
		}
	})
}

// TestPartyDefeatBeatsAParkedEnding: the last blow fells the last of the party
// and also turned the camp. One ending is told, and it is the defeat.
func (s *BothWaysSuite) TestPartyDefeatBeatsAParkedEnding() {
	enc := s.warCamp(passDriver{}, passStriker{}, s.standing)
	s.standing.down = []encounter.MemberID{alice}
	s.standing.partyDefeated = true

	_, err := enc.RecordTrain(outcomeTrain(swing(bwScout, alice), swing(bwScout, alice)))
	s.Require().NoError(err)

	status, err := enc.Status()
	s.Require().NoError(err)
	s.False(status.Open)
	s.Equal("party_defeated", status.Outcome.Ending)
	count := 0
	for _, kind := range beatsFrom(s.T(), enc, alice, 0) {
		if kind == "ended" {
			count++
		}
	}
	s.Equal(1, count, "one ending, and the stance ending was dropped")
}

// TestAParkedEndingBeatsADeclaredMemberDownEnding pins the order the godoc
// states: the fall of the scout would end the run as "scout-fell", but the
// stance ending the same blow parked is evaluated first.
func (s *BothWaysSuite) TestAParkedEndingBeatsADeclaredMemberDownEnding() {
	enc := s.openDriven(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins}},
			[]encounter.DispositionInput{{
				Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
			false,
		),
		[]encounter.MemberInput{player(alice, 0, 1), monster(bwScout, bwGoblins, 4, 1), monster(bwChief, bwGoblins, 5, 5)},
		passDriver{}, passStriker{}, s.standing,
		withdrawn(),
		encounter.EndingInput{Key: "war", Trigger: encounter.TriggerStance{
			Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
			Stance:  encounter.StanceHostile,
		}},
		encounter.EndingInput{Key: "scout-fell", Trigger: encounter.TriggerMemberDown{Member: bwScout}},
	)
	s.standing.down = []encounter.MemberID{bwScout}

	_, err := enc.RecordTrain(outcomeTrain(swing(alice, bwScout)))
	s.Require().NoError(err)

	status, err := enc.Status()
	s.Require().NoError(err)
	s.Equal("war", status.Outcome.Ending)
}

// TestAFailedTrainLeavesNoParkedEnding: a train that dies at any point after
// its deed parked an ending leaves nothing behind for the next train to close
// with. The rulebook is made unreachable at each of its questions in turn.
func (s *BothWaysSuite) TestAFailedTrainLeavesNoParkedEnding() {
	broken := errors.New("rulebook unreachable")
	dry := &downList{}
	dryEnc := s.warCamp(passDriver{}, passStriker{}, dry)
	dry.calls = 0
	_, err := dryEnc.RecordTrain(outcomeTrain(swing(alice, bwScout)))
	s.Require().NoError(err)
	total := dry.calls
	s.Require().Positive(total)

	survived := 0
	for failAt := 1; failAt <= total; failAt++ {
		standing := &downList{}
		enc := s.warCamp(passDriver{}, passStriker{}, standing)
		standing.calls, standing.failAt, standing.fail = 0, failAt, broken
		_, err := enc.RecordTrain(outcomeTrain(swing(alice, bwScout)))
		if err == nil {
			continue
		}
		status, serr := enc.Status()
		s.Require().NoError(serr)
		if !status.Open {
			continue
		}
		survived++
		standing.fail = nil
		_, err = enc.RecordTrain(outcomeTrain(swing(alice, bwScout)))
		s.Require().NoError(err)
		status, serr = enc.Status()
		s.Require().NoError(serr)
		s.True(status.Open, "failing at question %d left an ending parked for the next train", failAt)
	}
	s.Positive(survived, "some failure must land after the deed parked and before anything closed")
}
