package equipment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// TestEveryPackContentItemResolves is a real regression test, not a
// snapshot of the two packs that happened to work before rpg-toolkit#1544:
// it reads packs.All directly and checks every single Contents line
// against the live catalog, the same way Unpack itself will. Before this
// change, 5 of 7 packs failed here (candle, ink, chest, vestments, and
// ~20 others existed nowhere in the catalog).
func TestEveryPackContentItemResolves(t *testing.T) {
	for packID, pack := range packs.All {
		t.Run(string(packID), func(t *testing.T) {
			require.NotEmpty(t, pack.Contents, "a pack with no contents proves nothing")
			for _, content := range pack.Contents {
				detail := equipment.ResolveEquipmentDetail(shared.EquipmentID(content.ItemID))
				require.NotNil(t, detail, "content item %q has no catalog entry", content.ItemID)
				require.Positive(t, content.Quantity, "content item %q has a nonpositive quantity", content.ItemID)
			}
		})
	}
}

// TestPackContentQuantitiesMatchTheirCatalogUnit pins the fix for a real
// bug this suite's own predecessor caught: a PackItem.Quantity must count
// CATALOG UNITS, not the physical quantity a flavor description mentions.
// "50 feet of hempen rope" is Quantity: 1 of the one-coil catalog entry,
// not Quantity: 50; "a bag of 1,000 ball bearings" is Quantity: 1 of the
// one-bag entry, not Quantity: 1000. Every pack referencing these two IDs
// is checked explicitly, since the bug repeated across three packs
// (BurglarPack, DungeoneerPack, ExplorerPack) before being caught.
func TestPackContentQuantitiesMatchTheirCatalogUnit(t *testing.T) {
	for packID, pack := range packs.All {
		for _, content := range pack.Contents {
			switch content.ItemID {
			case string(items.HempenRope):
				require.Equal(t, 1, content.Quantity,
					"%s: hempen-rope names a whole 50-foot coil per unit", packID)
			case string(items.BallBearings):
				require.Equal(t, 1, content.Quantity,
					"%s: ball-bearings names a whole 1,000-bearing bag per unit", packID)
			}
		}
	}
}
