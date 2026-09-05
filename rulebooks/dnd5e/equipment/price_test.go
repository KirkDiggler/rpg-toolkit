package equipment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

func TestPriceOfResolvesEveryCatalogKind(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want currency.Money
	}{
		{"weapon", string(weapons.Longsword), currency.FromGold(15)},
		{"item", string(items.Torch), currency.FromCopper(1)},
		{"pack", string(packs.ExplorerPack), currency.FromGold(10)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := equipment.PriceOf(tc.id)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestPriceOfUnknownIDIsNotFound(t *testing.T) {
	_, err := equipment.PriceOf("no-such-item")
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeNotFound, rpgerr.GetCode(err))
}

// TestPriceOfResolvesEveryPopulatedAdventuringGearItem is the actual reason
// this wave populated items.All: every one of the 14 adventuring-gear
// constants must now resolve a real price, not just exist as an ItemID with
// no catalog entry.
func TestPriceOfResolvesEveryPopulatedAdventuringGearItem(t *testing.T) {
	for id := range items.All {
		t.Run(string(id), func(t *testing.T) {
			_, err := equipment.PriceOf(id)
			require.NoError(t, err)
		})
	}
}

// TestBurglarPackLanternIsSellable pins the fix for a real bug this wave
// found: BurglarPack's Contents referenced "hooded-lantern", which named no
// catalog entry — items.Lantern's actual ID is "lantern". A lantern granted
// by that pack would never have resolved a price or an equipment.GetByID
// lookup at all.
func TestBurglarPackLanternIsSellable(t *testing.T) {
	pack, ok := packs.All[packs.BurglarPack]
	require.True(t, ok)

	found := false
	for _, content := range pack.Contents {
		if content.ItemID != string(items.Lantern) {
			continue
		}
		found = true
		_, err := equipment.PriceOf(content.ItemID)
		require.NoError(t, err)
	}
	require.True(t, found, "BurglarPack must still grant a lantern, under a resolvable ID")
}
