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

// trade.go is the host seam's half of trading with a placed vendor —
// buying (rpg-project#369, design.md at rpg-project#370) and selling
// (rpg-toolkit#1537, design.md at rpg-project#385/#390), one verb, no new
// RPC. Interact already answers "what does this vendor carry"; this file is
// the verb that actually moves an item, over the same reach/visibility
// check Interact already performs.
//
// ONE DIRECTION PER CALL, still no barter: exactly one of Give.Items or
// Receive.Items is populated. Buying means Receive names the one item and
// Give.Currency is the payment; selling means Give names the one item and
// Receive.Currency is the payout. Neither side ever carries items AND
// currency it doesn't need — Give.Currency must be empty on a sell,
// Receive.Currency must be empty on a buy — and multiple DISTINCT items in
// one call needs validating every line before committing any of them,
// deferred to rpg-toolkit#1275's Quote, which is already a multi-line
// shape. See design.md §4-5 for the full buy reasoning; the sell design's
// own §1-3 for the mirror.
//
// THE SERVER ALONE DECIDES WHETHER AN OFFERED PRICE IS CORRECT — a security
// property, not a style choice (Kirk, 2026-09-05: "we need price validation
// on both sides, absolutely... none of this shit where the client hacks in
// their prices"). Trade computes the required price itself via
// equipment.PriceOf, scaled by quantity, and requires the offered currency
// (Give's on a buy, Receive's on a sell) to match it EXACTLY — no
// tolerance, no partial credit. A client reads a vendor's listed price
// purely to display it and pre-populate what to send; it is never trusted
// as the amount actually charged. The sell design doc restates this as a
// general law, since it applies symmetrically to both directions: "a
// client asserting an inflated expected payout is the same risk class as
// asserting a discounted payment."

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

	// Currency is the coin moving in this offer's direction — the payment
	// on a buy's Give, the payout on a sell's Receive. It must exactly
	// equal the server-computed price of the item changing hands, or the
	// trade is refused (ErrWrongPrice) — see this file's own doc for why an
	// offered amount is never trusted as correct on its own.
	Currency currency.Money
}

// TradeInput names who is trading with whom, over what reach, and what
// moves in each direction.
//
// EXACTLY ONE SHAPE PER CALL: either Give.Items is empty and Receive names
// one item (a buy, Give.Currency the payment), or Receive.Items is empty
// and Give names one item (a sell, Receive.Currency the payout). Both
// populated (barter) or neither (nothing to trade) is refused. The type is
// wider than any one direction needs on purpose — Give/Receive already say
// everything a buy/sell/barter/gift direction field would, so the wire
// contract does not need to change as more directions land — but this verb
// refuses everything outside its two known shapes today rather than
// silently narrowing a caller's request. See design.md §3-5 (buy) and the
// sell design's §1 (sell).
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

	// Give is what Actor hands over: items on a sell, currency on a buy.
	Give TradeOffer

	// Receive is what Actor gets: currency on a sell, items on a buy.
	Receive TradeOffer
}

// TradeOutput reports the descriptor reached (reflecting the stock change)
// and what trading recorded.
type TradeOutput struct {
	// Descriptor is what the target IS now, rebuilt after the stock change —
	// the same shape InteractOutput.Descriptor carries, so a caller can
	// refresh its stock display from this response without a second
	// Interact round trip.
	Descriptor WorldNPCDescriptor

	// Seq is the sequence number of the recorded `bought`/`sold` beat.
	Seq uint64

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// Trade buys from or sells to a placed vendor, confirming reach and
// visibility exactly as Interact does (reusing encounter.Interact as-is —
// design.md §5's own recommendation — so a purchase carries two beats,
// `interacted` then `bought`/`sold`, rather than factoring reach-checking
// into a second private path).
//
// Direction is read from the shape of the input: Receive.Items populated
// means buy, Give.Items populated means sell. Neither, or both, is
// ErrInvalidTradeOffer.
//
// Returns ErrNilInput, ErrNoMemberID (empty actor or target),
// ErrInvalidTradeOffer (neither or both of Give.Items/Receive.Items
// populated, the populated side is not exactly one entry, that entry has
// an empty ID or a nonpositive quantity, or the offer carries currency on
// the side that must stay empty), ErrWrongPrice (the offered currency does
// not exactly equal the server-computed price), ErrNoSessionID,
// ErrNoSession, ErrNoEncounter, ErrClosed, ErrNoMember (actor or target
// missing from the roster, or present but the wrong kind), ErrBadPosition,
// ErrOutOfRange, ErrNotVisible (all forwarded from the underlying Interact
// check), ErrNoSheet (target confirmed as KindWorld but session's own
// WorldNPCs store has nothing recorded for it), ErrNotAVendor (target has
// no npc.CapabilityVendor), ErrOutOfStock (buying: the vendor does not
// carry the item, or not enough of it), ErrNotInInventory (selling: the
// actor does not own enough of the item), ErrInsufficientFunds (the payer's
// wallet — actor buying, vendor selling — cannot cover the price),
// ErrNoCharacter/ErrBadCharacter/ErrBadRepository (from loading the actor's
// stored sheet), ErrBadNPC (the target's stored inventory bytes are
// malformed), or ErrSaveFailed with a populated report.
func (m *Manager) Trade(ctx context.Context, in *TradeInput) (*TradeOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("trade: %w", ErrNilInput)
	}
	if in.Actor == "" || in.Target == "" {
		return nil, fmt.Errorf("trade: %w", ErrNoMemberID)
	}

	giving := len(in.Give.Items) > 0
	receiving := len(in.Receive.Items) > 0
	switch {
	case giving == receiving:
		// Both populated is barter (not this wave); neither is nothing to
		// trade. Same "shape defect, not a smaller-but-legal input"
		// convention encounter.Interact already applies to a negative Range.
		return nil, fmt.Errorf("trade: %w", ErrInvalidTradeOffer)
	case giving:
		return m.sell(ctx, in)
	default:
		return m.buy(ctx, in)
	}
}

// buy is Trade's original direction: Receive names one item, Give.Currency
// pays for it.
func (m *Manager) buy(ctx context.Context, in *TradeInput) (*TradeOutput, error) {
	if len(in.Receive.Items) != 1 {
		return nil, fmt.Errorf("trade: receive must name exactly one item: %w", ErrInvalidTradeOffer)
	}
	item := in.Receive.Items[0]
	if item.ID == "" || item.Quantity <= 0 {
		return nil, fmt.Errorf("trade: %w", ErrInvalidTradeOffer)
	}
	if in.Receive.Currency != (currency.Money{}) {
		return nil, fmt.Errorf("trade: receive currency must be empty on a buy: %w", ErrInvalidTradeOffer)
	}

	price, err := requiredPrice(item)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}
	if in.Give.Currency != price {
		return nil, fmt.Errorf("trade: %w", ErrWrongPrice)
	}

	scope, targetID, content, err := m.openVendorTrade(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	if err := npcs.DecrementVendorStock(content, item.Type, item.ID, item.Quantity); err != nil {
		return nil, fmt.Errorf("trade: %w: %w", ErrOutOfStock, err)
	}

	actorData, err := m.fetchCharacterData(ctx, "actor", in.Actor)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	newWallet, err := actorData.Wallet.Sub(price)
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

	return m.finishTrade(ctx, scope, targetID, &encounter.RecordInput{
		Kind:    encounter.OutcomeBought,
		Actor:   encounter.MemberID(in.Actor),
		Targets: []encounter.MemberID{encounter.MemberID(in.Target)},
		Trade: &encounter.TradeDetail{
			ItemType: string(item.Type), ItemID: item.ID, Quantity: item.Quantity,
		},
	})
}

// sell is Trade's mirror direction (rpg-toolkit#1537): Give names one item
// the actor already owns, Receive.Currency pays the actor for it.
//
// The vendor's own payout is checked and debited BEFORE either side of the
// actor's data is touched — the one gate a vendor's own state can refuse
// (a set, exhausted Wallet), checked first so a refusal there costs nothing
// on the actor's side, the same "reject cheap" reasoning the pre-flight
// price check already applies. AddToVendorStock and the actor's own
// RemoveInventoryItem/Wallet.Add follow; RemoveInventoryItem is the only
// other gate this direction has (the actor not owning enough of the item),
// checked before the effects that cannot themselves fail.
func (m *Manager) sell(ctx context.Context, in *TradeInput) (*TradeOutput, error) {
	if len(in.Give.Items) != 1 {
		return nil, fmt.Errorf("trade: give must name exactly one item: %w", ErrInvalidTradeOffer)
	}
	item := in.Give.Items[0]
	if item.ID == "" || item.Quantity <= 0 {
		return nil, fmt.Errorf("trade: %w", ErrInvalidTradeOffer)
	}
	if in.Give.Currency != (currency.Money{}) {
		return nil, fmt.Errorf("trade: give currency must be empty on a sell: %w", ErrInvalidTradeOffer)
	}

	price, err := requiredPrice(item)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}
	if in.Receive.Currency != price {
		return nil, fmt.Errorf("trade: %w", ErrWrongPrice)
	}

	scope, targetID, content, err := m.openVendorTrade(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	if err := npcs.DebitVendorWallet(content, price); err != nil {
		return nil, fmt.Errorf("trade: %w: %w", ErrInsufficientFunds, err)
	}

	actorData, err := m.fetchCharacterData(ctx, "actor", in.Actor)
	if err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}
	if err := character.RemoveInventoryItem(actorData, character.InventoryItemData{
		Type: item.Type, ID: item.ID, Quantity: item.Quantity,
	}); err != nil {
		return nil, fmt.Errorf("trade: %w: %v", ErrNotInInventory, err)
	}

	if err := npcs.AddToVendorStock(content, item.Type, item.ID, item.Quantity); err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}
	actorData.Wallet = actorData.Wallet.Add(price)

	if err := m.saveCharacterRecord(ctx, scope, actorData); err != nil {
		return nil, fmt.Errorf("trade: %w", err)
	}

	return m.finishTrade(ctx, scope, targetID, &encounter.RecordInput{
		Kind:    encounter.OutcomeSold,
		Actor:   encounter.MemberID(in.Actor),
		Targets: []encounter.MemberID{encounter.MemberID(in.Target)},
		Trade: &encounter.TradeDetail{
			ItemType: string(item.Type), ItemID: item.ID, Quantity: item.Quantity,
		},
	})
}

// requiredPrice is the one server-computed number both directions check an
// offered currency against — a pure computation over the catalog and the
// caller's own input, so it runs before openForWrite alongside the other
// shape refusals: a wrong price costs nothing to reject.
func requiredPrice(item TradeItem) (currency.Money, error) {
	unitPrice, err := equipment.PriceOf(shared.EquipmentID(item.ID))
	if err != nil {
		return currency.Money{}, err
	}
	return currency.Money{Copper: unitPrice.Copper * item.Quantity}, nil
}

// openVendorTrade opens the write scope, confirms reach/visibility via
// Interact, and confirms the target is a placed vendor — the shared first
// half of both buy and sell.
func (m *Manager) openVendorTrade(ctx context.Context, in *TradeInput) (*writeScope, string, *npc.Data, error) {
	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, "", nil, err
	}

	confirmed, err := scope.enc.Interact(&encounter.InteractInput{
		Actor:  encounter.MemberID(in.Actor),
		Target: encounter.MemberID(in.Target),
		Range:  in.Range,
	})
	if err != nil {
		return nil, "", nil, translate(err)
	}
	targetID := string(confirmed.Target)

	idx, err := findWorldNPCIndex(scope.data, targetID)
	if err != nil {
		return nil, "", nil, err
	}
	content := &scope.data.WorldNPCs[idx].NPC
	if !slices.Contains(content.Capabilities, npc.CapabilityVendor) {
		return nil, "", nil, fmt.Errorf("target %q: %w", in.Target, ErrNotAVendor)
	}

	return scope, targetID, content, nil
}

// finishTrade is the shared tail of both directions: record the beat,
// rebuild the descriptor, and commit.
func (m *Manager) finishTrade(
	ctx context.Context, scope *writeScope, targetID string, record *encounter.RecordInput,
) (*TradeOutput, error) {
	recorded, err := scope.enc.Record(record)
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
