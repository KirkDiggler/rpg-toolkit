package conditions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type FaerieFireSuite struct{ suite.Suite }

func TestFaerieFireSuite(t *testing.T) { suite.Run(t, new(FaerieFireSuite)) }

// TestRepeatedAttacksRespectFrameSightAfterReloadAndRemoval: each attack reads
// whether the attacker sees the outlined target from its own frame, so sight
// lost and regained between attacks is honoured, an unknown sight fails the
// attack, and concentration teardown revokes the benefit.
func (s *FaerieFireSuite) TestRepeatedAttacksRespectFrameSightAfterReloadAndRemoval() {
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
	held := contributions.HeldCondition{Ref: refs.Conditions.FaerieFire().String(), SourceID: "caster"}
	ctx := context.Background()
	bus := events.NewEventBus()
	s.Require().NoError(c.Apply(ctx, bus))
	publish := func(event dnd5eEvents.AttackChainEvent) (int, error) {
		ch := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
		modified, err := dnd5eEvents.AttackChain.On(bus).PublishWithChain(ctx, event, ch)
		if err != nil {
			return 0, err
		}
		result, err := modified.Execute(ctx, event)
		if err != nil {
			return 0, err
		}
		return len(result.AdvantageSources), nil
	}
	attack := func(target string, sees bool) int {
		got, err := publish(framedAgainst(dnd5eEvents.AttackChainEvent{AttackerID: "attacker", TargetID: target}, 3, sees, held))
		s.Require().NoError(err)
		return got
	}
	s.Equal(1, attack("target", true))
	s.Equal(1, attack("target", true), "attacks do not consume Faerie Fire")
	s.Equal(0, attack("other", true))
	s.Equal(0, attack("target", false), "fog suppresses the benefit without removing the condition")
	s.Equal(1, attack("target", true), "benefit resumes when sight returns")

	unknown := framedAgainst(dnd5eEvents.AttackChainEvent{AttackerID: "attacker", TargetID: "target"}, 3, true, held)
	unknown.Frame.Pairs[0].Sees = contributions.Unknown[bool]()
	_, err = publish(unknown)
	s.Require().Error(err, "unknown sight fails the attack rather than switching the rule off")
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))

	s.Require().NoError(c.Remove(ctx, bus))
	s.Equal(0, attack("target", true), "concentration teardown revokes the benefit")
}
