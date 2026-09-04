package character_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// TestAddInventoryItemAppendsANewStack is the headline: the runtime path
// Trade needs, since compileInventory only ever runs at draft-compile time.
func TestAddInventoryItemAppendsANewStack(t *testing.T) {
	data := &character.Data{}

	err := character.AddInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1,
	})
	require.NoError(t, err)
	require.Equal(t, []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
	}, data.Inventory)
}

func TestAddInventoryItemMergesIntoAnExistingStack(t *testing.T) {
	data := &character.Data{
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		},
	}

	err := character.AddInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 2,
	})
	require.NoError(t, err)
	require.Equal(t, []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 3},
	}, data.Inventory)
}

func TestAddInventoryItemRejectsNilData(t *testing.T) {
	err := character.AddInventoryItem(nil, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
}

func TestAddInventoryItemRejectsEmptyID(t *testing.T) {
	data := &character.Data{}
	err := character.AddInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, Quantity: 1,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
}

// TestAddInventoryItemRejectsNonpositiveQuantity pins the same guard
// DecrementVendorStock applies on the vendor side: a zero or negative
// quantity here must be refused outright rather than merged in, since the
// merge branch would otherwise silently shrink or invert an existing stack.
func TestAddInventoryItemRejectsNonpositiveQuantity(t *testing.T) {
	for _, quantity := range []int{0, -1} {
		data := &character.Data{
			Inventory: []character.InventoryItemData{
				{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
			},
		}

		err := character.AddInventoryItem(data, character.InventoryItemData{
			Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: quantity,
		})
		require.Error(t, err)
		require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
		require.Equal(t, 1, data.Inventory[0].Quantity, "rejected call must not touch the existing stack")
	}
}

func TestAddInventoryItemRejectsUnknownEquipmentID(t *testing.T) {
	data := &character.Data{}
	err := character.AddInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: "not-a-real-weapon", Quantity: 1,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeNotFound, rpgerr.GetCode(err))
}

func TestAddInventoryItemRejectsCatalogTypeMismatch(t *testing.T) {
	data := &character.Data{}
	err := character.AddInventoryItem(data, character.InventoryItemData{
		Type: shared.EquipmentTypeArmor, ID: string(weapons.Longsword), Quantity: 1,
	})
	require.Error(t, err)
	require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
}
