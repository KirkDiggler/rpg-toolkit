package equipment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// TestNewFixedItemsResolve covers the four catalog entries added for
// background-grants (rpg-toolkit#1554 PR 1) — common clothes, traveler's
// clothes, hunting trap, and silk rope — sourced and cross-checked against
// two independent PHB/SRD references. Scroll case needed no new entry:
// items.CaseMap ("Case, Map or Scroll", 1gp/1lb) already covers it.
func TestNewFixedItemsResolve(t *testing.T) {
	tests := []struct {
		name       string
		id         items.ItemID
		wantWeight float32
		wantValue  int // copper pieces
	}{
		{"common clothes", items.ClothesCommon, 3, 50},
		{"traveler's clothes", items.ClothesTraveler, 4, 200},
		{"hunting trap", items.HuntingTrap, 25, 500},
		{"silk rope", items.SilkRope, 5, 1000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			equip, err := equipment.GetByID(tc.id)
			require.NoError(t, err)
			require.Equal(t, shared.EquipmentTypeItem, equip.EquipmentType())
			require.Equal(t, tc.wantWeight, equip.EquipmentWeight())

			price, err := equipment.PriceOf(string(tc.id))
			require.NoError(t, err)
			require.Equal(t, tc.wantValue, price.Copper)
		})
	}
}
