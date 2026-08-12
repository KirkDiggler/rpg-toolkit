package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/attack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/stretchr/testify/require"
)

func TestOchreJellyPseudopod(t *testing.T) {
	def := ochreJellyPseudopod(t)
	require.Equal(t, "pseudopod", def.ActionID)
	require.Equal(t, attack.CategoryNatural, def.Category)
	require.Equal(t, 1, def.Targeting.Reach)
	require.Equal(t, 4, def.Bonus.Fixed)
	require.Equal(t, []damage.Damage{
		{Dice: "2d6", Type: damage.Bludgeoning, FlatBonus: -2, Properties: []damage.Property{damage.PropertyCritEligible}},
		{Dice: "1d6", Type: damage.Acid, Properties: []damage.Property{damage.PropertyCritEligible}},
	}, def.Damage.Pools)
}

func TestOozesShareActionIDWithoutSharingRules(t *testing.T) {
	gray, ochre := grayOozePseudopod(t), ochreJellyPseudopod(t)
	require.Equal(t, gray.ActionID, ochre.ActionID)
	require.NotEqual(t, gray.Bonus.Fixed, ochre.Bonus.Fixed)
	require.NotEqual(t, gray.Damage.Pools, ochre.Damage.Pools)
}

func ochreJellyPseudopod(t *testing.T) attack.Definition {
	t.Helper()
	return emittedPseudopod(t, NewOchreJelly("ochre-jelly"))
}
