package conditions_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/assessment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type ragingAssessmentSuite struct{ suite.Suite }

func TestRagingAssessmentSuite(t *testing.T) { suite.Run(t, new(ragingAssessmentSuite)) }

func (s *ragingAssessmentSuite) input() *assessment.AssessDamageInput {
	return &assessment.AssessDamageInput{Frame: assessment.DamageFrame{
		ActorID: "actor", Ability: contributions.Known(abilities.STR),
		Melee: contributions.Known(true), HasWeaponPool: contributions.Known(true),
	}}
}

func (s *ragingAssessmentSuite) TestDetachedSnapshotAndReadOnlyAmount() {
	condition := &conditions.RagingCondition{CharacterID: "actor", DamageBonus: 2}
	before, err := condition.ToJSON()
	s.Require().NoError(err)
	bound, err := condition.AssessmentSnapshot(&assessment.BindInput{ID: "effect/0", OwnerID: "actor", Order: 3})
	s.Require().NoError(err)
	s.Equal("effect/0", bound.Binding.ID)
	s.Equal(3, bound.Binding.Order)
	s.Require().NotNil(bound.Binding.Damage)
	s.False(condition.IsApplied())
	for range 2 {
		out, assessErr := bound.Binding.Damage.AssessDamage(s.input())
		s.Require().NoError(assessErr)
		s.Require().NoError(out.Validate())
		s.Equal(contributions.Applies, out.Decision.Applicability)
		s.Require().Len(out.Changes, 1)
		s.Equal(2, *out.Changes[0].Fixed)
		s.Equal(assessment.PrimaryWeaponPool, out.Changes[0].PoolID)
		s.False(out.Changes[0].DoubleDiceOnCritical)
		*out.Changes[0].Fixed = 99
		out.Changes[0].Source.Ref.ID = "changed"
	}
	after, err := condition.ToJSON()
	s.Require().NoError(err)
	s.JSONEq(string(before), string(after))
	condition.DamageBonus = 4
	condition.CharacterID = "changed"
	bound.Binding.Source.Ref.ID = "also-changed"
	out, err := bound.Binding.Damage.AssessDamage(s.input())
	s.Require().NoError(err)
	s.Equal(2, *out.Changes[0].Fixed)
	s.Equal("raging", out.Changes[0].Source.Ref.ID)
}

func (s *ragingAssessmentSuite) TestKnownNegativesAndUnknownAreDifferent() {
	condition := &conditions.RagingCondition{CharacterID: "actor", DamageBonus: 2}
	bound, err := condition.AssessmentSnapshot(&assessment.BindInput{ID: "effect/0", OwnerID: "actor"})
	s.Require().NoError(err)
	for _, mutate := range []func(*assessment.AssessDamageInput){
		func(in *assessment.AssessDamageInput) { in.Frame.Ability = contributions.Known(abilities.DEX) },
		func(in *assessment.AssessDamageInput) { in.Frame.Melee = contributions.Known(false) },
		func(in *assessment.AssessDamageInput) { in.Frame.HasWeaponPool = contributions.Known(false) },
		func(in *assessment.AssessDamageInput) { in.Frame.ActorID = "other" },
	} {
		in := s.input()
		mutate(in)
		out, assessErr := bound.Binding.Damage.AssessDamage(in)
		s.Require().NoError(assessErr)
		s.Require().NoError(out.Validate())
		s.Equal(contributions.DoesNotApply, out.Decision.Applicability)
		s.Empty(out.Changes)
	}
	in := s.input()
	in.Frame.Ability = contributions.Unknown[abilities.Ability]()
	out, err := bound.Binding.Damage.AssessDamage(in)
	s.Require().NoError(err)
	s.Require().NoError(out.Validate())
	s.Equal(contributions.NeedsContext, out.Decision.Applicability)
	s.Empty(out.Changes)
	in.Frame.Melee = contributions.Known(false)
	out, err = bound.Binding.Damage.AssessDamage(in)
	s.Require().NoError(err)
	s.Equal(contributions.DoesNotApply, out.Decision.Applicability, "decisive known negative wins over irrelevant unknown ability")
}

func (s *ragingAssessmentSuite) TestSameAnswerDrivesTheRealDamageChain() {
	for _, tc := range []struct {
		name    string
		ability abilities.Ability
		melee   bool
		bonus   int
	}{
		{"strength", abilities.STR, true, 2},
		{"changed bonus", abilities.STR, true, 4},
		{"real zero", abilities.STR, true, 0},
		{"dexterity", abilities.DEX, true, 2},
		{"ranged", abilities.STR, false, 2},
	} {
		s.Run(tc.name, func() {
			ctx := context.Background()
			bus := events.NewEventBus()
			condition := &conditions.RagingCondition{CharacterID: "actor", DamageBonus: tc.bonus}
			bound, err := condition.AssessmentSnapshot(&assessment.BindInput{ID: "effect/0", OwnerID: "actor"})
			s.Require().NoError(err)
			in := s.input()
			in.Frame.Ability = contributions.Known(tc.ability)
			in.Frame.Melee = contributions.Known(tc.melee)
			answer, err := bound.Binding.Damage.AssessDamage(in)
			s.Require().NoError(err)
			s.Require().NoError(answer.Validate())
			s.Require().NoError(condition.Apply(ctx, bus))
			event := &dndevents.DamageChainEvent{
				AttackerID: "actor", TargetID: "target", AbilityUsed: tc.ability, IsMelee: tc.melee,
				WeaponDamageType: damage.Slashing,
				Components: []dndevents.DamageComponent{{Source: dndevents.DamageSourceWeapon,
					Properties: []damage.Property{damage.AddsAttackAbilityModifier}}},
			}
			chain := events.NewStagedChain[*dndevents.DamageChainEvent](combat.ModifierStages)
			fold, err := dndevents.DamageChain.On(bus).PublishWithChain(ctx, event, chain)
			s.Require().NoError(err)
			result, err := fold.Execute(ctx, event)
			s.Require().NoError(err)
			s.Require().Len(result.Components, 1+len(answer.Changes))
			if answer.Decision.Applicability == contributions.Applies {
				s.Require().Len(answer.Changes, 1)
				s.Equal(answer.Changes[0].Source, result.Components[1].Roll.Source)
				s.Equal(*answer.Changes[0].Fixed, *result.Components[1].Roll.Modifier)
			}
			s.False(condition.DidAttackThisTurn, "damage assessment does not sustain Rage")
			s.Require().NoError(condition.Remove(ctx, bus))
		})
	}
}

func (s *ragingAssessmentSuite) TestInputErrorsAndCoverage() {
	condition := &conditions.RagingCondition{CharacterID: "actor", DamageBonus: 2}
	for _, in := range []*assessment.BindInput{nil, {}, {ID: "id", OwnerID: "other"}, {ID: "id", OwnerID: "actor", Order: -1}} {
		out, err := condition.AssessmentSnapshot(in)
		s.Error(err)
		s.Nil(out)
	}
	bound, err := condition.AssessmentSnapshot(&assessment.BindInput{ID: "id", OwnerID: "actor"})
	s.Require().NoError(err)
	s.Require().Len(bound.Binding.Coverage, 5)
	s.Equal(assessment.Supported, bound.Binding.Coverage[1].Support)
	out, err := bound.Binding.Damage.AssessDamage(nil)
	s.Error(err)
	s.Nil(out)
}
