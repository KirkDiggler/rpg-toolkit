package monsters

import (
	"testing"

	"github.com/stretchr/testify/require"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func TestGoblinAuthorsItsScimitarDefinitionDirectly(t *testing.T) {
	definitions := NewGoblin("goblin-1").Actions()
	require.Len(t, definitions, 1)

	scimitar := definitions[0]
	require.Equal(t, refs.MonsterActions.GoblinScimitar(), &scimitar.Ref)
	require.Equal(t, "scimitar", scimitar.Name)
	require.NotNil(t, scimitar.Attack)
	require.Equal(t, &combatActions.MeleeDelivery{ReachFeet: 1}, scimitar.Attack.Delivery.Melee,
		"preserve the existing authored value; correcting its unit is separate work")
}
