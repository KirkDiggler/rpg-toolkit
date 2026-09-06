// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// unpack.go is the host seam's half of decomposing a pack the actor already
// owns into its own Contents (rpg-toolkit#1544, design.md's "Unpack"
// subsection). A pack can reach a character's inventory two ways —
// character-creation grants, which now decompose automatically
// (character/draft.go's materializeItem) — and a vendor purchase
// (session.Trade's buy side), which does NOT auto-decompose: expanding one
// bought line into eight behind a caller's back would be a surprising thing
// for Trade to do on its own. Unpack is the one generic mechanism that
// covers both a pack a character started with and one bought later,
// exactly as design.md requires ("packs are ordinary, re-purchasable shop
// goods in 5e, not a character-creation-only concept").
//
// NO COUNTERPARTY, NO STORY BEAT. Unlike Trade, Unpack's target is
// something the actor already owns — no reach check, no visibility check,
// nothing a witness needs to be told. It is the first session verb with no
// scope.enc calls at all; openForWrite/commit still load and resave the
// encounter aggregate (every write verb does), but this file never reads or
// writes it.

// UnpackInput names the pack to open and how many instances to open at once.
type UnpackInput struct {
	// Session is the session to act in.
	Session string

	// Actor is the member unpacking their own pack.
	Actor string

	// ItemID is the pack's catalog identifier. Must name a Pack the actor
	// already owns.
	ItemID string

	// Quantity is how many pack instances to unpack at once, contents
	// scaled accordingly — the same generic scaling TradeItem.Quantity
	// already applies. Must be strictly positive.
	Quantity int
}

// UnpackOutput reports what unpacking persisted.
type UnpackOutput struct {
	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// Unpack removes Quantity units of the named pack from the actor's own
// inventory and adds each of its Contents lines in their place, scaled by
// Quantity — full decomposition, no partial state: if any content line
// fails to resolve, nothing is touched.
//
// Returns ErrNilInput, ErrNoMemberID (empty actor), ErrInvalidUnpackRequest
// (empty ItemID or a nonpositive Quantity), ErrNotAPack (ItemID does not
// name a pack), ErrBadPackContents (the pack's own Contents don't resolve
// against the catalog — a content-authoring defect, not reachable through
// any pack in the current catalog), ErrNotInInventory (the actor does not
// own at least Quantity units of the pack), ErrNoSessionID, ErrNoSession,
// ErrNoCharacter/ErrBadCharacter/ErrBadRepository (from loading the actor's
// stored sheet), or ErrSaveFailed with a populated report.
func (m *Manager) Unpack(ctx context.Context, in *UnpackInput) (*UnpackOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("unpack: %w", ErrNilInput)
	}
	if in.Actor == "" {
		return nil, fmt.Errorf("unpack: %w", ErrNoMemberID)
	}
	if in.ItemID == "" || in.Quantity <= 0 {
		return nil, fmt.Errorf("unpack: %w", ErrInvalidUnpackRequest)
	}

	// Pure catalog lookup, no session I/O needed, so it runs before
	// openForWrite alongside the other shape refusals — the same
	// "reject cheap" pattern Trade's price check already uses.
	contents, isPack, err := equipment.ResolvePackContents(shared.EquipmentID(in.ItemID))
	if err != nil {
		return nil, fmt.Errorf("unpack: %w: %v", ErrBadPackContents, err)
	}
	if !isPack {
		return nil, fmt.Errorf("unpack: item %q: %w", in.ItemID, ErrNotAPack)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("unpack: %w", err)
	}

	actorData, err := m.fetchCharacterData(ctx, "actor", in.Actor)
	if err != nil {
		return nil, fmt.Errorf("unpack: %w", err)
	}

	if err := character.RemoveInventoryItem(actorData, character.InventoryItemData{
		Type: shared.EquipmentTypePack, ID: in.ItemID, Quantity: in.Quantity,
	}); err != nil {
		return nil, fmt.Errorf("unpack: %w: %v", ErrNotInInventory, err)
	}

	for _, content := range contents {
		if err := character.AddInventoryItem(actorData, character.InventoryItemData{
			Type: content.Type, ID: content.ID, Quantity: content.Quantity * in.Quantity,
		}); err != nil {
			return nil, fmt.Errorf("unpack: %w", err)
		}
	}

	if err := m.saveCharacterRecord(ctx, scope, actorData); err != nil {
		return nil, fmt.Errorf("unpack: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("unpack: %w", err)
	}

	return &UnpackOutput{Saved: report, Delivery: delivery}, nil
}
