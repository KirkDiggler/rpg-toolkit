// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

type typedCompositionSuite struct{ suite.Suite }

func TestTypedCompositionSuite(t *testing.T) { suite.Run(t, new(typedCompositionSuite)) }
func (s *typedCompositionSuite) world() *definitionWorld {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	return w
}
func (s *typedCompositionSuite) grant(w *definitionWorld, definitions *conditions.CompiledDefinitions, id, user, target, application string) *fixtureSource {
	source := newFixtureSource("source-"+application, user, id)
	grant, err := useDefinition(context.Background(), definitionUseInput{Source: source, RecipientID: target, ApplicationID: application, Definitions: definitions, Participants: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	s.Require().NoError(w.apply(grant.Prepared))
	return source
}

func (s *typedCompositionSuite) TestRingKeepsIdleRecipient() {
	defs, err := typedRagingRing()
	s.Require().NoError(err)
	w := s.world()
	source := s.grant(w, defs, "raging-ring", "A", "B", "ring")
	for range 10 {
		s.Require().NoError(w.turnEnd("B"))
	}
	s.Require().NoError(w.reload())
	owner, err := w.coordinator("ring")
	s.Require().NoError(err)
	s.Equal("A", owner)
	info, err := w.maintenance("A", "ring")
	s.Require().NoError(err)
	s.NotNil(info.Concentration)
	s.Nil(info.Duration)
	outgoing, err := w.damageBonus("B", "C", abilities.STR, true, true)
	s.Require().NoError(err)
	s.Equal(12, totalFixtureDamage(outgoing))
	s.Equal(source.ID, outgoing.Components[1].Roll.Source.SourceID)
	incoming, err := w.hit("B", damage.Slashing, 9)
	s.Require().NoError(err)
	s.Equal(4, incoming.Amount)
	s.Empty(incoming.FollowUps)
	save, err := w.saveAdvantage("B", abilities.STR)
	s.Require().NoError(err)
	s.Equal(1, save)
	check, err := w.checkAdvantage("B", skills.Athletics)
	s.Require().NoError(err)
	s.Equal(1, check)
	otherSave, err := w.saveAdvantage("B", abilities.DEX)
	s.Require().NoError(err)
	s.Zero(otherSave)
	s.Zero(source.Budget.Current)
}

func (s *typedCompositionSuite) TestConcentrationFailureEndsOnlyRing() {
	defs, err := typedRagingRing()
	s.Require().NoError(err)
	w := s.world()
	s.grant(w, defs, "raging-ring", "A", "B", "ring")
	bonus := 2
	raw, err := json.Marshal(conditions.RagingBindings{DamageBonus: conditions.RagingDamageBonus{Fixed: &bonus}})
	s.Require().NoError(err)
	native, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: "B", Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "B"}, Config: raw})
	s.Require().NoError(err)
	// A condition fixture, not a claim of native Barbarian feature acquisition.
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(w.bus).Publish(context.Background(), dndevents.ConditionAppliedEvent{Target: w.holder("B"), Condition: native.Condition}))
	taken, err := w.hit("A", damage.Fire, 6)
	s.Require().NoError(err)
	s.Require().Len(taken.FollowUps, 1)
	failure := taken.FollowUps[0].OnFailure
	s.Require().NoError(w.remove(failure.Owner, failure.Owner.SourceID, failure.Reason)) // test-selected failure of the actual follow-up
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().Len(w.holder("B").GetConditions(), 1)
	s.Equal(refs.Conditions.Raging().String(), w.holder("B").GetConditions()[0].Ref().String())
	result, err := w.damageBonus("B", "C", abilities.STR, true, true)
	s.Require().NoError(err)
	s.Equal(12, totalFixtureDamage(result))
	s.Equal("B", result.Components[1].Roll.Source.SourceID, "the remaining native effect retains its own origin")
}

func (s *typedCompositionSuite) TestSingleBenefitNeedsNoRageGrant() {
	compiled, err := conditions.CompileComposition(&conditions.CompileCompositionInput{ID: "ward", Spec: conditions.CompositionSpec{
		Name: "Ward", Coordinator: conditions.DefinitionRecipient,
		Benefits:    []effects.PartSpec{{Key: "resist", Effect: effects.Resistance{Types: []damage.Type{damage.Slashing}}, OnTerminal: generic.EndPart}},
		Maintenance: []effects.PartSpec{{Key: "rest", Effect: effects.EndOn{Beat: effects.BeatRest, Reason: "rest"}, OnTerminal: generic.EndApplication}},
	}})
	s.Require().NoError(err)
	w := s.world()
	s.grant(w, compiled.Definitions, "ward", "A", "B", "ward")
	result, err := w.damageBonus("B", "C", abilities.STR, true, true)
	s.Require().NoError(err)
	s.Equal(10, totalFixtureDamage(result))
	incoming, err := w.incoming("B", damage.Slashing, 9)
	s.Require().NoError(err)
	s.Equal(4, incoming.Types[0].Taken)
	save, err := w.saveAdvantage("B", abilities.STR)
	s.Require().NoError(err)
	s.Zero(save)
	s.Require().NoError(w.rest("B"))
	s.Empty(w.holder("B").GetConditions())
}

func (s *typedCompositionSuite) TestDifferentMaintenanceCannotRewriteActiveGrant() {
	original, err := typedRagingRing()
	s.Require().NoError(err)
	w := s.world()
	s.grant(w, original, "raging-ring", "A", "B", "first")
	bonus := 2
	benefits, err := conditions.RagingBenefits(&conditions.RagingBenefitsInput{DamageBonus: conditions.RagingDamageBonus{Fixed: &bonus}})
	s.Require().NoError(err)
	changed, err := conditions.CompileComposition(&conditions.CompileCompositionInput{ID: "raging-ring", Spec: conditions.CompositionSpec{
		Name: "Next-turn rage", Coordinator: conditions.DefinitionRecipient, Benefits: benefits.Parts,
		Maintenance: []effects.PartSpec{{Key: "next", Effect: effects.EndOn{Beat: effects.BeatTurnStart, Reason: "turn_started"}, OnTerminal: generic.EndApplication}},
	}})
	s.Require().NoError(err)
	s.grant(w, changed.Definitions, "raging-ring", "C", "C", "second")
	s.Require().NoError(w.reload())
	s.Require().NoError(w.turnStart("C"))
	s.Empty(w.holder("C").GetConditions())
	s.Require().NoError(w.turnStart("B"))
	s.NotEmpty(w.holder("B").GetConditions(), "the active saved grant does not consult a latest catalogue")
	owner, err := w.coordinator("first")
	s.Require().NoError(err)
	s.Equal("A", owner)
	result, err := w.damageBonus("B", "C", abilities.STR, true, true)
	s.Require().NoError(err)
	s.Equal(12, totalFixtureDamage(result))
}

func (s *typedCompositionSuite) TestUnnamedAttackOnlyGift() {
	compiled, err := conditions.CompileComposition(&conditions.CompileCompositionInput{ID: "unplanned-gift", Spec: conditions.CompositionSpec{
		Name: "An independently authored gift", Coordinator: conditions.DefinitionRecipient,
		Benefits:    []effects.PartSpec{{Key: "roll", Effect: effects.AddToRoll{Amount: effects.SignedDice{Notation: "1d6", Polarity: effects.Positive}, Rolls: []contributions.RollKind{contributions.RollKindAttack}, Selection: generic.SelectionRule{Mode: generic.AllApplicable}}, OnTerminal: generic.EndPart}},
		Maintenance: []effects.PartSpec{{Key: "next", Effect: effects.EndOn{Beat: effects.BeatTurnStart, Reason: "turn_started"}, OnTerminal: generic.EndApplication}},
	}})
	s.Require().NoError(err)
	w := s.world()
	s.grant(w, compiled.Definitions, "unplanned-gift", "A", "B", "gift")
	attack, err := w.holder("B").DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: contributions.RollKindAttack})
	s.Require().NoError(err)
	s.Require().Len(attack.Contributions, 1)
	s.Equal("1d6", attack.Contributions[0].Dice)
	save, err := w.holder("B").DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: contributions.RollKindSavingThrow})
	s.Require().NoError(err)
	s.Empty(save.Contributions)
	s.Require().NoError(w.rest("B"))
	s.NotEmpty(w.holder("B").GetConditions(), "rest was not selected as an ending")
	s.Require().NoError(w.turnStart("B"))
	s.Empty(w.holder("B").GetConditions())
}
