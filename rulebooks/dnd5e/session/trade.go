// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// trade.go is the host seam's half of buying from a placed vendor
// (rpg-project#369, design.md at rpg-project#370). Interact already answers
// "what does this vendor carry"; this file is the verb that actually moves
// an item — decrementing the vendor's stored stock and adding the item to
// the buyer's character inventory — over the same reach/visibility check
// Interact already performs.
//
// This wave's behavior is deliberately narrower than the wire shape:
// Give.Items must be empty (no item-for-item barter yet — that raises
// unresolved questions about what an unlisted item becomes in a vendor's
// stock) and Receive must name exactly one item, at any quantity up to what
// the vendor holds (multiple DISTINCT items in one call needs validating
// every line before committing any of them, deferred to rpg-toolkit#1275's
// Quote, which is already a multi-line shape). See design.md §4 for the full
// reasoning.
//
// Give.Currency IS the payment (rpg-toolkit#1534, design.md §5 Wave 4).
// THE SERVER ALONE DECIDES WHETHER AN OFFERED PRICE IS CORRECT — this is a
// security property, not a style choice (Kirk, 2026-09-05: "we need price
// validation on both sides, absolutely... none of this shit where the
// client hacks in their prices"). Trade computes the required price itself
// via equipment.PriceOf, scaled by quantity, and requires Give.Currency to
// match it EXACTLY — no tolerance, no partial credit. A client reads a
// vendor's listed price purely to display it and pre-populate what to send;
// it is never trusted as the amount actually charged. The same law will
// apply symmetrically once selling exists.

// TradeItem is one item and quantity on one side of a TradeOffer.
type TradeItem struct {
	// Type is the item's D&D equipment category.
	Type shared.EquipmentType

	// ID is the item's catalog identifier.
	ID string

	// Quantity is how many units of Type/ID this line names.
	Quantity int
}

// TradeOffer is one side of an exchange: the items and/or currency moving in
// that direction.
type TradeOffer struct {
	// Items are the item lines this offer names.
	Items []TradeItem

	// Currency is the coin moving in this offer's direction. On Give, this
	// is the payment: it must exactly equal the server-computed price of
	// what Receive names, or the trade is refused (ErrWrongPrice) — see
	// this file's own doc for why an offered amount is never trusted as
	// correct on its own.
	Currency currency.Money
}

// TradeInput names who is trading with whom, over what reach, and what
// moves in each direction.
//
// THIS WAVE ACCEPTS ONLY ONE SHAPE: Give.Items empty, Give.Currency exactly
// the price of Receive, Receive.Items exactly one entry. The type is wider
// than that on purpose — Give/Receive already say everything a
// buy/sell/barter/gift direction field would, so the wire contract does not
// need to change when a later wave lifts these limits — but this verb
// refuses everything outside that one shape today rather than silently
// narrowing a caller's request. See design.md §3-5.
type TradeInput struct {
	// Session is the session to act in.
	Session string

	// Actor is the initiating player member.
	Actor string

	// Target is the KindWorld vendor member being traded with.
	Target string

	// Range is the maximum distance, in cells, Target may stand from Actor.
	// Zero (the default) means adjacent — one cell. Forwarded to
	// encounter.Interact untouched, the same convention InteractInput.Range
	// documents.
	Range int

	// Give is what Actor hands over. Items must be empty this wave; Currency
	// is the payment and must exactly equal Receive's server-computed price.
	Give TradeOffer

	// Receive is what Actor gets. Must name exactly one item this wave.
	Receive TradeOffer
}

// TradeOutput reports the descriptor reached (reflecting the stock
// decrement) and what trading recorded.
type TradeOutput struct {
	// Descriptor is what the target IS now, rebuilt after the decrement —
	// the same shape InteractOutput.Descriptor carries, so a caller can
	// refresh its stock display from this response without a second
	// Interact round trip.
	Descriptor WorldNPCDescriptor

	// Seq is the sequence number of the recorded `traded` beat.
	Seq uint64

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// Trade buys one item from a placed vendor: confirms reach and visibility
// exactly as Interact does (reusing encounter.Interact as-is — design.md
// §5's own recommendation — so a purchase carries two beats, `interacted`
// then `traded`, rather than factoring reach-checking into a second private
// path), decrements the vendor's stored stock, adds the item to the actor's
// character inventory, and records the `traded` beat.
//
// The character save lands BEFORE the story beat, the same ordering
// death_save.go documents for its own save-then-record call: if Record then
// fails, reportUnrecorded's caller already knows the character write landed.
//
// Returns ErrNilInput, ErrNoMemberID (empty actor or target), ErrGiveNotSupported
// (Give.Items is non-empty — sell/barter is not this wave's), ErrInvalidTradeOffer
// (Receive.Items is not exactly one entry, or that entry has an empty ID or
// a nonpositive quantity), ErrWrongPrice (Give.Currency does not exactly
// equal the server-computed price of Receive), ErrNoSessionID, ErrNoSession,
// ErrNoEncounter, ErrClosed, ErrNoMember (actor or target missing from the
// roster, or present but the wrong kind), ErrBadPosition, ErrOutOfRange,
// ErrNotVisible (all forwarded from the underlying Interact check),
// ErrNoSheet (target confirmed as KindWorld but session's own WorldNPCs
// store has nothing recorded for it), ErrNotAVendor (target has no
// npc.CapabilityVendor), ErrOutOfStock (the vendor does not carry the item,
// or not enough of it), ErrInsufficientFunds (the actor's wallet cannot
// cover the price), ErrNoCharacter/ErrBadCharacter/ErrBadRepository (from
// loading the actor's stored sheet), ErrBadNPC (the target's stored
// inventory bytes are malformed), or ErrSaveFailed with a populated report.
func (m *Manager) Trade(ctx context.Context, in *TradeInput) (*TradeOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("trade: %w", ErrNilInput)
	}
	if in.Actor == "" || in.Target == "" {
		return nil, fmt.Errorf("trade: %w", ErrNoMemberID)
	}
	if len(in.Give.Items) > 0 {
		return nil, fmt.Errorf("trade: %w", ErrGiveNotSupported)
	}
	if len(in.Receive.Items) != 1 {
		return nil, fmt.Errorf("trade: receive must name exactly one item: %w", ErrInvalidTradeOffer)
	}
	item := in.Receive.Items[0]
	if item.ID == "" || item.Quantity <= 0 {
		return nil, fmt.Errorf("trade: %w", ErrInvalidTradeOffer)
	}

	// The price check is a pure computation over the catalog and the
	// caller's own input — no session I/O needed — so it runs here, before
	// openForWrite, alongside the other shape refusals: a wrong price costs
	// nothing to reject, the same as an empty item ID does.
	unitPrice, err := equipment.PriceOf(shared.EquipmentID(item.ID))
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}
	requiredPrice := currency.Money{Copper: unitPrice.Copper * item.Quantity}
	if in.Give.Currency != requiredPrice {
		return nil, fmt.Errorf("trade: %w", ErrWrongPrice)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	confirmed, err := scope.enc.Interact(&encounter.InteractInput{
		Actor:  encounter.MemberID(in.Actor),
		Target: encounter.MemberID(in.Target),
		Range:  in.Range,
	})
	if err != nil {
		return nil, fmt.Errorf("trade: %w", translate(err))
	}
	targetID := string(confirmed.Target)

	idx, err := findWorldNPCIndex(scope.data, targetID)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}
	content := &scope.data.WorldNPCs[idx].NPC
	if !slices.Contains(content.Capabilities, npc.CapabilityVendor) {
		return nil, fmt.Errorf("trade: target %q: %w", in.Target, ErrNotAVendor)
	}

	if err := npcs.DecrementVendorStock(content, item.Type, item.ID, item.Quantity); err != nil {
		return nil, fmt.Errorf("trade: %w: %w", ErrOutOfStock, err)
	}

	actorData, err := m.fetchCharacterData(ctx, "actor", in.Actor)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	newWallet, err := actorData.Wallet.Sub(requiredPrice)
	if err != nil {
		return nil, fmt.Errorf("trade: %w: %w", ErrInsufficientFunds, err)
	}
	actorData.Wallet = newWallet

	if err := character.AddInventoryItem(actorData, character.InventoryItemData{
		Type: item.Type, ID: item.ID, Quantity: item.Quantity,
	}); err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}
	if err := m.saveCharacterRecord(ctx, scope, actorData); err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	recorded, err := scope.enc.Record(&encounter.RecordInput{
		Kind:    encounter.OutcomeTraded,
		Actor:   encounter.MemberID(in.Actor),
		Targets: []encounter.MemberID{encounter.MemberID(in.Target)},
		Trade: &encounter.TradeDetail{
			ItemType: string(item.Type), ItemID: item.ID, Quantity: item.Quantity,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("trade: %w", reportUnrecorded(scope, translate(err)))
	}

	descriptor, err := worldNPCDescriptor(scope.data, targetID)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	return &TradeOutput{
		Descriptor: descriptor,
		Seq:        recorded.Seq,
		Saved:      report,
		Delivery:   delivery,
	}, nil
}
