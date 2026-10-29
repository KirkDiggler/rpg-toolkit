// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

// conditionBehavior is the rulebook's ConditionBehavior method set, copied
// here so the structural match is checked without importing a rulebook.
type conditionBehavior interface {
	Ref() *core.Ref
	IsApplied() bool
	Apply(ctx context.Context, bus events.EventBus) error
	Remove(ctx context.Context, bus events.EventBus) error
	ToJSON() (jsonv1.RawMessage, error)
}

var _ conditionBehavior = (*effects.Composed)(nil)

// rig is one composed condition on one bus.
type rig struct {
	s       *suite.Suite
	ctx     context.Context
	bus     events.EventBus
	c       *effects.Composed
	removed []removed
}

func newRig(s *suite.Suite, r machine.Recipe, ov effects.OutputVocabulary) *rig {
	source := &core.Ref{Module: "dnd5e", Type: "features", ID: "rage"}
	c, err := effects.Compose(r, rageVocabulary(), ov, holder, source, nil, "")
	s.Require().NoError(err)
	rg := &rig{s: s, ctx: context.Background(), bus: events.NewEventBus(), c: c}
	_, err = removedTopic.On(rg.bus).Subscribe(rg.ctx, func(_ context.Context, e removed) error {
		rg.removed = append(rg.removed, e)
		return nil
	})
	s.Require().NoError(err)
	s.Require().NoError(c.Apply(rg.ctx, rg.bus))
	return rg
}

const base = 10

func (r *rig) swing() int {
	total, err := damage(r.ctx, r.bus, testDamage{AttackerID: holder, Frame: strMelee(9), Total: base})
	r.s.Require().NoError(err)
	return total
}

func (r *rig) turnEnd(round int) {
	r.s.Require().NoError(turnEndTopic.On(r.bus).Publish(r.ctx, turnEnd{SubjectID: holder, Round: round}))
}

// freshless is Rage with the fresh state's outputs removed, so the three
// states differ: none, four, none.
func freshless() machine.Recipe {
	r := rageRecipe()
	st := r.States[stFresh]
	st.Outputs = nil
	r.States[stFresh] = st
	return r
}

type attachedSuite struct {
	suite.Suite
}

func (s *attachedSuite) TestBonusOnlyWhileInState() {
	rg := newRig(&s.Suite, freshless(), outputVocabulary())
	s.Equal(base, rg.swing(), "fresh carries no outputs here")

	rg.turnEnd(1)
	s.Equal(stRaging, rg.c.State())
	s.Equal(base+3, rg.swing(), "raging: the level-9 bonus")

	s.Require().NoError(restTopic.On(rg.bus).Publish(rg.ctx, rest{CharacterID: holder}))
	s.Equal(stEnded, rg.c.State())
	s.Equal(base, rg.swing(), "ended: detached")
	s.False(rg.c.IsApplied())
}

// The design's recipe puts the outputs on fresh as well: Rage's bonuses hold
// from activation, before the first turn end.
func (s *attachedSuite) TestTheDesignRecipeBonusesFromActivation() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	s.Equal(base+3, rg.swing(), "fresh")
	rg.turnEnd(1)
	s.Equal(base+3, rg.swing(), "raging, re-attached once, not twice")
	rg.turnEnd(2) // no activity
	s.Equal(stEnded, rg.c.State())
	s.Equal(base, rg.swing(), "ended")
}

func (s *attachedSuite) TestTheOtherKindsAttach() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	incoming := func(damageType string) testDamage {
		return testDamage{AttackerID: otherHolder, TargetID: holder, Type: damageType, Total: base}
	}
	slash, err := damage(rg.ctx, rg.bus, incoming("slashing"))
	s.Require().NoError(err)
	s.Equal(base/2, slash, "resistance to slashing")
	fire, err := damage(rg.ctx, rg.bus, incoming("fire"))
	s.Require().NoError(err)
	s.Equal(base, fire)

	save := testRoll{RollerID: holder, On: "saving_throws", Frame: rageFrame{Ability: abilitySTR}}
	adv, err := roll(rg.ctx, rg.bus, save)
	s.Require().NoError(err)
	s.True(adv, "advantage on a Strength save")
	adv, err = roll(rg.ctx, rg.bus, testRoll{RollerID: holder, On: "ability_checks", Frame: rageFrame{Ability: "dex"}})
	s.Require().NoError(err)
	s.False(adv, "no advantage on a Dexterity check")
}

func (s *attachedSuite) TestRemoveDetachesEverything() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	s.Require().NoError(rg.c.Remove(rg.ctx, rg.bus))
	s.False(rg.c.IsApplied())
	s.Equal(base, rg.swing())
	rg.turnEnd(1)
	s.Equal(stFresh, rg.c.State(), "the machine is detached too")
	s.Empty(rg.removed, "an outside Remove is not the condition ending itself")
	s.Require().NoError(rg.c.Apply(rg.ctx, rg.bus), "and it can be applied again")
	s.Equal(base+3, rg.swing())
}

func TestOutputsAttachedOnlyWhileInState(t *testing.T) {
	suite.Run(t, new(attachedSuite))
}

type endSuite struct {
	suite.Suite
}

func (s *endSuite) TestRemovalIsPublishedWhileTheOutputsStillAnswer() {
	ov := outputVocabulary()
	var totalAtEnd int
	var rg *rig
	end := ov.End
	ov.End = func(ctx context.Context, bus events.EventBus, h effects.Host, reason string) error {
		totalAtEnd = rg.swing()
		return end(ctx, bus, h, reason)
	}
	rg = newRig(&s.Suite, rageRecipe(), ov)
	rg.turnEnd(1)
	rg.turnEnd(2) // no activity

	s.Equal(base+3, totalAtEnd, "the removal hook runs before the chain stops answering")
	s.Equal(base, rg.swing(), "and afterwards the chain is clear")
	s.Equal([]removed{{Ref: "dnd5e:conditions:raging", Holder: holder, Reason: "no_combat_activity"}}, rg.removed)
}

func TestEndPublishesThenDetaches(t *testing.T) {
	suite.Run(t, new(endSuite))
}

type rowsSuite struct {
	suite.Suite
}

func (s *rowsSuite) TestRageAnswersFourRows() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	rows, err := rg.c.Rows(strMelee(9))
	s.Require().NoError(err)
	s.Equal([]effects.Row{
		{Kind: kindDamageBonus, Verdict: machine.Applies, Reason: "fact action.melee", Amount: 3},
		{Kind: kindResist, Verdict: machine.Applies, Reason: "always"},
		{Kind: kindAdvantage, Verdict: machine.Applies, Reason: "fact action.ability"},
		{Kind: kindAdvantage, Verdict: machine.Applies, Reason: "fact action.ability"},
	}, rows)
}

func (s *rowsSuite) TestTwoOutputsGiveTwoRows() {
	r := rageRecipe()
	st := r.States[stFresh]
	st.Outputs = st.Outputs[:2]
	r.States[stFresh] = st
	rg := newRig(&s.Suite, r, outputVocabulary())
	rows, err := rg.c.Rows(strMelee(1))
	s.Require().NoError(err)
	s.Len(rows, 2)
	s.Equal(2, rows[0].Amount, "level 1: +2")
}

func (s *rowsSuite) TestADependsGuardIsADependsRowNotAnError() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	frame := strMelee(9)
	frame.Ability = "" // unknown
	rows, err := rg.c.Rows(frame)
	s.Require().NoError(err)
	s.Equal(machine.Depends, rows[0].Verdict)
	s.Equal("fact action.ability", rows[0].Reason)
	s.Zero(rows[0].Amount)
}

func (s *rowsSuite) TestAnEndedConditionAnswersNoRows() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	s.Require().NoError(restTopic.On(rg.bus).Publish(rg.ctx, rest{CharacterID: holder}))
	rows, err := rg.c.Rows(strMelee(9))
	s.Require().NoError(err)
	s.Empty(rows)
}

func TestRowsSumOutputs(t *testing.T) {
	suite.Run(t, new(rowsSuite))
}

type composeSuite struct {
	suite.Suite
}

func withOutput(o jsontext.Value) machine.Recipe {
	r := rageRecipe()
	st := r.States[stRaging]
	st.Outputs = append(st.Outputs, o)
	r.States[stRaging] = st
	return r
}

func (s *composeSuite) TestRefusals() {
	five := 5
	cases := []struct {
		name   string
		recipe machine.Recipe
		params machine.Bag
		want   string
	}{
		{"an unknown kind", withOutput(output(effects.Output{Kind: "roll_bonus"})), nil,
			`kind "roll_bonus" is not in the output vocabulary`},
		{"an amount not in the vocabulary", withOutput(output(effects.Output{
			Kind: kindDamageBonus, Amount: &effects.Amount{Fixed: &five}, Params: jsontext.Value(`{"stage":"features"}`),
		})), nil, `amount "fixed" is not in the output vocabulary`},
		{"an amount with two fields", withOutput(output(effects.Output{
			Kind: kindDamageBonus, Amount: &effects.Amount{Fixed: &five, Dice: "1d4"},
			Params: jsontext.Value(`{"stage":"features"}`),
		})), nil, "amount sets 2 fields, want exactly 1"},
		{"an amount with no fields", withOutput(output(effects.Output{
			Kind: kindDamageBonus, Amount: &effects.Amount{}, Params: jsontext.Value(`{"stage":"features"}`),
		})), nil, "amount sets 0 fields, want exactly 1"},
		{"an output guard over an undeclared flag", withOutput(output(effects.Output{
			Kind: kindAdvantage, When: &machine.Guard{Flag: "is_frenzied"}, Params: jsontext.Value(`{"on":"saving_throws"}`),
		})), nil, `flag "is_frenzied" is not declared in the bag`},
		{"an output guard over an unknown fact", withOutput(output(effects.Output{
			Kind:   kindAdvantage,
			When:   &machine.Guard{Fact: &machine.FactLeaf{Name: "action.finesse", Op: machine.OpEq, Value: true}},
			Params: jsontext.Value(`{"on":"saving_throws"}`),
		})), nil, `fact "action.finesse" is not in the vocabulary`},
		{"params the kind refuses", withOutput(output(effects.Output{
			Kind: kindAdvantage, Params: jsontext.Value(`{"on":"saving_throws","twice":true}`),
		})), nil, `kind "advantage" refused its params`},
		{"an output that is not an object", withOutput(jsontext.Value(`[1]`)), nil, "output 4"},
		{"a parameter naming no bag key", rageRecipe(), machine.Bag{"counterpart_id": otherHolder},
			`parameter "counterpart_id" names no bag key`},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := effects.Compose(tc.recipe, rageVocabulary(), outputVocabulary(), holder, nil, tc.params, "")
			s.Require().Error(err)
			s.Contains(err.Error(), tc.want)
		})
	}
}

// A bag key only an output's guard reads is accepted: the composer names it
// to the machine's Validate.
func (s *composeSuite) TestAKeyOnlyAnOutputReadsIsRead() {
	r := withOutput(output(effects.Output{
		Kind: kindAdvantage, When: &machine.Guard{Flag: "is_frenzied"}, Params: jsontext.Value(`{"on":"saving_throws"}`),
	}))
	r.Bag["is_frenzied"] = false
	_, err := effects.Compose(r, rageVocabulary(), outputVocabulary(), holder, nil, nil, "")
	s.Require().NoError(err)
}

func (s *composeSuite) TestToJSONIsTheFlatBlobAndLoadsBack() {
	rg := newRig(&s.Suite, rageRecipe(), outputVocabulary())
	rg.turnEnd(3)
	data, err := rg.c.ToJSON()
	s.Require().NoError(err)
	s.JSONEq(`{"ref":"dnd5e:conditions:raging","member_id":"barbarian-1","source":"dnd5e:features:rage",`+
		`"state":"raging","saw_turn_end":true,"round_activated":3,"turns_active":0,`+
		`"was_hit_this_turn":false,"did_attack_this_turn":false}`, string(data))

	var b machine.Blob
	s.Require().NoError(json.Unmarshal(data, &b))
	loaded, err := effects.Compose(rageRecipe(), rageVocabulary(), outputVocabulary(),
		b.MemberID, b.Source, b.Bag, b.State)
	s.Require().NoError(err)
	again, err := loaded.ToJSON()
	s.Require().NoError(err)
	s.Equal(string(data), string(again), "byte-equal after a load")
	s.Equal(&ragingRef, loaded.Ref())
}

func TestComposeRefuses(t *testing.T) {
	suite.Run(t, new(composeSuite))
}
