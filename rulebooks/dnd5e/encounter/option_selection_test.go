// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// OptionSelectionSuite is Gate S1 of rpg-project#769 (sub-issue #921): the
// lines a speaker is OFFERED and the line [encounter.Encounter.Speak]'s door
// ADMITS are one judgment, so the two can never disagree. A gated line is
// absent from the offer, never greyed (the probe law), and the door refuses
// exactly what the offer left out, with [encounter.ErrNoOption] and without
// writing a beat or ending a sneak.
type OptionSelectionSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *OptionSelectionSuite) SetupTest() { s.ctx = context.Background() }

func TestOptionSelectionSuite(t *testing.T) { suite.Run(t, new(OptionSelectionSuite)) }

// plainSelectionLine is a line every speaker is offered.
func plainSelectionLine() encounter.Option {
	return encounter.Option{
		ID: "plain", Say: "Evening.",
		Success: []encounter.Answer{{Weight: 1, Say: "Evening yourself."}},
	}
}

// gatedSelectionLine is a plain line offered only while requires holds.
func gatedSelectionLine(id string, requires ...encounter.Trigger) encounter.Option {
	return encounter.Option{
		ID: id, Say: "I know what you did.", Requires: requires,
		Success: []encounter.Answer{{Weight: 1, Say: "…keep your voice down."}},
	}
}

// selectionScene is the option suite's room — alice and the goblin in sight
// of each other, the altar beside alice (examining it teaches the cultist
// fact), billy behind a wall row seeing none of it — with the goblin's lines
// and the purse's lines named.
func (s *OptionSelectionSuite) selectionScene(goblinLines, purseLines []encounter.Option) *encounter.Encounter {
	altar := encounter.PropInput{
		ID: theAltar, Ref: "test:props:altar", At: spatial.Position{X: 7, Y: 2},
		BlocksMovement: no(), BlocksLineOfSight: no(), Options: altarOptions(),
	}
	purse := holdableProp(thePurse, "test:props:purse", spatial.Position{X: 5, Y: 2})
	purse.Options = purseLines
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12)},
			Props:   append(wallRow(6, 4, 8), purse, altar),
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}, Options: goblinLines},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: (&OptionTestSuite{}).capabilities(),
	})
	s.Require().NoError(err)

	return enc
}

// sayTo speaks a plain line from alice to the goblin.
func (s *OptionSelectionSuite) sayTo(enc *encounter.Encounter, option string) (*encounter.SpeakOutput, error) {
	return enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: speakReach, Actor: alice, Target: goblin, OptionID: option, Roller: rollsLowest{},
	})
}

// teachAlice has alice read the altar, so she learns the cultist fact.
func (s *OptionSelectionSuite) teachAlice(enc *encounter.Encounter) {
	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: speakReach, Actor: alice, Prop: theAltar, OptionID: "examine", Skill: "religion",
		Beaten: true, DC: 12, Total: 16, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
}

// TestOfferAndDoorShareOneJudgment: a line gated on a fact is absent from the
// offer and refused at the door while the speaker lacks the fact — with no
// beat written — and both open the moment the speaker learns it.
func (s *OptionSelectionSuite) TestOfferAndDoorShareOneJudgment() {
	enc := s.selectionScene([]encounter.Option{
		plainSelectionLine(),
		gatedSelectionLine("secret", encounter.TriggerFact{Fact: cultistFact}),
	}, nil)

	offered, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"plain"}, ids(offered), "the secret waits on the cultist fact")

	before := enc.ToData().Log.NextSeq
	_, err = s.sayTo(enc, "secret")
	s.Require().ErrorIs(err, encounter.ErrNoOption, "the door refuses what the offer left out")
	s.Equal(before, enc.ToData().Log.NextSeq, "a refusal writes nothing")

	s.teachAlice(enc)

	offered, err = enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"plain", "secret"}, ids(offered), "alice read the sigils")

	out, err := s.sayTo(enc, "secret")
	s.Require().NoError(err, "the door admits what the offer shows")
	s.True(out.Beaten)
	reader := &OptionTestSuite{}
	reader.SetT(s.T())
	spoke := reader.beatsOfKind(enc, alice, encounter.BeatSpoke)
	s.Require().NotEmpty(spoke)
	s.Equal("secret", spoke[len(spoke)-1]["option"], "the spoke beat names the line")
}

// TestNotRequirementStaysInverted: a `not` line is offered to whoever lacks
// the fact and withheld — at the offer and at the door — from whoever knows
// it, judged per speaker (R9).
func (s *OptionSelectionSuite) TestNotRequirementStaysInverted() {
	enc := s.selectionScene([]encounter.Option{
		plainSelectionLine(),
		gatedSelectionLine("innocent", encounter.TriggerNot{Inner: encounter.TriggerFact{Fact: cultistFact}}),
	}, nil)

	offered, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"plain", "innocent"}, ids(offered), "alice knows nothing yet")

	s.teachAlice(enc)

	offered, err = enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"plain"}, ids(offered), "alice knows now")
	_, err = s.sayTo(enc, "innocent")
	s.ErrorIs(err, encounter.ErrNoOption, "and the door agrees")

	billys, err := enc.OptionsFor(billy, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"plain", "innocent"}, ids(billys), "billy saw nothing and is still offered it")
}

// TestPropLinesUseTheSameDoor: a prop's line gated on holding that prop is
// absent and refused while the speaker does not carry it, and offered and
// admitted once they do.
func (s *OptionSelectionSuite) TestPropLinesUseTheSameDoor() {
	enc := s.selectionScene([]encounter.Option{plainSelectionLine()}, []encounter.Option{
		gatedSelectionLine("count", encounter.TriggerHolds{Prop: thePurse}),
	})
	speakToPurse := func() (*encounter.SpeakOutput, error) {
		return enc.Speak(s.ctx, &encounter.SpeakInput{
			Range: speakReach, Actor: alice, Prop: thePurse, OptionID: "count", Roller: rollsLowest{},
		})
	}

	offered, err := enc.PropOptionsFor(alice, thePurse)
	s.Require().NoError(err)
	s.Empty(offered, "alice is not carrying the purse")
	before := enc.ToData().Log.NextSeq
	_, err = speakToPurse()
	s.Require().ErrorIs(err, encounter.ErrNoOption)
	s.Equal(before, enc.ToData().Log.NextSeq, "a refusal writes nothing")

	_, err = enc.Hold(&encounter.HoldInput{Member: alice, Target: thePurse})
	s.Require().NoError(err)

	offered, err = enc.PropOptionsFor(alice, thePurse)
	s.Require().NoError(err)
	s.Equal([]string{"count"}, ids(offered), "alice carries the purse")
	_, err = speakToPurse()
	s.NoError(err, "and the door admits it")
}

// TestSneakingIsRestoredOnRefusal: a line the speaker is not offered is
// refused after the mode was ended for the audience, and the refusal puts
// the mode and its verdicts back (sneaking R5): a refused verb changes
// nothing.
func (s *OptionSelectionSuite) TestSneakingIsRestoredOnRefusal() {
	scene := &SneakSuite{}
	scene.SetT(s.T())
	scene.open(sneakOptions{knightOptions: []encounter.Option{
		gatedSelectionLine("secret", encounter.TriggerFact{Fact: cultistFact}),
	}}, true)
	scene.sneak(true)
	verdicts := scene.enc.ToData().SneakVerdicts
	before := scene.enc.ToData().Log.NextSeq

	_, err := scene.enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: 20, Actor: sneakRogue, Target: sneakKnight, OptionID: "secret", Roller: rollsLowest{},
	})
	s.Require().ErrorIs(err, encounter.ErrNoOption, "the rogue does not know the fact")
	s.True(scene.enc.Sneaking(sneakRogue), "the refusal put the mode back")
	s.Equal(verdicts, scene.enc.ToData().SneakVerdicts, "and the verdicts with it")
	s.Equal(before, scene.enc.ToData().Log.NextSeq, "and wrote nothing")
}

// TestEveryRequirementMustHold: a line with two requirements (rpg-project#922
// Task 1) is one row with two named verdicts, and it is offered only when
// BOTH hold — absent and refused at the door while either is Unmet, with no
// beat written, and offered and admitted once both are Met.
func (s *OptionSelectionSuite) TestEveryRequirementMustHold() {
	enc := s.selectionScene([]encounter.Option{
		plainSelectionLine(),
		gatedSelectionLine("both", encounter.TriggerFact{Fact: cultistFact}, encounter.TriggerHolds{Prop: thePurse}),
	}, nil)

	offered, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"plain"}, ids(offered), "neither requirement holds")

	s.teachAlice(enc)

	offered, err = enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"plain"}, ids(offered), "the fact holds and the holding does not: one Unmet is absent")
	before := enc.ToData().Log.NextSeq
	_, err = s.sayTo(enc, "both")
	s.Require().ErrorIs(err, encounter.ErrNoOption, "and the door agrees")
	s.Equal(before, enc.ToData().Log.NextSeq, "a refusal writes nothing")

	_, err = enc.Hold(&encounter.HoldInput{Member: alice, Target: thePurse})
	s.Require().NoError(err)

	offered, err = enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"plain", "both"}, ids(offered), "both hold")
	_, err = s.sayTo(enc, "both")
	s.NoError(err, "and the door admits it")
}
