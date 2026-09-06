package character

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// TestMaterializeItemDecomposesEveryRealPack is a real regression test over
// every pack in the live catalog (rpg-toolkit#1544), not just the two or
// three packs draft_test.go's own suite happens to exercise via class
// choices. Before this change, materializeItem would have produced one
// opaque InventoryItem per pack; this proves each pack instead produces its
// exact Contents, scaled by the requested quantity.
func TestMaterializeItemDecomposesEveryRealPack(t *testing.T) {
	for packID, pack := range packs.All {
		t.Run(string(packID), func(t *testing.T) {
			equip, err := equipment.GetByID(shared.SelectionID(packID))
			require.NoError(t, err)
			require.Equal(t, shared.EquipmentTypePack, equip.EquipmentType())

			got := materializeItem(equip, 2) // quantity > 1 to prove scaling
			require.Len(t, got, len(pack.Contents), "one InventoryItem per content line, not one per pack")

			for i, content := range pack.Contents {
				require.Equal(t, content.ItemID, got[i].Equipment.EquipmentID())
				require.Equal(t, content.Quantity*2, got[i].Quantity,
					"content quantity must scale by the pack quantity requested")
			}
		})
	}
}

// TestMaterializeItemLeavesNonPacksUntouched pins that decomposition is
// specific to EquipmentTypePack — an ordinary weapon still materializes as
// itself.
func TestMaterializeItemLeavesNonPacksUntouched(t *testing.T) {
	equip, err := equipment.GetByID(shared.SelectionID("longsword"))
	require.NoError(t, err)

	got := materializeItem(equip, 3)
	require.Equal(t, []InventoryItem{{Equipment: equip, Quantity: 3}}, got)
}
