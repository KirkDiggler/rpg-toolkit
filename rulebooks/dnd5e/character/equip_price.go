// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"errors"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// ErrArmorInFight refuses an equipment change that would put on or take off
// body armour while the character is in a fight (rpg-project#542 R6). It
// covers every worn slot, not only SlotArmor: nothing but a hand can change
// in a fight, and today no item can occupy the other worn slots at all.
//
// It is a refusal of the change, never a price. The session translates it into
// its own in-fight refusal; match it with errors.Is.
var ErrArmorInFight = errors.New("body armour cannot change in a fight")

// PriceEquipmentInput is one equipment change to price: equip ItemID into
// Slot, or, when ItemID is empty, empty Slot.
type PriceEquipmentInput struct {
	// Slot is the slot the change names.
	Slot InventorySlot

	// ItemID is the inventory item to equip. Empty means unequip Slot.
	ItemID string
}

// PriceEquipmentOutput is what the change costs this turn, and why.
type PriceEquipmentOutput struct {
	// Profile is the compiled price. Nil means the change moves nothing a
	// price can see — an item that only changes hands, or a slot that was
	// already as asked — and is free.
	Profile *combat.SpendProfile

	// Stowed are the item ids leaving the character's hands, a shield doffed
	// included. Each one costs the action.
	Stowed []string

	// Drawn are the item ids entering the character's hands, a shield donned
	// included. A weapon or other item costs the object interaction while the
	// turn still holds one and the action after; a shield always costs the
	// action.
	Drawn []string
}

// PriceEquipment compiles the price of one equipment change against this
// character's readied turn. It writes nothing: the sheet's slots and ledger
// are exactly as they were.
//
// # For resolution: the door charges this
//
// The profile is charged at resolution's door like every other turn price —
// [combat.Pay] against this sheet, then [Character.EquipItem] or
// [Character.UnequipItem] for the same change. An unaffordable profile is the
// gate's refusal, not this function's: this function prices, it never decides
// whether the price can be paid. Price and apply in one interaction, against
// the same sheet; a price computed against another turn's ledger is a
// different price.
//
// # For session: only in a fight
//
// The price belongs to a fight. Outside one an equip is a plain sheet verb and
// costs nothing, so a session in free roam does not call this at all. The
// ledger must be readied for the actor's turn (see
// [Character.RefreshForTurn]); a sheet with no economy is refused rather than
// priced, because a price read off an absent economy would always say the
// object interaction is spent.
//
// # The rule (rpg-project#542 R1, R6)
//
//   - Putting any held item away costs the action. The 2014 letter makes
//     stowing the free object interaction and dropping free; no drop exists
//     for equipment, so stowing is billed as the action — the divergence R1
//     rules.
//   - Drawing into an empty hand costs the object interaction; when this
//     turn's interaction is spent, a draw costs the action instead (the
//     letter's Use an Object).
//   - A swap is both: the action for the stow, the interaction for the draw.
//   - Donning or doffing a shield costs the action (R6): a shield is not a
//     weapon.
//   - Body armour cannot change in a fight: refused with [ErrArmorInFight].
//
// Each stow, draw and shield is priced on its own, so a change that stows two
// items costs the action twice. The occupancy rules are EquipItem's own — a
// two-handed weapon clears the off hand, a single copy moves between hands —
// so the price is read off the hands before and after exactly that change. An
// item that only moves from one hand to the other is in hand both before and
// after and costs nothing.
//
// An equip EquipItem would refuse is refused here with the same error.
func (c *Character) PriceEquipment(input *PriceEquipmentInput) (*PriceEquipmentOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "no equipment change to price")
	}
	if c.actionEconomy == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidState,
			"an equipment change is priced against a readied turn, and this sheet has none")
	}

	next, err := c.slotsAfter(input)
	if err != nil {
		return nil, err
	}

	for slot := range unionSlots(c.equipmentSlots, next) {
		if isHand(slot) || c.equipmentSlots.Get(slot) == next.Get(slot) {
			continue
		}
		return nil, rpgerr.WrapWithCode(ErrArmorInFight, rpgerr.CodeNotAllowed,
			"equipment change refused in a fight",
			rpgerr.WithMeta("slot", slot))
	}

	stowed, drawn := handsDiff(c.equipmentSlots, next)

	actions, interactions := len(stowed), 0
	interactionsLeft := c.CapacityLeft(combat.CapacityObjectInteraction)
	for _, id := range drawn {
		switch {
		case c.isShield(id):
			actions++
		case interactionsLeft > 0:
			interactions++
			interactionsLeft--
		default:
			actions++
		}
	}

	out := &PriceEquipmentOutput{Stowed: stowed, Drawn: drawn}
	if actions == 0 && interactions == 0 {
		return out, nil
	}

	out.Profile = &combat.SpendProfile{}
	if actions > 0 {
		out.Profile.Slots = map[coreCombat.ActionType]int{coreCombat.ActionStandard: actions}
	}
	if interactions > 0 {
		out.Profile.Capacity = map[combat.CapacityType]int{combat.CapacityObjectInteraction: interactions}
	}

	return out, nil
}

// slotsAfter is the occupancy the change would leave, without writing it.
func (c *Character) slotsAfter(input *PriceEquipmentInput) (EquipmentSlots, error) {
	if input.ItemID != "" {
		return c.slotsAfterEquip(input.Slot, input.ItemID)
	}

	next := EquipmentSlots{}
	for slot, id := range c.equipmentSlots {
		if slot != input.Slot {
			next[slot] = id
		}
	}
	return next, nil
}

// isShield reports whether the inventory item with this id is a shield.
func (c *Character) isShield(itemID string) bool {
	item, _ := c.ownedEquipment(itemID)
	a, ok := item.(*armor.Armor)
	return ok && a.Category == armor.CategoryShield
}

// isHand reports whether a slot is one of the two hands.
func isHand(slot InventorySlot) bool {
	return slot == SlotMainHand || slot == SlotOffHand
}

// unionSlots is every slot either occupancy names.
func unionSlots(a, b EquipmentSlots) map[InventorySlot]struct{} {
	out := make(map[InventorySlot]struct{}, len(a)+len(b))
	for slot := range a {
		out[slot] = struct{}{}
	}
	for slot := range b {
		out[slot] = struct{}{}
	}
	return out
}

// handsDiff compares what the two hands hold before and after, as a multiset
// of item ids, so an item that only changed hands is neither stowed nor drawn.
// The main hand is read first, so the order is stable.
func handsDiff(before, after EquipmentSlots) (stowed, drawn []string) {
	held := func(slots EquipmentSlots) map[string]int {
		counts := map[string]int{}
		for _, hand := range [...]InventorySlot{SlotMainHand, SlotOffHand} {
			if id := slots.Get(hand); id != "" {
				counts[id]++
			}
		}
		return counts
	}

	was, will := held(before), held(after)
	for _, hand := range [...]InventorySlot{SlotMainHand, SlotOffHand} {
		if id := before.Get(hand); id != "" && was[id] > will[id] {
			stowed = append(stowed, id)
			was[id]--
		}
	}

	was = held(before)
	for _, hand := range [...]InventorySlot{SlotMainHand, SlotOffHand} {
		if id := after.Get(hand); id != "" && will[id] > was[id] {
			drawn = append(drawn, id)
			will[id]--
		}
	}

	return stowed, drawn
}
