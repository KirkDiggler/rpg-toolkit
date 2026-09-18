package conditions_test

import (
	"context"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
	"testing"
)

type ShieldOfFaithSuite struct{ suite.Suite }

func TestShieldOfFaithSuite(t *testing.T) { suite.Run(t, new(ShieldOfFaithSuite)) }

func (s *ShieldOfFaithSuite) TestProtectionSurvivesReloadDoesNotStackAndEndsIndependently() {
	ctx := context.Background()
	bus := events.NewEventBus()
	var held []*conditions.ShieldOfFaithCondition
	for _, caster := range []string{"cleric-one", "cleric-two"} {
		c, err := conditions.NewShieldOfFaithCondition(conditions.NewShieldOfFaithConditionInput{
			MemberID: "target", SourceID: caster, SourceRef: refs.Spells.ShieldOfFaith(),
		})
		s.Require().NoError(err)
		blob, err := c.ToJSON()
		s.Require().NoError(err)
		loaded, err := conditions.LoadJSON(blob)
		s.Require().NoError(err)
		c = loaded.(*conditions.ShieldOfFaithCondition)
		s.Equal(caster, c.ConditionAddress().SourceID)
		s.Require().NoError(c.Apply(ctx, bus))
		held = append(held, c)
	}
	ac := func(member string, armored bool) int {
		event := &combat.ACChainEvent{CharacterID: member, HasArmor: armored, HasShield: armored,
			Breakdown: &combat.ACBreakdown{Total: 18}}
		chain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
		modified, err := combat.ACChain.On(bus).PublishWithChain(ctx, event, chain)
		s.Require().NoError(err)
		result, err := modified.Execute(ctx, event)
		s.Require().NoError(err)
		return result.Breakdown.Total
	}
	s.Equal(20, ac("target", true), "adds to armor and a physical shield without stacking castings")
	s.Equal(20, ac("target", false), "also protects an unarmored recipient")
	s.Equal(18, ac("other", true), "does not change another creature")
	s.Require().NoError(held[0].Remove(ctx, bus))
	s.Equal(20, ac("target", true), "second caster remains active")
	s.Require().NoError(held[1].Remove(ctx, bus))
	s.Equal(18, ac("target", true))
}

func (s *ShieldOfFaithSuite) TestRequiresRecipientCasterAndCanonicalSpell() {
	for _, input := range []conditions.NewShieldOfFaithConditionInput{
		{SourceID: "caster", SourceRef: refs.Spells.ShieldOfFaith()},
		{MemberID: "target", SourceRef: refs.Spells.ShieldOfFaith()},
		{MemberID: "target", SourceID: "caster", SourceRef: refs.Spells.Bless()},
	} {
		_, err := conditions.NewShieldOfFaithCondition(input)
		s.Error(err)
	}
}
