// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	castBard     encounter.MemberID = "bard"
	castFighter  encounter.MemberID = "cast-fighter"
	castSkeleton encounter.MemberID = "cast-skeleton"
)

// viciousMockery and trueStrike are the two halves of every cantrip: one rolls
// a save to see whether it delivers, one just delivers.
var (
	viciousMockery = encounter.SpellIdentity{
		Ref: "dnd5e:spells:vicious-mockery", Name: "Vicious Mockery",
	}
	trueStrike = encounter.SpellIdentity{
		Ref: "dnd5e:spells:true-strike", Name: "True Strike",
	}
)

// RecordCastSuite pins the encounter-owned transaction that turns one cast,
// the save its gate rolled, and the effects it delivered into durable story.
type RecordCastSuite struct {
	suite.Suite
}

func TestRecordCastSuite(t *testing.T) {
	suite.Run(t, new(RecordCastSuite))
}

// scene keeps the skeleton behind a wall so first light does not form a fight;
// cast tests need only a roster, a record, and an observable Standing.
func (s *RecordCastSuite) scene(standing encounter.Standing) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Retention: encounter.RetentionUnbounded,
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("cast-yard", 0, 0, 12, 12)},
			Props:   wallRow(6, 4, 8),
		},
		Members: []encounter.MemberInput{
			{ID: castBard, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: castFighter, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 2}},
			{ID: castSkeleton, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

func (s *RecordCastSuite) storyEntries(
	enc *encounter.Encounter, who encounter.MemberID, seqs []uint64,
) []record.Entry {
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	s.Require().NoError(err)

	bySeq := make(map[uint64]record.Entry, len(story))
	for _, entry := range story {
		bySeq[entry.Seq] = entry
	}
	out := make([]record.Entry, 0, len(seqs))
	for _, seq := range seqs {
		entry, ok := bySeq[seq]
		s.Require().True(ok, "%s did not receive seq %d", who, seq)
		out = append(out, entry)
	}
	return out
}

func (s *RecordCastSuite) beatNames(entries []record.Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		var beat struct {
			Beat   string `json:"beat"`
			Result struct {
				Kind string `json:"kind"`
			} `json:"result"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Result.Kind != "" {
			names = append(names, beat.Result.Kind)
			continue
		}
		names = append(names, beat.Beat)
	}
	return names
}

// psychicDamage is Vicious Mockery's 1d4 landing on the skeleton: the face is
// 3, the requested damage is 3, and the sheet applied all of it.
func psychicDamage() encounter.ActivationResult {
	return encounter.ActivationResult{
		Kind:       encounter.ResultDamageApplied,
		Target:     castSkeleton,
		Ref:        viciousMockery.Ref,
		Name:       viciousMockery.Name,
		Amount:     3,
		Requested:  3,
		Before:     7,
		After:      4,
		DamageType: "psychic",
		Calculation: &encounter.RollCalculation{
			Components: []encounter.RollComponent{
				{
					Source: encounter.RollSource{Ref: viciousMockery.Ref, Name: viciousMockery.Name},
					Dice: &encounter.DiceTrace{
						Notation:      "1d4",
						DieSize:       4,
						OriginalRolls: []int{3},
						FinalRolls:    []int{3},
						Subtotal:      3,
					},
				},
			},
			Total: 3,
		},
	}
}

func mockedCondition() encounter.ActivationResult {
	return encounter.ActivationResult{
		Kind:   encounter.ResultConditionApplied,
		Target: castSkeleton,
		Ref:    "dnd5e:conditions:mocked",
		Name:   "Mocked",
	}
}

func failedSave() *encounter.CastSave {
	return &encounter.CastSave{
		Saver: castSkeleton, Ability: "wisdom", Roll: 6, Total: 8, DC: 13, Succeeded: false,
	}
}

func viciousMockeryCast() *encounter.RecordCastInput {
	return &encounter.RecordCastInput{
		Actor:   castBard,
		Target:  castSkeleton,
		Spell:   viciousMockery,
		Save:    failedSave(),
		Results: []encounter.ActivationResult{psychicDamage(), mockedCondition()},
	}
}

// TestAGatedCastReadsCastSaveDamageCondition is the whole slice in one scene,
// and it is the walk's own log: the order the table reads is the order the
// beats land in.
func (s *RecordCastSuite) TestAGatedCastReadsCastSaveDamageCondition() {
	standing := &countingStanding{}
	enc := s.scene(standing)
	callsBefore := standing.calls

	out, err := enc.RecordCast(viciousMockeryCast())
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 4)
	for i := 1; i < len(out.Seqs); i++ {
		s.Less(out.Seqs[i-1], out.Seqs[i], "beat %d precedes beat %d", i-1, i)
	}
	s.Equal(callsBefore+1, standing.calls, "noticeDown is consulted once for the whole transaction")

	entries := s.storyEntries(enc, castBard, out.Seqs)
	s.Equal([]string{"cast", "saved", "damage-applied", "condition-applied"}, s.beatNames(entries))
}

// TestTheCastAndSavedPayloads pins both new bodies exactly, key for key. The
// save carries the roll, the total, the DC and the answer, and names the spell
// it was against.
func (s *RecordCastSuite) TestTheCastAndSavedPayloads() {
	enc := s.scene(everyoneStanding{})

	out, err := enc.RecordCast(viciousMockeryCast())
	s.Require().NoError(err)

	entries := s.storyEntries(enc, castBard, out.Seqs)
	s.Equal(
		`{"beat":"cast","actor":"bard",`+
			`"spell":{"ref":"dnd5e:spells:vicious-mockery","name":"Vicious Mockery"},`+
			`"target":"cast-skeleton"}`,
		string(entries[0].Payload),
	)
	s.Equal(
		`{"beat":"saved","saver":"cast-skeleton","ability":"wisdom","roll":6,"total":8,"dc":13,`+
			`"succeeded":false,`+
			`"source":{"ref":"dnd5e:spells:vicious-mockery","name":"Vicious Mockery"}}`,
		string(entries[1].Payload),
	)
	s.Equal("outcome", entries[0].Tags["tag"])
	s.Equal("outcome", entries[1].Tags["tag"])
}

// TestTheDamageCarriesItsAmountAndItsFace is why damage is shaped like
// healing: the player is owed the die that was rolled, not just the number
// that was subtracted.
func (s *RecordCastSuite) TestTheDamageCarriesItsAmountAndItsFace() {
	enc := s.scene(everyoneStanding{})

	out, err := enc.RecordCast(viciousMockeryCast())
	s.Require().NoError(err)

	entries := s.storyEntries(enc, castBard, out.Seqs)
	s.Equal(
		`{"beat":"activation-result","actor":"bard","result":{"kind":"damage-applied",`+
			`"target":"cast-skeleton","amount":3,"requested":3,"before":7,"after":4,`+
			`"calculation":{"components":[{"source":{"ref":"dnd5e:spells:vicious-mockery",`+
			`"name":"Vicious Mockery"},"dice":{"notation":"1d4","die_size":4,`+
			`"original_rolls":[3],"final_rolls":[3],"subtotal":3}}],"total":3},`+
			`"ref":"dnd5e:spells:vicious-mockery","name":"Vicious Mockery",`+
			`"damage_type":"psychic"}}`,
		string(entries[2].Payload),
	)
}

// TestAConditionStillRefusesRollFacts — widening damage did not open the door
// for every other kind. A condition has no roll behind it and may carry none.
func (s *RecordCastSuite) TestAConditionStillRefusesRollFacts() {
	enc := s.scene(everyoneStanding{})

	withCalculation := mockedCondition()
	withCalculation.Calculation = psychicDamage().Calculation
	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery,
		Results: []encounter.ActivationResult{withCalculation},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "record cast: result 0 condition-applied forbids calculation")

	withAmount := mockedCondition()
	withAmount.Amount = 3
	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery,
		Results: []encounter.ActivationResult{withAmount},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "forbids amount")
}

// TestDamageIsHeldToHealingsLaw — the calculation is required and its total
// must equal the requested damage, exactly as a healing's does. A damage
// number with no roll behind it is not writable.
func (s *RecordCastSuite) TestDamageIsHeldToHealingsLaw() {
	enc := s.scene(everyoneStanding{})

	noCalculation := psychicDamage()
	noCalculation.Calculation = nil
	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery, Save: failedSave(),
		Results: []encounter.ActivationResult{noCalculation},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "damage-applied requires a calculation")

	disagreeing := psychicDamage()
	disagreeing.Requested = 4
	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery, Save: failedSave(),
		Results: []encounter.ActivationResult{disagreeing},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "calculation total 3 does not equal the requested 4")
}

// TestTheDamageTypeIsRequiredAndNobodyElseMayCarryOne — the type is what the
// psychic 1d4 IS, so damage without one is not writable; and it belongs to
// damage alone, so a condition carrying one is a field filled in by mistake.
func (s *RecordCastSuite) TestTheDamageTypeIsRequiredAndNobodyElseMayCarryOne() {
	enc := s.scene(everyoneStanding{})

	untyped := psychicDamage()
	untyped.DamageType = ""
	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery, Save: failedSave(),
		Results: []encounter.ActivationResult{untyped},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "record cast: result 0 damage-applied damage type")

	typedCondition := mockedCondition()
	typedCondition.DamageType = "psychic"
	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery,
		Results: []encounter.ActivationResult{typedCondition},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "condition-applied forbids damage type")

	typedHealing := encounter.ActivationResult{
		Kind: encounter.ResultHealingApplied, Target: castBard,
		Ref: "dnd5e:spells:cure-wounds", Name: "Cure Wounds",
		DamageType: "psychic",
	}
	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Spell: viciousMockery,
		Results: []encounter.ActivationResult{typedHealing},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "healing-applied forbids damage type", "healing is damage's twin and still refuses one")
}

// TestAnUngatedCastAppendsNoSavedBeat is True Strike: no roll happened, so no
// beat says one did. A saved beat reading 0 against DC 0 would be a lie.
func (s *RecordCastSuite) TestAnUngatedCastAppendsNoSavedBeat() {
	enc := s.scene(everyoneStanding{})

	out, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard,
		Spell: trueStrike,
		Results: []encounter.ActivationResult{
			{
				Kind: encounter.ResultConditionApplied, Target: castBard,
				Ref: "dnd5e:conditions:true-strike", Name: "True Strike",
			},
		},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 2)

	entries := s.storyEntries(enc, castBard, out.Seqs)
	s.Equal([]string{"cast", "condition-applied"}, s.beatNames(entries))
	s.Equal(
		`{"beat":"cast","actor":"bard","spell":{"ref":"dnd5e:spells:true-strike","name":"True Strike"}}`,
		string(entries[0].Payload),
		"a cast with no named target carries no empty target key",
	)
}

// TestASuccessfulSaveIsACompleteCast — the save happened and nothing followed,
// and the record says exactly that.
func (s *RecordCastSuite) TestASuccessfulSaveIsACompleteCast() {
	enc := s.scene(everyoneStanding{})

	save := failedSave()
	save.Roll = 18
	save.Total = 20
	save.Succeeded = true

	out, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery, Save: save,
	})
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 2)

	entries := s.storyEntries(enc, castBard, out.Seqs)
	s.Equal([]string{"cast", "saved"}, s.beatNames(entries))

	var beat struct {
		Succeeded bool `json:"succeeded"`
		Total     int  `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(entries[1].Payload, &beat))
	s.True(beat.Succeeded)
	s.Equal(20, beat.Total)
}

// TestEverybodyReadsEveryBeat is the pre-v1 full-data rule: the audience is
// today's full sorted roster, the save's numbers included.
func (s *RecordCastSuite) TestEverybodyReadsEveryBeat() {
	enc := s.scene(everyoneStanding{})

	out, err := enc.RecordCast(viciousMockeryCast())
	s.Require().NoError(err)

	wantAudience := []string{"bard", "cast-fighter", "cast-skeleton"}
	for _, who := range []encounter.MemberID{castBard, castFighter, castSkeleton} {
		entries := s.storyEntries(enc, who, out.Seqs)
		s.Require().Len(entries, 4)
		for i, entry := range entries {
			got := make([]string, 0, len(entry.Audience))
			for _, id := range entry.Audience {
				got = append(got, string(id))
			}
			s.Equal(wantAudience, got, "%s at transaction offset %d", who, i)
		}
	}
}

// TestTheCastSurvivesAReload — the beats are durable story, not a live-object
// convenience. What the table read before the save is what it reads after.
func (s *RecordCastSuite) TestTheCastSurvivesAReload() {
	enc := s.scene(everyoneStanding{})

	out, err := enc.RecordCast(viciousMockeryCast())
	s.Require().NoError(err)
	before := s.storyEntries(enc, castBard, out.Seqs)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  enc.ToData(),
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	after := s.storyEntries(reloaded, castBard, out.Seqs)
	s.Require().Len(after, len(before))
	var damage struct {
		Result struct {
			DamageType string `json:"damage_type"`
		} `json:"result"`
	}
	s.Require().NoError(json.Unmarshal(after[2].Payload, &damage))
	s.Equal("psychic", damage.Result.DamageType, "the damage type survives the blob")
	for i := range before {
		s.Equal(string(before[i].Payload), string(after[i].Payload), "beat %d", i)
		s.Equal(before[i].Tags["tag"], after[i].Tags["tag"], "beat %d tag", i)
		s.Equal(before[i].Audience, after[i].Audience, "beat %d audience", i)
	}
	s.Equal(
		[]string{"cast", "saved", "damage-applied", "condition-applied"},
		s.beatNames(after),
	)
}

// TestItNarratesWhileATurnIsPaused is the ruling this file makes explicit.
// RecordCast is a RECORD verb: it appends story and moves no clock, exactly as
// RecordActivation and RecordRollWindow do. A cast narrated while a driven
// monster waits mid-walk is the post-roll case, not a malfunction — and only
// the verbs that advance the clock refuse a pause.
func (s *RecordCastSuite) TestItNarratesWhileATurnIsPaused() {
	inner := &PauseTestSuite{}
	inner.SetT(s.T())
	enc := inner.walkingScene(&pausingMover{pauseAt: map[int]bool{1: true}}, &downList{})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(enc.Paused(), "the fixture must actually be holding a paused walk")

	out, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: encounter.MemberID(alice), Target: encounter.MemberID(goblin), Spell: viciousMockery,
		Save: &encounter.CastSave{
			Saver: encounter.MemberID(goblin), Ability: "wisdom", Roll: 4, Total: 5, DC: 13,
		},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 2)

	s.True(enc.Paused(), "narrating a cast neither resumes nor clears the paused walk")
	s.Equal(encounter.MemberID(goblin), enc.PausedMember(), "the walking monster still holds the slot")
}

// TestItRefusesWhatItCannotNarrate — fail closed, every way in.
func (s *RecordCastSuite) TestItRefusesWhatItCannotNarrate() {
	enc := s.scene(everyoneStanding{})

	_, err := enc.RecordCast(nil)
	s.Require().ErrorIs(err, encounter.ErrNilInput)

	_, err = enc.RecordCast(&encounter.RecordCastInput{Spell: viciousMockery})
	s.Require().ErrorIs(err, encounter.ErrNoMember)

	_, err = enc.RecordCast(&encounter.RecordCastInput{Actor: "nobody", Spell: viciousMockery})
	s.Require().ErrorIs(err, encounter.ErrNoMember)

	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: "nobody", Spell: viciousMockery,
	})
	s.Require().ErrorIs(err, encounter.ErrNoMember)

	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Spell: encounter.SpellIdentity{Name: "Vicious Mockery"},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)

	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Spell: encounter.SpellIdentity{Ref: viciousMockery.Ref},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)

	for _, tc := range []struct {
		name string
		save encounter.CastSave
		want error
	}{
		{"no saver", encounter.CastSave{Ability: "wisdom", Roll: 6, DC: 13}, encounter.ErrNoMember},
		{
			"unknown saver",
			encounter.CastSave{Saver: "nobody", Ability: "wisdom", Roll: 6, DC: 13},
			encounter.ErrNoMember,
		},
		{
			"no ability",
			encounter.CastSave{Saver: castSkeleton, Roll: 6, DC: 13},
			encounter.ErrInvalidData,
		},
		{
			"roll is not a d20",
			encounter.CastSave{Saver: castSkeleton, Ability: "wisdom", Roll: 21, DC: 13},
			encounter.ErrInvalidData,
		},
		{
			"no dc",
			encounter.CastSave{Saver: castSkeleton, Ability: "wisdom", Roll: 6},
			encounter.ErrInvalidData,
		},
	} {
		save := tc.save
		_, saveErr := enc.RecordCast(&encounter.RecordCastInput{
			Actor: castBard, Target: castSkeleton, Spell: viciousMockery, Save: &save,
		})
		s.Require().ErrorIs(saveErr, tc.want, tc.name)
	}
}

// TestNothingLandsWhenAnythingIsRefused is the transaction boundary: a bad
// second result must not leave the cast and the save in the story.
func (s *RecordCastSuite) TestNothingLandsWhenAnythingIsRefused() {
	enc := s.scene(everyoneStanding{})

	storyBefore, err := enc.Story(&encounter.StoryInput{Audience: castBard})
	s.Require().NoError(err)

	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: castBard, Target: castSkeleton, Spell: viciousMockery, Save: failedSave(),
		Results: []encounter.ActivationResult{
			psychicDamage(),
			{Kind: encounter.ResultConditionApplied, Target: "nobody", Ref: "r", Name: "n"},
		},
	})
	s.Require().ErrorIs(err, encounter.ErrNoMember)

	storyAfter, err := enc.Story(&encounter.StoryInput{Audience: castBard})
	s.Require().NoError(err)
	s.Len(storyAfter, len(storyBefore), "a refused cast appends nothing at all")
}

// TestRecordCastClosedShapes keeps the cast's carriers limited to primitives,
// as the activation's are, rather than importing or embedding root D&D types.
func (s *RecordCastSuite) TestRecordCastClosedShapes() {
	s.Equal([]string{"Ref", "Name"}, structFieldNames(encounter.SpellIdentity{}))
	s.Equal(
		[]string{"Saver", "Ability", "Roll", "Total", "DC", "Succeeded"},
		structFieldNames(encounter.CastSave{}),
	)
	s.Equal(
		[]string{"Actor", "Target", "Spell", "Save", "Results", "ConcentrationBreaks"},
		structFieldNames(encounter.RecordCastInput{}),
	)
	s.Equal(
		[]string{"Caster", "Spell", "Reason", "Save", "Removed"},
		structFieldNames(encounter.ConcentrationBreak{}),
	)
	s.Equal([]string{"Seqs", "IntelDeltas"}, structFieldNames(encounter.RecordCastOutput{}))
}

// TestAClosedEncounterRecordsNothing — the door every record verb keeps.
func (s *RecordCastSuite) TestAClosedEncounterRecordsNothing() {
	enc := s.scene(everyoneStanding{})
	_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
	s.Require().NoError(err)

	_, err = enc.RecordCast(viciousMockeryCast())
	s.Require().ErrorIs(err, encounter.ErrClosed)
}
