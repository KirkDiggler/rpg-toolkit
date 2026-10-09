// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
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

	// Slot is the slot the change named: the one the items moved through,
	// what the session hands encounter.RecordEquipInput.Slot.
	Slot character.InventorySlot

	// Stowed are the items the change took off or put away — a weapon stowed,
	// a shield doffed, body armour taken off — as full ref strings
	// ("dnd5e:weapons:longsword"), main hand first, then off hand, then the
	// worn slots. They are the rulebook's plan ([character.Character.PlanEquipment]),
	// the same plan a fight's price is computed from. One beat each.
	Stowed []string

	// Drawn are the items the change put in hand or on, in Stowed's order.
	// One beat each, after the stows.
	Drawn []string

	// Paid is the spend profile charged to the sheet's ledger. Nil when the
	// change cost nothing: free roam, or a change no price can see (an item
	// that only moved between hands, a slot already as asked).
	Paid *combat.SpendProfile
}

// Equip makes one equipment change on one character, charging its price at
// the door first when the change is made in a fight.
//
// The price is charged here, as every turn price is (rpg-project#542, "Equip
// and unequip"); the session compiles nothing and pays nothing. In a fight
// the order is the door's, and every step comes before anything is applied:
//
//   - the sheet is readied for [EquipInput.Fight]'s turn ([ReadyForTurn]):
//     its first act of the fight starts the turn, and a bank left from an
//     earlier turn is full before it is priced, which is the ordering
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

	// The plan comes first and in both paths: it is what the beats say, it
	// needs no turn, and a change the rulebook refuses is refused here before
	// anything is readied or charged.
	plan, err := ch.PlanEquipment(&character.PlanEquipmentInput{Slot: in.Slot, ItemID: in.ItemID})
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrBadEquip, one.ID(), err)
	}

	out = &EquipOutput{Slot: in.Slot, Stowed: refStrings(plan.Stowed), Drawn: refStrings(plan.Drawn)}
	if in.Fight != nil {
		price, err := payForEquip(ctx, ch, in, cast)
		if err != nil {
			return nil, err
		}
		out.Paid = price.Profile
	}

	if err := applyEquip(ch, in); err != nil {
		return nil, err
	}

	changed, err := ch.ToData()
	if err != nil {
		return nil, fmt.Errorf("resolution: equip %q: %w", one.ID(), err)
	}
	out.Character = changed

	return out, nil
}

// ReadyForTurn puts a live sheet into the turn it is about to act in: a sheet
// with no economy yet (its first act of the fight) starts the turn, and one
// already in combat is refreshed for it — an economy filed under this turn is
// left exactly as it is, so a second act cannot refill what the first spent.
//
// It is the one readiness rule for every caller that prices against a turn:
// [Equip] calls it before the rulebook prices the change, and the session
// calls it before it compiles a swing's price. The speed seeded is
// turn.Speed, which the caller computes ([Turn]).
//
// Returns the rulebook's own error from StartTurn or RefreshForTurn.
func ReadyForTurn(ctx context.Context, sheet *character.Character, turn Turn) error {
	if !sheet.InCombat() {
		_, err := sheet.StartTurn(ctx, &character.StartTurnInput{TurnNumber: turn.Number, Speed: turn.Speed})
		return err
	}

	_, err := sheet.RefreshForTurn(ctx, &character.RefreshForTurnInput{TurnNumber: turn.Number, Speed: turn.Speed})
	return err
}

// payForEquip readies the sheet for the fight's turn, prices the change and
// charges the price, and returns the price it charged.
func payForEquip(
	ctx context.Context, ch *character.Character, in *EquipInput, cast *Participants,
) (*character.PriceEquipmentOutput, error) {
	if err := ReadyForTurn(ctx, ch, *in.Fight); err != nil {
		return nil, fmt.Errorf("resolution: ready %q for turn %d: %w", ch.GetID(), in.Fight.Number, err)
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

// refStrings is the rulebook's refs as the strings the record carries.
func refStrings(in []*core.Ref) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, ref := range in {
		out = append(out, ref.String())
	}
	return out
}
