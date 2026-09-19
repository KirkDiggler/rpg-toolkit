package conditions_test

import (
	"context"
	"errors"
	mockdice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	de "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"testing"
)

type DivineFavorSuite struct{ suite.Suite }

func TestDivineFavorSuite(t *testing.T) { suite.Run(t, new(DivineFavorSuite)) }

func (s *DivineFavorSuite) TestWeaponHitsReloadCriticalAndTeardown() {
	for _, tc := range []struct {
		name            string
		melee, critical bool
	}{
		{"melee", true, false}, {"ranged", false, false}, {"critical", true, true},
	} {
		s.Run(tc.name, func() {
			ctx := context.Background()
			bus := events.NewEventBus()
			made, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.DivineFavor().String(), MemberID: "caster", SourceRef: refs.Spells.DivineFavor().String()})
			s.Require().NoError(err)
			blob, err := made.Condition.ToJSON()
			s.Require().NoError(err)
			restored, err := conditions.LoadJSON(blob)
			s.Require().NoError(err)
			c := restored.(*conditions.DivineFavorCondition)
			s.Equal("caster", c.ConditionAddress().SourceID)
			roller := mockdice.NewMockRoller(gomock.NewController(s.T()))
			count := 1
			faces := []int{3}
			if tc.critical {
				count = 2
				faces = []int{3, 4}
			}
			roller.EXPECT().RollN(gomock.Any(), count, 4).Return(faces, nil).Times(2)
			c.BindRoller(roller)
			c.BindRoller(nil)
			s.Require().NoError(c.Apply(ctx, bus))
			hit := func() *de.DamageChainEvent {
				return &de.DamageChainEvent{AttackerID: "caster", TargetID: "enemy", IsMelee: tc.melee, IsCritical: tc.critical, Components: []de.DamageComponent{{Source: de.DamageSourceWeapon, DamageType: damage.Slashing, Properties: []damage.Property{damage.AddsAttackAbilityModifier}}}}
			}
			for range 2 {
				result, err := s.execute(bus, hit())
				s.Require().NoError(err)
				s.Require().Len(result.Components, 2)
				extra := result.Components[1]
				s.Equal(damage.Radiant, extra.DamageType)
				s.Equal(tc.critical, extra.IsCritical)
				s.Equal(refs.Spells.DivineFavor(), extra.Roll.Source.Ref)
				s.Equal("caster", extra.Roll.Source.SourceID)
				s.Equal(faces, extra.Roll.Dice.OriginalRolls)
				s.Equal(faces, extra.Roll.Dice.FinalRolls)
				expected := 3
				if tc.critical {
					expected = 7
				}
				s.Equal(expected, extra.Roll.Dice.Subtotal)
			}
			s.Require().NoError(c.Remove(ctx, bus))
			result, err := s.execute(bus, hit())
			s.Require().NoError(err)
			s.Len(result.Components, 1)
		})
	}
}

func (s *DivineFavorSuite) TestExcludesSpellsOtherAttackersAndDuplicateBonusesWithoutRolling() {
	ctx := context.Background()
	bus := events.NewEventBus()
	c, err := conditions.NewDivineFavorCondition(conditions.NewDivineFavorConditionInput{MemberID: "caster", SourceID: "caster", SourceRef: refs.Spells.DivineFavor()})
	s.Require().NoError(err)
	c.BindRoller(mockdice.NewMockRoller(gomock.NewController(s.T())))
	s.Require().NoError(c.Apply(ctx, bus))
	for _, event := range []*de.DamageChainEvent{
		{AttackerID: "caster", Components: []de.DamageComponent{{Source: de.DamageSourceSpell}}},
		{AttackerID: "other", Components: []de.DamageComponent{{Source: de.DamageSourceWeapon, Properties: []damage.Property{damage.AddsAttackAbilityModifier}}}},
		{AttackerID: "caster", Components: []de.DamageComponent{{Source: de.DamageSourceWeapon, Properties: []damage.Property{damage.AddsAttackAbilityModifier}}, {Source: de.DamageSourceSpell, Roll: de.RollComponent{Source: de.RollSource{Ref: refs.Spells.DivineFavor()}}}}},
	} {
		before := len(event.Components)
		result, err := s.execute(bus, event)
		s.Require().NoError(err)
		s.Len(result.Components, before)
	}
}

func (s *DivineFavorSuite) TestRollFailurePropagates() {
	ctx := context.Background()
	bus := events.NewEventBus()
	c, err := conditions.NewDivineFavorCondition(conditions.NewDivineFavorConditionInput{MemberID: "caster", SourceID: "caster", SourceRef: refs.Spells.DivineFavor()})
	s.Require().NoError(err)
	roller := mockdice.NewMockRoller(gomock.NewController(s.T()))
	failure := errors.New("dice unavailable")
	roller.EXPECT().RollN(gomock.Any(), 1, 4).Return(nil, failure)
	c.BindRoller(roller)
	s.Require().NoError(c.Apply(ctx, bus))
	_, err = s.execute(bus, &de.DamageChainEvent{AttackerID: "caster", Components: []de.DamageComponent{{Source: de.DamageSourceWeapon, Properties: []damage.Property{damage.AddsAttackAbilityModifier}}}})
	s.ErrorIs(err, failure)
}

func (s *DivineFavorSuite) execute(bus events.EventBus, event *de.DamageChainEvent) (*de.DamageChainEvent, error) {
	ctx := context.Background()
	c := events.NewStagedChain[*de.DamageChainEvent](combat.ModifierStages)
	modified, err := de.DamageChain.On(bus).PublishWithChain(ctx, event, c)
	if err != nil {
		return nil, err
	}
	return modified.Execute(ctx, event)
}
