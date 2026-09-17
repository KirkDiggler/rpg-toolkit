// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const (
	// ResistanceName is the display name for a creature holding a Resistance die.
	ResistanceName = "Resistance"
	// ResistanceDie is the die Resistance grants — 1d4 at every level (no scaling).
	ResistanceDie = "1d4"
)

// ResistanceConditionData is the persisted source-qualified Resistance condition.
type ResistanceConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewResistanceConditionInput names the recipient, caster, and canonical
// spell that created a Resistance condition.
type NewResistanceConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// ResistanceCondition is a Resistance die in somebody's hand, offered on
// their next saving throw rather than added to it — [GuidedCondition]'s
// "offer, don't modify" shape, folded on
// [dnd5eEvents.PostSaveRollOfferChain] instead: RAW's own boundary is
// identical ("you can roll the die and add the number rolled to one saving
// throw of your choice ... before or after making the save"), just for
// saves instead of checks.
//
// # It ends like Bless and Guidance, not like Bardic Inspiration
//
// Resistance is concentration, up to one minute — the same duration
// category as [BlessedCondition] and [GuidedCondition], not Bardic
// Inspiration's ten-minutes-or-combat-end. So this condition subscribes to
// long-rest cleanup (Bless's precedent) and relies on the existing
// concentration teardown for everything else — no combat-end subscription
// of its own, unlike [InspiredCondition].
//
// # Three subscriptions: the offer, the take, and the long rest
//
// No ability filter: RAW says "one saving throw of its choice," so the
// offer fires on whichever save the recipient next makes, not a particular
// ability.
type ResistanceCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	bus             events.EventBus
	restSubID       string
	subscriptionIDs []string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*ResistanceCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*ResistanceCondition)(nil)
)

// NewResistanceCondition creates one source-qualified Resistance effect.
func NewResistanceCondition(input NewResistanceConditionInput) (*ResistanceCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "resistance condition requires a recipient member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "resistance condition requires a caster source id")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.Resistance().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "resistance condition source ref must be Resistance")
	}

	return &ResistanceCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.Resistance(),
	}, nil
}

// Ref returns the canonical Resistance condition ref.
func (r *ResistanceCondition) Ref() *core.Ref { return refs.Conditions.Resistance() }

// ConditionAddress derives this condition's exact identity from its
// persisted state.
func (r *ResistanceCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     r.MemberID,
		ConditionRef: r.Ref().String(),
		SourceID:     r.SourceID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (r *ResistanceCondition) IsApplied() bool { return r.bus != nil }

// Apply subscribes the die to the save-offer chain, to the taken topic that
// spends it, and to long-rest cleanup.
func (r *ResistanceCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if r.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "resistance condition already applied")
	}
	r.bus = bus

	offers := dnd5eEvents.PostSaveRollOfferChain.On(bus)
	offerSub, err := offers.SubscribeWithChain(ctx, r.onPostSaveRollOffer)
	if err != nil {
		r.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to post-save-roll offer chain")
	}
	r.subscriptionIDs = append(r.subscriptionIDs, offerSub)

	taken := dnd5eEvents.OfferTakenTopic.On(bus)
	takenSub, err := taken.Subscribe(ctx, r.onOfferTaken)
	if err != nil {
		_ = r.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to offer taken topic")
	}
	r.subscriptionIDs = append(r.subscriptionIDs, takenSub)

	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: r.ConditionAddress(), Remove: r.Remove,
	})
	if err != nil {
		_ = r.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe resistance condition to long rest")
	}
	r.restSubID = restSubID

	return nil
}

// Remove unsubscribes this condition from every event it joined.
func (r *ResistanceCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if r.bus == nil {
		return nil
	}

	ids := r.subscriptionIDs
	if r.restSubID != "" {
		ids = append(ids, r.restSubID)
	}
	total := len(ids)
	var errs []error
	for _, subID := range ids {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	r.subscriptionIDs = nil
	r.restSubID = ""
	r.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON serializes the source-qualified condition without runtime state.
func (r *ResistanceCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(ResistanceConditionData{
		Ref:       refs.Conditions.Resistance(),
		MemberID:  r.MemberID,
		SourceID:  r.SourceID,
		SourceRef: refs.Spells.Resistance(),
	})
}

func (r *ResistanceCondition) loadJSON(data json.RawMessage) error {
	var stored ResistanceConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal resistance data")
	}
	r.MemberID = stored.MemberID
	r.SourceID = stored.SourceID
	r.SourceRef = refs.Spells.Resistance()
	return nil
}

// onPostSaveRollOffer puts the die on the table when the holder's own
// saving-throw d20 has been rolled. Nothing is spent here.
func (r *ResistanceCondition) onPostSaveRollOffer(
	_ context.Context,
	event *dnd5eEvents.PostSaveRollOfferEvent,
	c chain.Chain[*dnd5eEvents.PostSaveRollOfferEvent],
) (chain.Chain[*dnd5eEvents.PostSaveRollOfferEvent], error) {
	if event == nil || event.SaverID != r.MemberID {
		return c, nil
	}

	offer := func(
		_ context.Context, e *dnd5eEvents.PostSaveRollOfferEvent,
	) (*dnd5eEvents.PostSaveRollOfferEvent, error) {
		e.Offers = append(e.Offers, dnd5eEvents.Offer{
			Ref: refs.Conditions.Resistance(), Name: ResistanceName, Audience: r.MemberID,
			Die: ResistanceDie, SourceID: r.SourceID,
		})
		return e, nil
	}
	if err := c.Add(combat.StageConditions, "resistance_offer", offer); err != nil {
		return c, rpgerr.Wrapf(err, "failed to offer resistance die for member %s", r.MemberID)
	}
	return c, nil
}

// onOfferTaken spends the die, because it was taken. The face is not this
// condition's to roll — the machine that resolved the save rolled it and
// reports it here, exactly as [GuidedCondition.onOfferTaken] does.
func (r *ResistanceCondition) onOfferTaken(ctx context.Context, event dnd5eEvents.OfferTakenEvent) error {
	if event.Audience != r.MemberID {
		return nil
	}
	if event.Ref == nil || event.Ref.ID != refs.Conditions.Resistance().ID {
		return nil
	}
	return r.end(ctx, "spent")
}

// end publishes the removal and detaches. The publish comes first so the
// removal is on the bus while this condition is still the thing that owned
// the die, which is what an activation's effect collector reads.
func (r *ResistanceCondition) end(ctx context.Context, reason string) error {
	if r.bus == nil {
		return nil
	}
	bus := r.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     r.MemberID,
		ConditionRef: refs.Conditions.Resistance().String(),
		SourceID:     r.SourceID,
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish resistance removal for member %s", r.MemberID)
	}
	return r.Remove(ctx, bus)
}
