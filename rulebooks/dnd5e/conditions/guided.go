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
	// GuidedName is the display name for a creature holding a Guidance die.
	GuidedName = "Guidance"
	// GuidedDie is the die Guidance grants — 1d4 at every level (no scaling).
	GuidedDie = "1d4"
)

// GuidedConditionData is the persisted source-qualified Guidance condition.
type GuidedConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewGuidedConditionInput names the recipient, caster, and canonical spell
// that created a Guided condition.
type NewGuidedConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// GuidedCondition is a Guidance die in somebody's hand, offered on their next
// ability check rather than added to it — the same "offer, don't modify"
// shape [InspiredCondition] uses for attacks, folded on
// [dnd5eEvents.PostCheckRollOfferChain] instead: RAW's own boundary is
// identical ("It can roll the die before or after making the ability
// check"), just for checks instead of attacks.
//
// # It ends like Bless, not like Bardic Inspiration
//
// Guidance is concentration, up to one minute — the same duration category
// as [BlessedCondition], not Bardic Inspiration's ten-minutes-or-combat-end.
// So this condition subscribes to long-rest cleanup (Bless's precedent) and
// relies on the existing concentration teardown for everything else — no
// combat-end subscription of its own, unlike [InspiredCondition].
//
// # Three subscriptions: the offer, the take, and the long rest
//
// No skill filter: RAW says "one ability check of its choice," so the offer
// fires on whichever check the recipient next makes, not a particular skill.
type GuidedCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	bus             events.EventBus
	restSubID       string
	subscriptionIDs []string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*GuidedCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*GuidedCondition)(nil)
)

// NewGuidedCondition creates one source-qualified Guidance effect.
func NewGuidedCondition(input NewGuidedConditionInput) (*GuidedCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "guided condition requires a recipient member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "guided condition requires a caster source id")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.Guidance().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "guided condition source ref must be Guidance")
	}

	return &GuidedCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.Guidance(),
	}, nil
}

// Ref returns the canonical Guided condition ref.
func (g *GuidedCondition) Ref() *core.Ref { return refs.Conditions.Guided() }

// ConditionAddress derives this condition's exact identity from its
// persisted state.
func (g *GuidedCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     g.MemberID,
		ConditionRef: g.Ref().String(),
		SourceID:     g.SourceID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (g *GuidedCondition) IsApplied() bool { return g.bus != nil }

// Apply subscribes the die to the check-offer chain, to the taken topic that
// spends it, and to long-rest cleanup.
func (g *GuidedCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if g.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "guided condition already applied")
	}
	g.bus = bus

	offers := dnd5eEvents.PostCheckRollOfferChain.On(bus)
	offerSub, err := offers.SubscribeWithChain(ctx, g.onPostCheckRollOffer)
	if err != nil {
		g.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to post-check-roll offer chain")
	}
	g.subscriptionIDs = append(g.subscriptionIDs, offerSub)

	taken := dnd5eEvents.OfferTakenTopic.On(bus)
	takenSub, err := taken.Subscribe(ctx, g.onOfferTaken)
	if err != nil {
		_ = g.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to offer taken topic")
	}
	g.subscriptionIDs = append(g.subscriptionIDs, takenSub)

	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: g.ConditionAddress(), Remove: g.Remove,
	})
	if err != nil {
		_ = g.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe guided condition to long rest")
	}
	g.restSubID = restSubID

	return nil
}

// Remove unsubscribes this condition from every event it joined.
func (g *GuidedCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if g.bus == nil {
		return nil
	}

	ids := g.subscriptionIDs
	if g.restSubID != "" {
		ids = append(ids, g.restSubID)
	}
	total := len(ids)
	var errs []error
	for _, subID := range ids {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	g.subscriptionIDs = nil
	g.restSubID = ""
	g.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON serializes the source-qualified condition without runtime state.
func (g *GuidedCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(GuidedConditionData{
		Ref:       refs.Conditions.Guided(),
		MemberID:  g.MemberID,
		SourceID:  g.SourceID,
		SourceRef: refs.Spells.Guidance(),
	})
}

func (g *GuidedCondition) loadJSON(data json.RawMessage) error {
	var stored GuidedConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal guided data")
	}
	g.MemberID = stored.MemberID
	g.SourceID = stored.SourceID
	g.SourceRef = refs.Spells.Guidance()
	return nil
}

// onPostCheckRollOffer puts the die on the table when the holder's own
// ability-check d20 has been rolled. Nothing is spent here.
func (g *GuidedCondition) onPostCheckRollOffer(
	_ context.Context,
	event *dnd5eEvents.PostCheckRollOfferEvent,
	c chain.Chain[*dnd5eEvents.PostCheckRollOfferEvent],
) (chain.Chain[*dnd5eEvents.PostCheckRollOfferEvent], error) {
	if event == nil || event.CheckerID != g.MemberID {
		return c, nil
	}

	offer := func(
		_ context.Context, e *dnd5eEvents.PostCheckRollOfferEvent,
	) (*dnd5eEvents.PostCheckRollOfferEvent, error) {
		e.Offers = append(e.Offers, dnd5eEvents.Offer{
			Ref: refs.Conditions.Guided(), Name: GuidedName, Audience: g.MemberID, Die: GuidedDie,
		})
		return e, nil
	}
	if err := c.Add(combat.StageConditions, "guided_offer", offer); err != nil {
		return c, rpgerr.Wrapf(err, "failed to offer guidance die for member %s", g.MemberID)
	}
	return c, nil
}

// onOfferTaken spends the die, because it was taken. The face is not this
// condition's to roll — the machine that resolved the check rolled it and
// reports it here, exactly as [InspiredCondition.onOfferTaken] does.
func (g *GuidedCondition) onOfferTaken(ctx context.Context, event dnd5eEvents.OfferTakenEvent) error {
	if event.Audience != g.MemberID {
		return nil
	}
	if event.Ref == nil || event.Ref.ID != refs.Conditions.Guided().ID {
		return nil
	}
	return g.end(ctx, "spent")
}

// end publishes the removal and detaches. The publish comes first so the
// removal is on the bus while this condition is still the thing that owned
// the die, which is what an activation's effect collector reads.
func (g *GuidedCondition) end(ctx context.Context, reason string) error {
	if g.bus == nil {
		return nil
	}
	bus := g.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     g.MemberID,
		ConditionRef: refs.Conditions.Guided().String(),
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish guided removal for member %s", g.MemberID)
	}
	return g.Remove(ctx, bus)
}
