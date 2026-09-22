package features

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dndCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/require"
)

func TestWrathOfTheStormIsReactionOnly(t *testing.T) {
	out, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Features.WrathOfTheStorm().String(), Config: json.RawMessage(`{}`), CharacterID: "tempest-1"})
	require.NoError(t, err)
	require.Equal(t, coreCombat.ActionReaction, out.Feature.ActionType())
	require.ErrorContains(t, out.Feature.CanActivate(context.Background(), nil, FeatureInput{}), "offered only after a hit")
	require.ErrorContains(t, out.Feature.Activate(context.Background(), nil, FeatureInput{}), "offered only after a hit")
}

func TestWrathOfTheStormRoundTripsIdentity(t *testing.T) {
	out, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Features.WrathOfTheStorm().String(), Config: json.RawMessage(`{}`), CharacterID: "tempest-1"})
	require.NoError(t, err)
	encoded, err := out.Feature.ToJSON()
	require.NoError(t, err)
	loaded, err := LoadJSON(encoded)
	require.NoError(t, err)
	require.Equal(t, out.Feature.GetID(), loaded.GetID())
}

type wrathTestMember struct{}

func (wrathTestMember) GetID() string           { return "owner" }
func (wrathTestMember) GetHitPoints() int       { return 10 }
func (wrathTestMember) GetMaxHitPoints() int    { return 10 }
func (wrathTestMember) AC() int                 { return 10 }
func (wrathTestMember) HasShieldEquipped() bool { return false }
func (wrathTestMember) CanReact() bool          { return true }
func (wrathTestMember) AbilityScores() shared.AbilityScores {
	return shared.AbilityScores{abilities.WIS: 15}
}
func (wrathTestMember) ProficiencyBonus() int  { return 2 }
func (wrathTestMember) PassivePerception() int { return 10 }

type wrathTestCast struct {
	visible bool
}

func (c *wrathTestCast) Member(id string) (dndCombat.Member, bool) {
	if id != "owner" {
		return nil, false
	}
	return wrathTestMember{}, true
}
func (c *wrathTestCast) Members() []string                  { return []string{"owner", "attacker"} }
func (c *wrathTestCast) IsHostile(_, _ string) (bool, bool) { return true, true }
func (c *wrathTestCast) IsAllied(_, _ string) (bool, bool)  { return false, true }
func (c *wrathTestCast) ResourceStatus(string, coreResources.ResourceKey) (int, int, bool) {
	return 1, 1, true
}
func (c *wrathTestCast) HasCondition(string, *core.Ref) (bool, bool) { return false, true }
func (c *wrathTestCast) SeesWithin(_, _ string, _ int) (bool, bool)  { return c.visible, true }

var _ gamectx.Cast = (*wrathTestCast)(nil)

func TestWrathOfTheStormRequiresVisibleAttacker(t *testing.T) {
	for _, tc := range []struct {
		name      string
		visible   bool
		wantOffer bool
	}{
		{name: "visible", visible: true, wantOffer: true},
		{name: "hidden by fog", visible: false, wantOffer: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			feature := &WrathOfTheStorm{id: "wrath", name: "Wrath of the Storm", characterID: "owner"}
			ctx := gamectx.WithCast(context.Background(), &wrathTestCast{visible: tc.visible})
			event := &dndEvents.PostHitEvent{AttackerID: "attacker", TargetID: "owner"}
			c := events.NewStagedChain[*dndEvents.PostHitEvent](dndCombat.ModifierStages)
			_, err := feature.onHit(ctx, event, c)
			require.NoError(t, err)
			out, err := c.Execute(ctx, event)
			require.NoError(t, err)
			if tc.wantOffer {
				require.Len(t, out.Offers, 1)
				require.Equal(t, "wrath_of_the_storm", out.Offers[0].ResourceKey)
			} else {
				require.Empty(t, out.Offers)
			}
		})
	}
}
