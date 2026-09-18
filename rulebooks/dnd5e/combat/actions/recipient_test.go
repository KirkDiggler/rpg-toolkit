package actions_test

import (
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCastRecipientCooldown(t *testing.T) {
	p := actions.CastProfile{RecipientBlockedBy: []core.Ref{*refs.Conditions.SanctuaryImmune()}}
	for _, source := range []string{"first-caster", "other-caster"} {
		blob, err := json.Marshal(map[string]any{"ref": refs.Conditions.SanctuaryImmune(), "source_id": source})
		require.NoError(t, err)
		allowed, err := p.AllowsRecipient([]json.RawMessage{blob})
		require.NoError(t, err)
		require.False(t, allowed)
	}
	allowed, err := p.AllowsRecipient(nil)
	require.NoError(t, err)
	require.True(t, allowed)
	_, err = p.AllowsRecipient([]json.RawMessage{json.RawMessage(`{`)})
	require.Error(t, err)
	clone := p.Clone()
	clone.RecipientBlockedBy[0] = *refs.Conditions.Sanctuary()
	require.Equal(t, *refs.Conditions.SanctuaryImmune(), p.RecipientBlockedBy[0])
}
