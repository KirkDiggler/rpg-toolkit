package features

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// loadableFeatureRefs is every ref LoadJSON routes, so a feature added to the
// loader without prose is caught by the loop below.
func loadableFeatureRefs() []*core.Ref {
	return []*core.Ref{
		refs.Features.BlessingOfTheTrickster(),
		refs.Features.BardicInspiration(),
		refs.Features.Rage(),
		refs.Features.SecondWind(),
		refs.Features.WardingFlare(),
		refs.Features.WrathOfTheStorm(),
		refs.Features.ActionSurge(),
		refs.Features.FlurryOfBlows(),
		refs.Features.PatientDefense(),
		refs.Features.StepOfTheWind(),
		refs.Features.RecklessAttack(),
		refs.Features.DeflectMissiles(),
	}
}

func TestEveryLoadableFeatureDescribesItself(t *testing.T) {
	require.Len(t, loadableFeatureRefs(), 12)
	for _, ref := range loadableFeatureRefs() {
		t.Run(ref.ID, func(t *testing.T) {
			out, err := CreateFromRef(&CreateFromRefInput{Ref: ref.String(), Config: json.RawMessage(`{}`), CharacterID: "owner"})
			require.NoError(t, err)
			encoded, err := out.Feature.ToJSON()
			require.NoError(t, err)
			loaded, err := LoadJSON(encoded)
			require.NoError(t, err)
			if ref.ID == refs.Features.PatientDefense().ID {
				return
			}
			require.NotEmpty(t, loaded.Description())
			require.Equal(t, out.Feature.Description(), loaded.Description())
		})
	}
}

// Patient Defense spends ki without delivering its Dodge (toolkit#1986), so its
// card shows missing information. This pins that ruling; delete it with the
// repair, which lands the prose.
func TestPatientDefenseDescriptionIsAbsent(t *testing.T) {
	out, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Features.PatientDefense().String(), Config: json.RawMessage(`{}`), CharacterID: "owner"})
	require.NoError(t, err)
	require.Empty(t, out.Feature.Description())
}

func TestWardingFlareOfferDescribed(t *testing.T) {
	feature := &WardingFlare{id: "flare", name: "Warding Flare", characterID: "owner"}
	bus := events.NewEventBus()
	require.NoError(t, feature.Apply(context.Background(), bus))
	ctx := gamectx.WithCast(context.Background(), &flareCast{
		wrathTestCast: wrathTestCast{visible: true}, remaining: 1, distance: 5, known: true,
	})
	e := dndEvents.AttackChainEvent{AttackerID: "attacker", TargetID: "owner"}
	c, err := dndEvents.AttackChain.On(bus).PublishWithChain(ctx, e, events.NewStagedChain[dndEvents.AttackChainEvent](combat.ModifierStages))
	require.NoError(t, err)
	folded, err := c.Execute(ctx, e)
	require.NoError(t, err)
	require.Len(t, folded.BeforeRollOffers, 1)
	require.NotEmpty(t, folded.BeforeRollOffers[0].Description)
	require.Equal(t, feature.Description(), folded.BeforeRollOffers[0].Description)
}

func TestWrathOfTheStormOptionsDescribed(t *testing.T) {
	feature := &WrathOfTheStorm{id: "wrath", name: "Wrath of the Storm", characterID: "owner"}
	ctx := gamectx.WithCast(context.Background(), &wrathTestCast{visible: true})
	event := &dndEvents.PostHitEvent{AttackerID: "attacker", TargetID: "owner"}
	c := events.NewStagedChain[*dndEvents.PostHitEvent](combat.ModifierStages)
	_, err := feature.onHit(ctx, event, c)
	require.NoError(t, err)
	out, err := c.Execute(ctx, event)
	require.NoError(t, err)
	require.Len(t, out.Offers, 1)
	require.Equal(t, feature.Description(), out.Offers[0].Description)
	require.Len(t, out.Offers[0].Options, 2)
	descriptions := map[string]bool{}
	for _, option := range out.Offers[0].Options {
		require.NotEmpty(t, option.Description, option.ID)
		descriptions[option.Description] = true
	}
	require.Len(t, descriptions, 2, "each option explains its own damage type")
}
