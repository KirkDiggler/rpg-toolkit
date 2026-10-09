package features

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// undescribedFeatures are features that spend a resource without delivering
// their benefit, so their card shows missing information rather than prose the
// code does not honour. Each entry names the defect that lifts it.
var undescribedFeatures = map[string]string{
	refs.Features.PatientDefense().ID:  "toolkit#1986",
	refs.Features.DeflectMissiles().ID: "toolkit#1992",
}

// loadableFeatureRefs reflects over the refs namespace and keeps every ref the
// feature factory can build, so a feature added to refs and the factory without
// prose is caught by the loop below.
func loadableFeatureRefs(t *testing.T) []*core.Ref {
	t.Helper()
	var loadable []*core.Ref
	namespace := reflect.ValueOf(refs.Features)
	for i := 0; i < namespace.NumMethod(); i++ {
		ref, ok := namespace.Method(i).Call(nil)[0].Interface().(*core.Ref)
		if !ok || ref == nil {
			continue
		}
		if _, err := CreateFromRef(&CreateFromRefInput{Ref: ref.String(), Config: json.RawMessage(`{}`), CharacterID: "owner"}); err == nil {
			loadable = append(loadable, ref)
		}
	}
	require.NotEmpty(t, loadable)
	return loadable
}

func TestEveryLoadableFeatureDescribesItself(t *testing.T) {
	for _, ref := range loadableFeatureRefs(t) {
		t.Run(ref.ID, func(t *testing.T) {
			out, err := CreateFromRef(&CreateFromRefInput{Ref: ref.String(), Config: json.RawMessage(`{}`), CharacterID: "owner"})
			require.NoError(t, err)
			encoded, err := out.Feature.ToJSON()
			require.NoError(t, err)
			loaded, err := LoadJSON(encoded)
			require.NoError(t, err)
			if _, undescribed := undescribedFeatures[ref.ID]; undescribed {
				require.Empty(t, loaded.Description())
				return
			}
			require.NotEmpty(t, loaded.Description())
			require.Equal(t, out.Feature.Description(), loaded.Description())
		})
	}
}

// Patient Defense (toolkit#1986) and Deflect Missiles (toolkit#1992) spend a
// resource without delivering their benefit, so their cards show missing
// information. This pins that ruling; remove an entry with its repair.
func TestUndescribedFeaturesStayAbsent(t *testing.T) {
	for id, issue := range undescribedFeatures {
		out, err := CreateFromRef(&CreateFromRefInput{Ref: "dnd5e:features:" + id, Config: json.RawMessage(`{}`), CharacterID: "owner"})
		require.NoError(t, err, issue)
		require.Empty(t, out.Feature.Description(), issue)
	}
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
