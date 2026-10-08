// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// EquipInput is one equipment change on one persisted character: equip ItemID
// into Slot, or, when ItemID is empty, empty Slot.
type EquipInput struct {
	// Character is a record, not a live sheet. The entry clones, strictly
	// loads and attaches it on its own transient interaction surface.
	Character *character.Data

	// Slot is the slot the change names. Required.
	Slot character.InventorySlot

	// ItemID is the inventory item to equip. Empty unequips Slot.
	ItemID string

	// Fight is the turn the character is acting in when the change is made in
	// a fight, and nil when it is not.
	//
	// Nil is free roam: the change costs nothing, because the economy is a
	// fight's (rpg-project#542 R1), and nothing is priced or charged. Present,
	// the door readies the sheet for this turn, prices the change with the
	// rulebook's own rule ([character.Character.PriceEquipment]), charges the
	// price to this sheet, and only then applies the change. Whether the
	// member is in a fight, and whose turn it is, are the encounter's to
	// answer and the session's to ask; this entry is told.
	Fight *Turn
}

// EquipOutput is the changed record and what the change moved.
type EquipOutput struct {
	// Character is an independently owned snapshot of the attached sheet after
	// the change was paid for and applied.
	Character *character.Data

	// Stowed are the items that left a slot: the hands first, main hand
	// before off hand, then any worn slot in slot order. A shield doffed is a
	// stow; an item that only moved between hands is neither. One beat each.
	Stowed []EquipMove

	// Drawn are the items that entered a slot, in Stowed's order. A shield
	// donned is a draw. One beat each, after the stows.
	Drawn []EquipMove

	// Paid is the spend profile charged to the sheet's ledger. Nil when the
	// change cost nothing: free roam, or a change no price can see (an item
	// that only moved between hands, a slot already as asked).
	Paid *combat.SpendProfile
}

// EquipMove is one item leaving or entering one slot: what the session hands
// the encounter's equip record (encounter.RecordEquipInput's Slot and its
// Stowed or Drawn).
type EquipMove struct {
	// Slot is the slot the item left or entered.
	Slot character.InventorySlot

	// Ref is the item as its full ref string ("dnd5e:weapons:longsword"),
	// read off the inventory entry that carries it.
	Ref string
}

// Equip makes one equipment change on one character, charging its price at
// the door first when the change is made in a fight.
//
// The price is charged here, as every turn price is (rpg-project#542, "Equip
// and unequip"); the session compiles nothing and pays nothing. In a fight
// the order is the door's, and every step comes before anything is applied:
//
//   - the sheet is readied for [EquipInput.Fight]'s turn, so a bank left from
//     an earlier turn is full before it is priced, which is the ordering
//     rpg-toolkit#1100 asks callers of a precompiled price to compensate for;
//   - the change is priced against that readied ledger by the rulebook;
//   - the price is charged, all or none.
//
// A price the sheet cannot pay refuses with [ErrCannotPay] and returns no
// record, so nothing is applied and nothing is written. Body armour changed in
// a fight refuses with [character.ErrArmorInFight], reachable by errors.Is. An
// equip the rulebook refuses — an item not carried, a slot it cannot occupy —
// is [ErrBadEquip] wrapping the rulebook's own error.
//
// The rulebook's occupancy rules (a two-handed weapon clears the off hand, a
// single copy moves between hands, an occupied slot is swapped) run inside
// [character.Character.EquipItem] and [character.Character.UnequipItem]
// unchanged. No runtime character or event bus crosses this data boundary.
func Equip(ctx context.Context, in *EquipInput) (*EquipOutput, error) {
	return equipOn(ctx, in, newSurface(events.NewEventBus()))
}

// equipOn is Equip with the surface handed in so lifecycle tests can hold the
// real bus underneath. It stays unexported because callers must not supply or
// retain an interaction bus.
func equipOn(
	ctx context.Context, in *EquipInput, surf *surface,
) (out *EquipOutput, err error) {
	// Teardown on every exit, refusals included; when both the operation and
	// teardown fail, both stay reachable by errors.Is as Resolve does.
	defer func() {
		tearErr := surf.teardown(ctx)
		if tearErr == nil {
			return
		}

		out = nil
		if err != nil {
			err = errors.Join(err, tearErr)
			return
		}
		err = fmt.Errorf("resolution: teardown: %w", tearErr)
	}()

	if in == nil {
		return nil, ErrNilInput
	}
	if in.Slot == "" {
		return nil, fmt.Errorf("%w: no slot named", ErrBadEquip)
	}

	one := Participant{Character: cloneCharacterData(in.Character)}
	if err := one.validate(); err != nil {
		return nil, err
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants: []Participant{one},
		Roller:       refusingRoller{},
		// Strict: this entry writes the sheet back, so a dropped effect would
		// be persisted as a deletion.
	})
	if err != nil {
		return nil, err
	}

	ctx = installTruth(ctx, nil, cast, nil)

	ch, ok := cast.Character(one.ID())
	if !ok {
		return nil, fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, one.ID())
	}

	// The occupancy before the change, owned here: the loader may retain the
	// record's map, and applying the change rewrites the sheet's in place.
	before := maps.Clone(one.Character.EquipmentSlots)

	out = &EquipOutput{}
	var stowedIDs, drawnIDs []string
	if in.Fight != nil {
		price, err := payForEquip(ctx, ch, in, cast)
		if err != nil {
			return nil, err
		}
		out.Paid = price.Profile
		stowedIDs, drawnIDs = price.Stowed, price.Drawn
	}

	if err := applyEquip(ch, in); err != nil {
		return nil, err
	}

	changed, err := ch.ToData()
	if err != nil {
		return nil, fmt.Errorf("resolution: equip %q: %w", one.ID(), err)
	}
	out.Character = changed

	// In a fight the moves are the ones the price was computed from; in free
	// roam nothing was priced, and this package names them. A worn slot can
	// only change in free roam: the price refuses it in a fight.
	if in.Fight == nil {
		stowedIDs, drawnIDs = handsMoved(before, changed.EquipmentSlots)
	}
	out.Stowed, err = movesOf(stowedIDs, before, changed.Inventory)
	if err != nil {
		return nil, fmt.Errorf("resolution: equip %q: %w", one.ID(), err)
	}
	out.Drawn, err = movesOf(drawnIDs, changed.EquipmentSlots, changed.Inventory)
	if err != nil {
		return nil, fmt.Errorf("resolution: equip %q: %w", one.ID(), err)
	}
	if in.Fight == nil {
		wornStowed, wornDrawn, err := wornMoved(before, changed.EquipmentSlots, changed.Inventory)
		if err != nil {
			return nil, fmt.Errorf("resolution: equip %q: %w", one.ID(), err)
		}
		out.Stowed = append(out.Stowed, wornStowed...)
		out.Drawn = append(out.Drawn, wornDrawn...)
	}

	return out, nil
}

// payForEquip readies the sheet for the fight's turn, prices the change and
// charges the price, and returns the price it charged.
func payForEquip(
	ctx context.Context, ch *character.Character, in *EquipInput, cast *Participants,
) (*character.PriceEquipmentOutput, error) {
	if _, err := ch.RefreshForTurn(ctx, &character.RefreshForTurnInput{
		TurnNumber: in.Fight.Number,
		Speed:      in.Fight.Speed,
	}); err != nil {
		return nil, fmt.Errorf("resolution: refresh %q for turn %d: %w", ch.GetID(), in.Fight.Number, err)
	}

	price, err := ch.PriceEquipment(&character.PriceEquipmentInput{Slot: in.Slot, ItemID: in.ItemID})
	if err != nil {
		if errors.Is(err, character.ErrArmorInFight) {
			// A refusal of the change in a fight, not a malformed request: it
			// passes through under its own name.
			return nil, fmt.Errorf("resolution: equip %q: %w", ch.GetID(), err)
		}
		return nil, fmt.Errorf("%w: %q: %w", ErrBadEquip, ch.GetID(), err)
	}

	// The turn was readied above, so the door's own refresh is not asked for:
	// the price was read off this bank and is charged against this bank.
	cost := &Cost{PayerID: ch.GetID(), Profile: price.Profile}
	if err := cost.validate(); err != nil {
		return nil, err
	}
	if err := payAtTheDoor(ctx, cost, cast); err != nil {
		return nil, err
	}

	return price, nil
}

// applyEquip applies the change through the rulebook's own equip verbs.
func applyEquip(ch *character.Character, in *EquipInput) error {
	var err error
	if in.ItemID == "" {
		err = ch.UnequipItem(in.Slot)
	} else {
		err = ch.EquipItem(in.Slot, in.ItemID)
	}
	if err != nil {
		return fmt.Errorf("%w: %q: %w", ErrBadEquip, ch.GetID(), err)
	}
	return nil
}

// movesOf places each hand item id in the hand that holds it in slots, main
// hand first, one hand per copy, and names it by its full ref.
func movesOf(
	ids []string, slots character.EquipmentSlots, inventory []character.InventoryItemData,
) ([]EquipMove, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	taken := map[character.InventorySlot]bool{}
	moves := make([]EquipMove, 0, len(ids))
	for _, id := range ids {
		slot := character.InventorySlot("")
		for _, hand := range [...]character.InventorySlot{character.SlotMainHand, character.SlotOffHand} {
			if !taken[hand] && slots.Get(hand) == id {
				slot = hand
				break
			}
		}
		if slot == "" {
			return nil, fmt.Errorf("%q moved but no hand holds it", id)
		}
		taken[slot] = true

		ref, err := itemRef(id, inventory)
		if err != nil {
			return nil, err
		}
		moves = append(moves, EquipMove{Slot: slot, Ref: ref})
	}

	return moves, nil
}

// wornMoved names what left and entered every slot that is not a hand, in
// slot order.
func wornMoved(
	before, after character.EquipmentSlots, inventory []character.InventoryItemData,
) (stowed, drawn []EquipMove, err error) {
	slots := map[character.InventorySlot]struct{}{}
	for slot := range before {
		slots[slot] = struct{}{}
	}
	for slot := range after {
		slots[slot] = struct{}{}
	}
	ordered := slices.Sorted(maps.Keys(slots))

	for _, slot := range ordered {
		if slot == character.SlotMainHand || slot == character.SlotOffHand {
			continue
		}
		was, will := before.Get(slot), after.Get(slot)
		if was == will {
			continue
		}
		if was != "" {
			ref, refErr := itemRef(was, inventory)
			if refErr != nil {
				return nil, nil, refErr
			}
			stowed = append(stowed, EquipMove{Slot: slot, Ref: ref})
		}
		if will != "" {
			ref, refErr := itemRef(will, inventory)
			if refErr != nil {
				return nil, nil, refErr
			}
			drawn = append(drawn, EquipMove{Slot: slot, Ref: ref})
		}
	}

	return stowed, drawn, nil
}

// itemRef is the full ref of the inventory item with this id, its type read
// off the inventory entry: a weapon, armour (a shield included), a tool, or
// any other carried equipment.
func itemRef(id string, inventory []character.InventoryItemData) (string, error) {
	for _, item := range inventory {
		if item.ID != id {
			continue
		}
		kind := refs.TypeEquipment
		switch item.Type {
		case shared.EquipmentTypeWeapon:
			kind = refs.TypeWeapons
		case shared.EquipmentTypeArmor:
			kind = refs.TypeArmor
		case shared.EquipmentTypeTool:
			kind = refs.TypeTools
		}
		return (&core.Ref{Module: refs.Module, Type: kind, ID: core.ID(id)}).String(), nil
	}

	return "", fmt.Errorf("%q moved but is not in the inventory", id)
}

// handsMoved names what left and what entered the two hands between two
// occupancies, as a multiset of item ids, main hand first, so an item that
// only changed hands is neither.
//
// It is the free-roam half of what [character.Character.PriceEquipment]
// reports in a fight, where no price is compiled because nothing is charged.
// The rulebook's own diff is unexported; the agreement between the two is
// pinned by a test over the same changes priced in a fight.
func handsMoved(before, after character.EquipmentSlots) (stowed, drawn []string) {
	hands := [...]character.InventorySlot{character.SlotMainHand, character.SlotOffHand}
	held := func(slots character.EquipmentSlots) map[string]int {
		counts := map[string]int{}
		for _, hand := range hands {
			if id := slots.Get(hand); id != "" {
				counts[id]++
			}
		}
		return counts
	}

	was, will := held(before), held(after)
	for _, hand := range hands {
		if id := before.Get(hand); id != "" && was[id] > will[id] {
			stowed = append(stowed, id)
			was[id]--
		}
	}

	was = held(before)
	for _, hand := range hands {
		if id := after.Get(hand); id != "" && will[id] > was[id] {
			drawn = append(drawn, id)
			will[id]--
		}
	}

	return stowed, drawn
}
