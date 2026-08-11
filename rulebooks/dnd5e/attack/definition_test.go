package attack_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/attack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/stretchr/testify/require"
)

func TestNaturalDefinitionAllowsNoEquipmentWeapon(t *testing.T) {
	def := attack.Definition{
		ActionID: "pseudopod", DisplayName: "Pseudopod", Category: attack.CategoryNatural,
		Bonus: attack.FixedBonus(3), Targeting: attack.MeleeReach(1),
		Damage: damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6", Type: damage.Bludgeoning}}},
	}

	require.NoError(t, def.Validate())
}

func TestEquipmentWeaponDefinitionRequiresWeapon(t *testing.T) {
	require.Error(t, (&attack.Definition{
		ActionID: "slash", Category: attack.CategoryEquipmentWeapon,
		Bonus: attack.DerivedBonus(), Targeting: attack.MeleeReach(1),
		Damage: damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d8", Type: damage.Slashing}}},
	}).Validate())
}

func TestDefinitionsWithSameActionIDRemainIndependent(t *testing.T) {
	gray := validNatural("pseudopod", 3)
	ochre := validNatural("pseudopod", 4)

	require.Equal(t, gray.ActionID, ochre.ActionID)
	require.NotEqual(t, gray.Bonus, ochre.Bonus)
}

func TestRangedTargetingValidation(t *testing.T) {
	require.NoError(t, attack.Ranged(30, 120).Validate())
	require.Error(t, attack.Ranged(0, 120).Validate())
	require.Error(t, attack.Ranged(30, 29).Validate())
}

func validNatural(actionID string, bonus int) attack.Definition {
	return attack.Definition{
		ActionID: actionID, Category: attack.CategoryNatural,
		Bonus: attack.FixedBonus(bonus), Targeting: attack.MeleeReach(1),
		Damage: damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6", Type: damage.Bludgeoning}}},
	}
}
