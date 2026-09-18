package monsters

import (
	"testing"

	"github.com/stretchr/testify/require"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func TestGoblinWieldsCatalogWeapons(t *testing.T) {
	definitions := NewGoblin("goblin-1").Actions()
	require.Len(t, definitions, 2)

	scimitar := definitions[0]
	require.Equal(t, refs.Weapons.Scimitar(), &scimitar.Ref,
		"the action's ref is the weapon's; there is no such thing as a goblin scimitar")
	require.Equal(t, "Scimitar", scimitar.Name)
	require.NotNil(t, scimitar.Attack)
	require.Equal(t, &combatActions.MeleeDelivery{ReachFeet: 5}, scimitar.Attack.Delivery.Melee,
		"the reach comes off the catalog now, so the authored one-foot defect cannot come back")

	shortbow := definitions[1]
	require.Equal(t, refs.Weapons.Shortbow(), &shortbow.Ref)
	require.Equal(t, &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320},
		shortbow.Attack.Delivery.Ranged)
}
