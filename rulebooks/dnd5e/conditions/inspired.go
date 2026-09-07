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

// InspiredDie is the die a level-1 bard's inspiration grants. A d6 until the
// bard reaches level 5, and a constant here rather than a computed size
// because nothing in this slice levels a bard up; the day one does, the size
// is carried on the condition's own parameters, which is why [InspiredCondition]
// stores it rather than reading this.
const InspiredDie = "1d6"

// InspiredName is what the die is called wherever a player is shown it — the
// offer's name, the condition catalog's entry, and the label on the button
// that spends it. One constant because those three must not drift.
const InspiredName = "Bardic Inspiration"

// InspiredConditionData is the serializable form of the inspired condition,
// stored by the game server as an opaque JSON blob.
type InspiredConditionData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
	SourceID string    `json:"source_id"`
	Die      string    `json:"die"`
}

// InspiredCondition is a Bardic Inspiration die in somebody's hand.
//
// # It offers, it does not add
//
// The obvious implementation writes the die into AttackChainEvent.AttackBonus,
// the way Archery writes its +2 (fighting_style_archery.go). That chain carries
// an UNSOURCED scalar, so the number would arrive on the roll with nothing
// naming it, and — worse — it would be spent on a roll the player never saw.
// RAW's own boundary is "after rolling the d20 and before the DM says whether
// the roll succeeds or fails", so this condition subscribes to
// [dnd5eEvents.PostRollOfferChain] instead and appends an
// [dnd5eEvents.Offer]. Somebody is then ASKED, and the die is spent when the
// offer is TAKEN.
//
// # Three subscriptions, and the ones that are absent are the design
//
// The offer chain, the taken topic, and the two ends (combat end and rest).
// It never subscribes to AttackChain — see above — and it does not yet fold on
// saving throws or ability checks, which is the rest of Bardic Inspiration as
// written and the same offer on two more chains rather than a second mechanism.
//
// # It ends with the fight, not with a clock
//
// RAW is ten minutes. There is no minute and no round boundary in this
// rulebook's vocabulary (encounter's BoundaryKind is turn_started | turn_ended
// | combat_ended), so the divergence is named rather than approximated: the die
// lasts until the fight ends or the holder rests. Raging's law, and Raging's
// two subscriptions.
type InspiredCondition struct {
	// MemberID is who holds the die.
	MemberID string

	// SourceID is the bard who granted it. Carried so a second grant, a
	// display, or a later "whose inspiration was that" question has an answer
	// that was recorded rather than inferred.
	SourceID string

	// Die is the notation offered — [InspiredDie] at level 1.
	Die string

	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure InspiredCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*InspiredCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (i *InspiredCondition) Ref() *core.Ref { return refs.Conditions.Inspired() }

// NewInspiredCondition creates the held-die condition for one ally. An empty
// die falls back to [InspiredDie]: a condition that offered "" would be an
// offer nothing could roll, and refusing at a constructor a rulebook calls
// internally would push the failure to a place with less to say about it.
func NewInspiredCondition(memberID, sourceID, die string) *InspiredCondition {
	if die == "" {
		die = InspiredDie
	}
	return &InspiredCondition{MemberID: memberID, SourceID: sourceID, Die: die}
}

// IsApplied returns true if this condition is currently applied.
func (i *InspiredCondition) IsApplied() bool { return i.bus != nil }

// Apply subscribes the die to the offer chain, to the taken topic that spends
// it, and to the two boundaries that end it.
func (i *InspiredCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if i.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "inspired condition already applied")
	}
	i.bus = bus

	offers := dnd5eEvents.PostRollOfferChain.On(bus)
	offerSub, err := offers.SubscribeWithChain(ctx, i.onPostRollOffer)
	if err != nil {
		i.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to post-roll offer chain")
	}
	i.subscriptionIDs = append(i.subscriptionIDs, offerSub)

	taken := dnd5eEvents.OfferTakenTopic.On(bus)
	takenSub, err := taken.Subscribe(ctx, i.onOfferTaken)
	if err != nil {
		_ = i.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to offer taken topic")
	}
	i.subscriptionIDs = append(i.subscriptionIDs, takenSub)

	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	combatSub, err := combatEnds.Subscribe(ctx, i.onCombatEnd)
	if err != nil {
		_ = i.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to combat end topic")
	}
	i.subscriptionIDs = append(i.subscriptionIDs, combatSub)

	// Any rest, short or long — Raging's rule, and the honest end of a
	// duration this rulebook has no clock for.
	rests := dnd5eEvents.RestTopic.On(bus)
	restSub, err := rests.Subscribe(ctx, i.onRest)
	if err != nil {
		_ = i.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to rest topic")
	}
	i.subscriptionIDs = append(i.subscriptionIDs, restSub)

	return nil
}

// Remove unsubscribes this condition from all events.
func (i *InspiredCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if i.bus == nil {
		return nil
	}

	total := len(i.subscriptionIDs)
	var errs []error
	for _, subID := range i.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	i.subscriptionIDs = nil
	i.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (i *InspiredCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(InspiredConditionData{
		Ref:      refs.Conditions.Inspired(),
		MemberID: i.MemberID,
		SourceID: i.SourceID,
		Die:      i.Die,
	})
}

// loadJSON loads inspired condition state from JSON.
func (i *InspiredCondition) loadJSON(data json.RawMessage) error {
	var stored InspiredConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal inspired data")
	}
	i.MemberID = stored.MemberID
	i.SourceID = stored.SourceID
	i.Die = stored.Die
	if i.Die == "" {
		i.Die = InspiredDie
	}
	return nil
}

// onPostRollOffer puts the die on the table when the holder's own d20 has been
// rolled. It appends an offer and changes no number: nothing is spent here,
// and a roll nobody answers leaves the die in hand.
func (i *InspiredCondition) onPostRollOffer(
	_ context.Context,
	event *dnd5eEvents.PostRollOfferEvent,
	c chain.Chain[*dnd5eEvents.PostRollOfferEvent],
) (chain.Chain[*dnd5eEvents.PostRollOfferEvent], error) {
	if event == nil || event.AttackerID != i.MemberID {
		return c, nil
	}

	offer := func(
		_ context.Context, e *dnd5eEvents.PostRollOfferEvent,
	) (*dnd5eEvents.PostRollOfferEvent, error) {
		e.Offers = append(e.Offers, dnd5eEvents.Offer{
			Ref:      refs.Conditions.Inspired(),
			Name:     InspiredName,
			Audience: i.MemberID,
			Die:      i.Die,
		})
		return e, nil
	}
	if err := c.Add(combat.StageConditions, "inspired_offer", offer); err != nil {
		return c, rpgerr.Wrapf(err, "failed to offer inspiration die for member %s", i.MemberID)
	}
	return c, nil
}

// onOfferTaken spends the die, because it was taken. The face is not this
// condition's to roll — the machine that applied it rolled it and reports it
// here, exactly as the movement machine reports a reaction it ran.
//
// Unsubscribing mid-dispatch is safe: the bus snapshots subscribers before
// invoking handlers (see HelpedCondition.onAttackChain).
func (i *InspiredCondition) onOfferTaken(ctx context.Context, event dnd5eEvents.OfferTakenEvent) error {
	if event.Audience != i.MemberID {
		return nil
	}
	if event.Ref == nil || event.Ref.ID != refs.Conditions.Inspired().ID {
		return nil
	}
	return i.end(ctx, "spent")
}

// onCombatEnd ends the die with the fight — the divergence from RAW's ten
// minutes, named on the type.
func (i *InspiredCondition) onCombatEnd(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
	if event.SubjectID != i.MemberID {
		return nil
	}
	return i.end(ctx, "combat ended")
}

// onRest ends the die on any rest, short or long, for the holder.
func (i *InspiredCondition) onRest(ctx context.Context, event dnd5eEvents.RestEvent) error {
	if event.CharacterID != i.MemberID {
		return nil
	}
	return i.end(ctx, "rest")
}

// end publishes the removal and detaches. The publish comes first so the
// removal is on the bus while this condition is still the thing that owned the
// die, which is what an activation's effect collector reads.
func (i *InspiredCondition) end(ctx context.Context, reason string) error {
	if i.bus == nil {
		return nil
	}
	bus := i.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     i.MemberID,
		ConditionRef: refs.Conditions.Inspired().String(),
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish inspired removal for member %s", i.MemberID)
	}
	return i.Remove(ctx, bus)
}
