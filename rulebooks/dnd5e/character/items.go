// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

// WornSlots are the slots an item with its own record may name. The other
// three slots, the hands and the armour, are read as weapons and armour from
// the rulebook's catalogue, so an item's id there would read as nothing.
func WornSlots() []InventorySlot {
	return []InventorySlot{SlotCloak, SlotAmulet, SlotRingLeft, SlotRingRight, SlotBelt, SlotHelm, SlotBoots}
}

// acceptItem is what both AddItem and the load refuse beside
// [magicitems.Data.Validate]: a slot that is not a worn slot, and an id that
// is an id of the rulebook's catalogue.
func acceptItem(item magicitems.Data) error {
	if err := item.Validate(); err != nil {
		return rpgerr.Wrap(err, "item is not valid")
	}
	worn := WornSlots()
	for _, slot := range item.Description.Slots {
		if !slices.Contains(worn, InventorySlot(slot)) {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"item %q names slot %q, which is not a worn slot: the hands and the armour are read as weapons and armour from the equipment catalogue",
				item.ID, slot)
		}
	}
	if _, err := equipment.GetByID(item.ID); err == nil {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"item id %q is an id of the equipment catalogue: an item with its own record needs an id of its own", item.ID)
	}
	return nil
}

// AddItem puts an item with its own record on saved data, as
// [AddInventoryItem] puts a catalogue row there. The item is copied. It is
// refused when it is not valid, names a slot that is not a worn slot, has the
// id of an item already owned, or has an id of the equipment catalogue.
func AddItem(data *Data, item magicitems.Data) error {
	if data == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character data is required")
	}
	if err := acceptItem(item); err != nil {
		return err
	}
	for _, owned := range data.Items {
		if owned.ID == item.ID {
			return rpgerr.Newf(rpgerr.CodeInvalidArgument, "item %q is already owned", item.ID)
		}
	}
	data.Items = append(data.Items, item.Clone())
	return nil
}

// RemoveItem takes an item off saved data and returns it as it was, count and
// all. Any slot holding it is cleared. An id not owned is refused.
func RemoveItem(data *Data, id string) (magicitems.Data, error) {
	if data == nil {
		return magicitems.Data{}, rpgerr.New(rpgerr.CodeInvalidArgument, "character data is required")
	}
	for i, owned := range data.Items {
		if owned.ID != id {
			continue
		}
		data.Items = slices.Delete(slices.Clone(data.Items), i, i+1)
		for slot, held := range data.EquipmentSlots {
			if held == id {
				delete(data.EquipmentSlots, slot)
			}
		}
		return owned.Clone(), nil
	}
	return magicitems.Data{}, rpgerr.Newf(rpgerr.CodeNotFound, "item %q is not owned", id)
}

// Items returns the items this character owns that keep their own record, as
// copies in id order.
func (c *Character) Items() []magicitems.Data {
	out := make([]magicitems.Data, 0, len(c.items))
	for _, item := range c.items {
		out = append(out, item.Clone())
	}
	slices.SortFunc(out, func(a, b magicitems.Data) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return out
}

// ownedItem finds an owned item by id.
func (c *Character) ownedItem(id string) (magicitems.Data, bool) {
	for _, item := range c.items {
		if item.ID == id {
			return item, true
		}
	}
	return magicitems.Data{}, false
}

// slotsAfterWearing is slotsAfterEquip for an item with its own record: the
// slot must be one the item names, an item worn in a second slot moves (it is
// one thing), and a slot holding something else is replaced.
func (c *Character) slotsAfterWearing(slot InventorySlot, item magicitems.Data) (EquipmentSlots, error) {
	if !slices.Contains(item.Description.Slots, string(slot)) {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "item cannot be equipped in that slot",
			rpgerr.WithMeta("item_id", item.ID),
			rpgerr.WithMeta("slot", slot))
	}

	next := make(EquipmentSlots, len(c.equipmentSlots)+1)
	for held, id := range c.equipmentSlots {
		if id != item.ID {
			next[held] = id
		}
	}
	next.Set(slot, item.ID)
	return next, nil
}
