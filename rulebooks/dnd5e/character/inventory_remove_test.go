package character_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

func TestRemoveInventoryItemDecrementsAnExistingStack(t *testing.T) {
	data := &character.Data{
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 3},
		},
	}

	err := character.RemoveInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1,
	})
	require.NoError(t, err)
	require.Equal(t, []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 2},
	}, data.Inventory)
}

// TestRemoveInventoryItemExactAmountRemovesTheStack pins the fix
// npcs.DecrementVendorStock needed the hard way (#1508): a stack emptied to
// exactly zero must be REMOVED from Inventory, not left at Quantity: 0 —
// character's own strict load already rejects a persisted nonpositive
// quantity (load.go), so leaving one behind would write data this same
// package's next load could not accept.
func TestRemoveInventoryItemExactAmountRemovesTheStack(t *testing.T) {
	data := &character.Data{
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 2},
		},
	}

	err := character.RemoveInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 2,
	})
	require.NoError(t, err)
	require.Empty(t, data.Inventory, "an exactly-exhausted stack must be removed, not left at zero")
}

func TestRemoveInventoryItemInsufficientQuantityIsRefused(t *testing.T) {
	data := &character.Data{
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		},
	}

	err := character.RemoveInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 2,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeNotFound, rpgerr.GetCode(err))
	require.Equal(t, 1, data.Inventory[0].Quantity, "the rejected call must not have touched the stack")
}

func TestRemoveInventoryItemNotOwnedIsRefused(t *testing.T) {
	data := &character.Data{}

	err := character.RemoveInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeNotFound, rpgerr.GetCode(err))
}

func TestRemoveInventoryItemTypeMismatchIsNotOwned(t *testing.T) {
	data := &character.Data{
		Inventory: []character.InventoryItemData{
			// Same ID, different Type: a stack this package would never
			// itself produce, but RemoveInventoryItem matches Type AND ID,
			// so a mismatched Type must not be treated as owned.
			{Type: shared.EquipmentTypeArmor, ID: string(weapons.Longsword), Quantity: 1},
		},
	}

	err := character.RemoveInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeNotFound, rpgerr.GetCode(err))
}

func TestRemoveInventoryItemRejectsNilData(t *testing.T) {
	err := character.RemoveInventoryItem(nil, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
}

func TestRemoveInventoryItemRejectsEmptyID(t *testing.T) {
	data := &character.Data{}
	err := character.RemoveInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, Quantity: 1,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
}

// TestRemoveInventoryItemRejectsNonpositiveQuantity pins the same guard
// AddInventoryItem applies: a zero or negative quantity here would
// otherwise ADD to the stack via the merge branch's arithmetic rather than
// remove from it, so it is rejected outright.
func TestRemoveInventoryItemRejectsNonpositiveQuantity(t *testing.T) {
	for _, quantity := range []int{0, -1} {
		data := &character.Data{
			Inventory: []character.InventoryItemData{
				{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
			},
		}

		err := character.RemoveInventoryItem(data, character.InventoryItemData{
			Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: quantity,
		})
		require.Error(t, err)
		require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
		require.Equal(t, 1, data.Inventory[0].Quantity, "rejected call must not touch the existing stack")
	}
}
