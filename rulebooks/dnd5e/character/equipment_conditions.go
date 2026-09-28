// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// removeReleasedEquipmentConditions is shared by both equipment mutations.
// Removal happens immediately, so putting the same weapon back cannot revive
// an expired enchantment. It works on pure loaded sheets as well as attached
// sheets; only the latter have an event audience to publish to.
func (c *Character) removeReleasedEquipmentConditions(previous EquipmentSlots) error {
	current := append([]dnd5eEvents.ConditionBehavior(nil), c.conditions...)
	for _, condition := range current {
		bound, ok := condition.(interface{ EquipmentBinding() (string, string) })
		if !ok {
			continue
		}
		slot, itemID := bound.EquipmentBinding()
		if c.equipmentSlots.Get(InventorySlot(slot)) == itemID {
			continue
		}
		// EquipItem can move a single owned copy directly between hands.
		// Preserve that known identity; an explicit UnequipItem supplies no
		// previous slots and is always a release, even if later re-equipped.
		if mover, ok := condition.(interface{ MoveEquipmentBinding(string) }); ok && previous != nil {
			moved := false
			for _, hand := range []InventorySlot{SlotMainHand, SlotOffHand} {
				if c.equipmentSlots.Get(hand) == itemID && previous.Get(hand) != itemID {
					mover.MoveEquipmentBinding(string(hand))
					c.dirty = true
					if c.bus != nil {
						if err := dnd5eEvents.ConditionStateChangedTopic.On(c.bus).Publish(context.Background(), dnd5eEvents.ConditionStateChangedEvent{MemberID: c.id, ConditionRef: condition.Ref()}); err != nil {
							return err
						}
					}
					moved = true
					break
				}
			}
			if moved {
				continue
			}
		}
		address := conditions.ConditionAddressOf(c.id, condition)
		removed := dnd5eEvents.ConditionRemovedEvent{MemberID: c.id, ConditionRef: address.ConditionRef, SourceID: address.SourceID, Reason: "weapon released"}
		if c.bus != nil {
			if err := dnd5eEvents.ConditionRemovedTopic.On(c.bus).Publish(context.Background(), removed); err != nil {
				return err
			}
		} else if err := c.onConditionRemoved(context.Background(), nil, removed); err != nil {
			return err
		}
	}
	return nil
}
