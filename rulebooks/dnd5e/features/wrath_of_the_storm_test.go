package features_test

import (
	"context"
	"encoding/json"
	"testing"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
)

func TestWrathOfTheStormIsReactionOnly(t *testing.T) {
	out, err := features.CreateFromRef(&features.CreateFromRefInput{Ref: refs.Features.WrathOfTheStorm().String(), Config: json.RawMessage(`{}`), CharacterID: "tempest-1"})
	require.NoError(t, err)
	require.Equal(t, coreCombat.ActionReaction, out.Feature.ActionType())
	require.ErrorContains(t, out.Feature.CanActivate(context.Background(), nil, features.FeatureInput{}), "offered only after a hit")
	require.ErrorContains(t, out.Feature.Activate(context.Background(), nil, features.FeatureInput{}), "offered only after a hit")
}

func TestWrathOfTheStormRoundTripsIdentity(t *testing.T) {
	out, err := features.CreateFromRef(&features.CreateFromRefInput{Ref: refs.Features.WrathOfTheStorm().String(), Config: json.RawMessage(`{}`), CharacterID: "tempest-1"})
	require.NoError(t, err)
	encoded, err := out.Feature.ToJSON()
	require.NoError(t, err)
	loaded, err := features.LoadJSON(encoded)
	require.NoError(t, err)
	require.Equal(t, out.Feature.GetID(), loaded.GetID())
}
