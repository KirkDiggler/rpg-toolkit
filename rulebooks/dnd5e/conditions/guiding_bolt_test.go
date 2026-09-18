package conditions

import (
	"context"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGuidingBoltTargetConsumesOnceForAnyAttacker(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	c, err := NewGuidingBoltCondition(NewGuidingBoltConditionInput{MemberID: "target", SourceID: "caster", SourceRef: refs.Spells.GuidingBolt()})
	require.NoError(t, err)
	require.NoError(t, c.Apply(ctx, bus))
	attack := func(target string) dnd5eEvents.AttackChainEvent {
		e := dnd5eEvents.AttackChainEvent{AttackerID: "ally", TargetID: target}
		chain, err := dnd5eEvents.AttackChain.On(bus).PublishWithChain(ctx, e, events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages))
		require.NoError(t, err)
		out, err := chain.Execute(ctx, e)
		require.NoError(t, err)
		return out
	}
	require.Empty(t, attack("other").AdvantageSources)
	require.True(t, c.IsApplied())
	result := attack("target")
	require.Len(t, result.AdvantageSources, 1)
	require.Equal(t, "caster", result.AdvantageSources[0].SourceID)
	require.True(t, c.IsApplied(), "assembling a chain is not rolling an attack")
	rolled := &dnd5eEvents.PostRollOfferEvent{AttackerID: "ally", TargetID: "target", Roll: 1}
	_, err = dnd5eEvents.PostRollOfferChain.On(bus).PublishWithChain(ctx, rolled, events.NewStagedChain[*dnd5eEvents.PostRollOfferEvent](combat.ModifierStages))
	require.NoError(t, err)
	require.False(t, c.IsApplied())
	require.Empty(t, attack("target").AdvantageSources)
}

func TestGuidingBoltCasterClockSurvivesReload(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	c, err := NewGuidingBoltCondition(NewGuidingBoltConditionInput{MemberID: "target", SourceID: "caster", SourceRef: refs.Spells.GuidingBolt()})
	require.NoError(t, err)
	require.NoError(t, c.Apply(ctx, bus))
	changes := 0
	_, err = dnd5eEvents.ConditionStateChangedTopic.On(bus).Subscribe(ctx, func(_ context.Context, e dnd5eEvents.ConditionStateChangedEvent) error {
		changes++
		require.Equal(t, "target", e.MemberID)
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, dnd5eEvents.TurnEndTopic.On(bus).Publish(ctx, dnd5eEvents.TurnEndEvent{SubjectID: "target"}))
	require.Equal(t, 2, c.TurnEndsLeft)
	require.NoError(t, dnd5eEvents.TurnEndTopic.On(bus).Publish(ctx, dnd5eEvents.TurnEndEvent{SubjectID: "caster"}))
	require.Equal(t, 1, c.TurnEndsLeft)
	require.Equal(t, 1, changes)
	raw, err := c.ToJSON()
	require.NoError(t, err)
	require.NoError(t, c.Remove(ctx, bus))
	loaded, err := LoadJSON(raw)
	require.NoError(t, err)
	require.NoError(t, loaded.Apply(ctx, bus))
	require.NoError(t, dnd5eEvents.TurnEndTopic.On(bus).Publish(ctx, dnd5eEvents.TurnEndEvent{SubjectID: "caster"}))
	require.False(t, loaded.IsApplied())
}
