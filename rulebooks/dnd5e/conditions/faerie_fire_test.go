package conditions_test

import (
	"context"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
	"testing"
)

type FaerieFireSuite struct{ suite.Suite }

func TestFaerieFireSuite(t *testing.T) { suite.Run(t, new(FaerieFireSuite)) }

type faerieSight struct{ visible, known bool }

func (v *faerieSight) SeesWithin(_, _ string, _ int) (bool, bool) { return v.visible, v.known }

func (s *FaerieFireSuite) TestRepeatedAttacksRespectLiveSightAfterReloadAndRemoval() {
	c, err := conditions.NewFaerieFireCondition(conditions.NewFaerieFireConditionInput{
		MemberID: "target", SourceID: "caster", SourceRef: refs.Spells.FaerieFire(),
	})
	s.Require().NoError(err)
	blob, err := c.ToJSON()
	s.Require().NoError(err)
	loaded, err := conditions.LoadJSON(blob)
	s.Require().NoError(err)
	c = loaded.(*conditions.FaerieFireCondition)
	s.Equal("caster", c.ConditionAddress().SourceID)
	sight := &faerieSight{true, true}
	ctx := gamectx.WithVisibility(context.Background(), sight)
	bus := events.NewEventBus()
	s.Require().NoError(c.Apply(ctx, bus))
	attack := func(ctx context.Context, target string) int {
		event := dnd5eEvents.AttackChainEvent{AttackerID: "attacker", TargetID: target}
		ch := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
		modified, err := dnd5eEvents.AttackChain.On(bus).PublishWithChain(ctx, event, ch)
		s.Require().NoError(err)
		result, err := modified.Execute(ctx, event)
		s.Require().NoError(err)
		return len(result.AdvantageSources)
	}
	s.Equal(1, attack(ctx, "target"))
	s.Equal(1, attack(ctx, "target"), "attacks do not consume Faerie Fire")
	s.Equal(0, attack(ctx, "other"))
	sight.visible = false
	s.Equal(0, attack(ctx, "target"), "fog suppresses the benefit without removing the condition")
	sight.visible = true
	s.Equal(1, attack(ctx, "target"), "benefit resumes when sight returns")
	sight.known = false
	s.Equal(0, attack(ctx, "target"), "unknown sight cannot establish eligibility")
	s.Equal(0, attack(context.Background(), "target"))
	sight.known = true
	s.Require().NoError(c.Remove(ctx, bus))
	s.Equal(0, attack(ctx, "target"), "concentration teardown revokes the benefit")
}
