// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ConcentratingName is what a player is shown wherever this condition appears.
const ConcentratingName = "Concentrating"

// The six reasons concentration ends. They are a rulebook vocabulary rather
// than a wire contract: the removal fact carries a plain string and a record
// reads it, so a seventh reason costs nothing but this list.
const (
	// ConcentrationEndedDamage is a failed Constitution check after damage.
	ConcentrationEndedDamage = "damage"

	// ConcentrationEndedRecast is the caster casting another concentration
	// spell. Named here because the vocabulary is the rulebook's; the drop
	// itself belongs to the machine that resolves the new cast.
	ConcentrationEndedRecast = "recast"

	// ConcentrationEndedDuration is the spell's own clock running out.
	ConcentrationEndedDuration = "duration"

	// ConcentrationEndedCombatEnd is the fight ending under it.
	ConcentrationEndedCombatEnd = "combat_end"

	// ConcentrationEndedSpellEnded is the last child ending on its own — the
	// spell is over, so holding it together is over with it.
	ConcentrationEndedSpellEnded = "spell_ended"

	// ConcentrationEndedCasterDown is the caster dropping to 0 hit points. A
	// caster at 0 does not roll to keep a spell.
	ConcentrationEndedCasterDown = "caster_down"
)

// endsTheHold reports whether a removal carrying this reason is the hold
// itself ending, rather than one child ending on its own.
//
// It is the difference between "my True Strike was consumed, so the spell is
// over" and "the spell ended, so my True Strike came off". Both arrive as the
// same fact with a different reason, and only the second must not be read as
// the last child leaving — otherwise a strip would end the hold a second time
// with the wrong reason attached.
func endsTheHold(reason string) bool {
	switch reason {
	case ConcentrationEndedDamage, ConcentrationEndedRecast, ConcentrationEndedDuration,
		ConcentrationEndedCombatEnd, ConcentrationEndedSpellEnded, ConcentrationEndedCasterDown:
		return true
	default:
		return false
	}
}

// ConcentrationDCFloor is the lowest a concentration check can ask for. RAW
// 2014: DC 10 or half the damage taken, whichever is higher.
const ConcentrationDCFloor = 10

// ConcentrationDC is the check a caster owes after taking damage:
// max(10, floor(damage / 2)).
//
// A FUNCTION HERE RATHER THAN A DCSource, because a DCSource exists for a DC
// the machine derives at resolution time from what it is holding. Nothing here
// is derived at resolution time: the condition was handed the applied amount
// by the damage fact and settles the number in its own handler, and the
// settled number rides the gate as a static DC. Arithmetic is description, not
// rolling.
func ConcentrationDC(damageTaken int) int {
	if damageTaken < 0 {
		// Negative damage is not a thing 5e has, and a floor that came out
		// below itself because someone passed -4 would be worse than the clamp.
		damageTaken = 0
	}
	half := damageTaken / 2 // damage is never negative here, so this floors
	if half < ConcentrationDCFloor {
		return ConcentrationDCFloor
	}
	return half
}

// ConcentratingConditionData is the serializable form of the concentrating
// condition, stored by the game server as an opaque JSON blob.
type ConcentratingConditionData struct {
	Ref          *core.Ref              `json:"ref"`
	MemberID     string                 `json:"member_id"`
	SpellRef     string                 `json:"spell_ref"`
	SpellName    string                 `json:"spell_name"`
	TurnEndsLeft int                    `json:"turn_ends_left"`
	Children     []dnd5eEvents.ChildRef `json:"children,omitempty"`
}

// ConcentratingCondition is one caster holding one spell together, and the
// owner of everything that spell left on the board.
//
// # It sits on the CASTER, and it owns the effects, not the other way round
//
// A concentration spell puts its effects wherever the spell says — the
// caster's own sheet, a target's, several targets' — and this condition holds
// their ADDRESSES. When it ends, they end with it: one removal fact per
// address, honoured by each member's own keeper. That indirection is the whole
// design. A pointer to another member's condition object could not survive the
// blob round trip through the sheet, and reaching across to delete something
// from a sheet this condition does not own is not a thing the toolkit does.
//
// # It ends six ways, and only one of them is a roll
//
// Damage provokes a Constitution check ([ConcentrationDC]) which this condition
// DESCRIBES and a machine rolls; the caster dropping to 0 ends it with no check
// at all; its own clock runs out; the fight ends; its last child ends; or
// another concentration cast displaces it.
//
// # What it never does is roll
//
// The damage handler appends a [dnd5eEvents.FollowUp] — data, with a settled
// DC and a consequence — to the fact it just heard, and stops. A subscriber
// that rolled here would put a rules decision in a subscription order and hide
// the roll from the record.
type ConcentratingCondition struct {
	// MemberID is the caster holding the spell together.
	MemberID string

	// SpellRef is what is being concentrated on, as a ref string.
	SpellRef string

	// SpellName is what to call it in a log or a badge, so a display never
	// has to turn a ref back into English.
	SpellName string

	// TurnEndsLeft is how many of the caster's turn ends remain before the
	// spell's own duration runs out. ONE CLOCK, and it is this one: the
	// children do not count their own.
	TurnEndsLeft int

	// Children are the addresses of the effects this spell left behind.
	Children []dnd5eEvents.ChildRef

	bus             events.EventBus
	subscriptionIDs []string

	// ending guards the re-entrant walk through [ConcentratingCondition.end]:
	// publishing a child's removal reaches this condition's own removal
	// handler, which would otherwise empty the list and end it again from
	// inside itself.
	ending bool
}

// Ensure ConcentratingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*ConcentratingCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (c *ConcentratingCondition) Ref() *core.Ref { return refs.Conditions.Concentrating() }

// NewConcentratingCondition creates the caster's hold on one spell, good for
// turnEnds of the caster's turn ends unless something takes it first.
//
// The children are added as the spell delivers them, through
// [ConcentratingCondition.AddChild]: what a cast leaves behind is known when it
// lands, not when the profile is read.
func NewConcentratingCondition(casterID, spellRef, spellName string, turnEnds int) *ConcentratingCondition {
	return &ConcentratingCondition{
		MemberID:     casterID,
		SpellRef:     spellRef,
		SpellName:    spellName,
		TurnEndsLeft: turnEnds,
	}
}

// AddChild records one effect this spell left on the board, so ending the
// concentration ends that effect too.
//
// It states the change on the bus when applied, because the child list is
// persisted with the sheet and a change nobody hears about is a change
// discarded at save time. A duplicate address is ignored rather than doubled:
// the list is a set of addresses, and publishing one removal twice would ask
// two keepers to drop the same thing.
func (c *ConcentratingCondition) AddChild(ctx context.Context, child dnd5eEvents.ChildRef) error {
	if child.MemberID == "" || child.ConditionRef == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "concentration child needs a member and a condition ref")
	}
	for _, existing := range c.Children {
		if existing == child {
			return nil
		}
	}
	c.Children = append(c.Children, child)

	if c.bus == nil {
		return nil
	}
	return publishStateChanged(ctx, c.bus, c.MemberID, c.Ref())
}

// IsApplied returns true if this condition is currently applied.
func (c *ConcentratingCondition) IsApplied() bool { return c.bus != nil }

// Apply subscribes the hold to the one thing that threatens it and the three
// boundaries that end it: damage taken, the caster's turn ends, the end of the
// fight, and its own children ending.
//
// EVERY HANDLER CLOSES OVER THE BUS IT WAS SUBSCRIBED WITH, the way both sheet
// keepers hand their own handlers a bus rather than reading one off the sheet.
// Here it is load-bearing rather than tidy: the bus snapshots its subscribers
// before invoking them, so a handler still runs after something detached it
// mid-dispatch — and a keeper pruning this hold does exactly that. A handler
// that read c.bus would find nil and answer nothing, which is a rules decision
// made by subscription order.
func (c *ConcentratingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if c.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "concentrating condition already applied")
	}
	c.bus = bus

	damageTaken := dnd5eEvents.DamageTakenTopic.On(bus)
	damageSub, err := damageTaken.Subscribe(ctx,
		func(ctx context.Context, event *dnd5eEvents.DamageTakenEvent) error {
			return c.onDamageTaken(ctx, bus, event)
		})
	if err != nil {
		c.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to damage taken topic")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, damageSub)

	turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
	turnSub, err := turnEnds.Subscribe(ctx,
		func(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
			return c.onTurnEnd(ctx, bus, event)
		})
	if err != nil {
		_ = c.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn end topic")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, turnSub)

	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	combatSub, err := combatEnds.Subscribe(ctx,
		func(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
			return c.onCombatEnd(ctx, bus, event)
		})
	if err != nil {
		_ = c.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to combat end topic")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, combatSub)

	conditionsRemoved := dnd5eEvents.ConditionRemovedTopic.On(bus)
	removedSub, err := conditionsRemoved.Subscribe(ctx,
		func(ctx context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			return c.onConditionRemoved(ctx, bus, event)
		})
	if err != nil {
		_ = c.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to condition removed topic")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, removedSub)

	// A rest is not one of this condition's own ends — its clock and combat
	// end both fire first in any ordinary fight — but a blob that survived to
	// a long rest must not outlive it, which is the registry every
	// combat-scoped condition here is in.
	restSub, err := subscribeRemoveOnLongRest(ctx, bus, c.MemberID, c.Ref(), c.Remove)
	if err != nil {
		_ = c.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, restSub)

	return nil
}

// Remove unsubscribes this condition from all events.
func (c *ConcentratingCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if c.bus == nil {
		return nil
	}

	total := len(c.subscriptionIDs)
	var errs []error
	for _, subID := range c.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	c.subscriptionIDs = nil
	c.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (c *ConcentratingCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(ConcentratingConditionData{
		Ref:          refs.Conditions.Concentrating(),
		MemberID:     c.MemberID,
		SpellRef:     c.SpellRef,
		SpellName:    c.SpellName,
		TurnEndsLeft: c.TurnEndsLeft,
		Children:     c.Children,
	})
}

// loadJSON loads concentrating condition state from JSON.
func (c *ConcentratingCondition) loadJSON(data json.RawMessage) error {
	var stored ConcentratingConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal concentrating data")
	}
	c.MemberID = stored.MemberID
	c.SpellRef = stored.SpellRef
	c.SpellName = stored.SpellName
	c.TurnEndsLeft = stored.TurnEndsLeft
	if c.TurnEndsLeft <= 0 {
		// The same reading True Strike's counter takes: a blob whose count ran
		// out without the removal landing is honestly "one more turn end", not
		// "already expired". A hold that vanished on load would take its
		// children's owner with it silently.
		c.TurnEndsLeft = 1
	}
	c.Children = stored.Children
	return nil
}

// onDamageTaken answers the damage fact with the check the caster owes, or
// ends the hold outright when the caster went down.
//
// IT APPENDS AND STOPS. The follow-up carries a settled DC and a consequence
// and nothing that could produce a number; the machine that published the fact
// is what runs it.
func (c *ConcentratingCondition) onDamageTaken(
	ctx context.Context, bus events.EventBus, event *dnd5eEvents.DamageTakenEvent,
) error {
	if event == nil || event.MemberID != c.MemberID {
		return nil
	}

	if event.DroppedToZero {
		// A caster at 0 hit points does not roll to keep a spell, so this is
		// not a check that auto-fails — it is no check at all, and no save
		// beat in the record.
		return c.end(ctx, bus, ConcentrationEndedCasterDown)
	}

	spellRef, err := core.ParseString(c.SpellRef)
	if err != nil {
		// Fail closed and loudly: a concentration check whose cause could not
		// name the spell would be a save in the record with nothing at stake.
		return rpgerr.Wrapf(err, "concentrating condition holds an unparseable spell ref %q", c.SpellRef)
	}

	event.FollowUps = append(event.FollowUps, dnd5eEvents.FollowUp{
		SaverID: c.MemberID,
		Ability: abilities.CON,
		DC:      ConcentrationDC(event.Amount),
		Cause: dnd5eEvents.SaveCause{
			Trigger:        dnd5eEvents.SaveTriggerConcentration,
			EffectRef:      spellRef,
			InstigatorID:   event.Cause.InstigatorID,
			InstigatorType: event.Cause.InstigatorType,
		},
		OnFailure: dnd5eEvents.Consequence{
			Remove: append([]dnd5eEvents.ChildRef(nil), c.Children...),
			Owner: dnd5eEvents.ChildRef{
				MemberID:     c.MemberID,
				ConditionRef: c.Ref().String(),
			},
			Reason: ConcentrationEndedDamage,
		},
	})

	return nil
}

// onTurnEnd counts down the spell's own duration and ends the hold when the
// count runs out.
func (c *ConcentratingCondition) onTurnEnd(
	ctx context.Context, bus events.EventBus, event dnd5eEvents.TurnEndEvent,
) error {
	if event.SubjectID != c.MemberID || c.ending {
		return nil
	}

	c.TurnEndsLeft--
	if c.TurnEndsLeft > 0 {
		return publishStateChanged(ctx, bus, c.MemberID, c.Ref())
	}

	return c.end(ctx, bus, ConcentrationEndedDuration)
}

// onCombatEnd ends the hold with the fight.
func (c *ConcentratingCondition) onCombatEnd(
	ctx context.Context, bus events.EventBus, event dnd5eEvents.CombatEndEvent,
) error {
	if event.SubjectID != c.MemberID {
		return nil
	}
	return c.end(ctx, bus, ConcentrationEndedCombatEnd)
}

// onConditionRemoved answers two different facts on one topic: a child of this
// hold ending, and this hold itself being ended by somebody else.
//
// The second is not hypothetical and it is not rare. Three of the six reasons
// reach this condition as a removal published elsewhere — a failed check's
// consequence, the recast drop, and the long-rest net — because the thing that
// decided is the thing that publishes. So a removal addressed to this
// condition is an END, honoured here with the reason it arrived carrying.
//
// A child leaving is the other half: dropped from the list, and when the last
// one goes on its OWN account the spell is over ([ConcentrationEndedSpellEnded]).
// Without that, a bard whose True Strike was consumed still reads as
// concentrating and still drops "nothing" on the next concentration cast.
func (c *ConcentratingCondition) onConditionRemoved(
	ctx context.Context, bus events.EventBus, event dnd5eEvents.ConditionRemovedEvent,
) error {
	if c.ending {
		// One of the removals we are publishing right now. The list is already
		// spoken for.
		return nil
	}

	if event.MemberID == c.MemberID && event.ConditionRef == c.Ref().String() {
		// Somebody else ended this hold. They published the owner's fact, so
		// this takes the children and the reason and does not republish it.
		return c.endFromFact(ctx, bus, event.Reason)
	}

	removed := dnd5eEvents.ChildRef{MemberID: event.MemberID, ConditionRef: event.ConditionRef}
	kept := make([]dnd5eEvents.ChildRef, 0, len(c.Children))
	for _, child := range c.Children {
		if child != removed {
			kept = append(kept, child)
		}
	}
	if len(kept) == len(c.Children) {
		return nil
	}

	hadChildren := len(c.Children) > 0
	c.Children = kept
	if len(c.Children) == 0 && hadChildren && !endsTheHold(event.Reason) {
		return c.end(ctx, bus, ConcentrationEndedSpellEnded)
	}

	return publishStateChanged(ctx, bus, c.MemberID, c.Ref())
}

// end publishes one removal per child address, then the fact that says WHY,
// then its own removal, and detaches.
//
// THE CHILDREN COME OFF FIRST, while their owner still names them: each
// address is a fact on the bus for that member's own keeper to honour. The
// reason rides [dnd5eEvents.ConcentrationEndedEvent] rather than the removals,
// because a condition-removed landing on a skeleton's sheet with no cast beat
// near it reads as a random drop. The owner's own removal goes last, which is
// what says the hold itself is over.
func (c *ConcentratingCondition) end(ctx context.Context, bus events.EventBus, reason string) error {
	if bus == nil || c.ending {
		return nil
	}
	c.ending = true
	defer func() { c.ending = false }()

	if err := c.strip(ctx, bus, reason); err != nil {
		return err
	}

	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     c.MemberID,
		ConditionRef: c.Ref().String(),
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish concentration removal for member %s", c.MemberID)
	}

	return c.Remove(ctx, bus)
}

// endFromFact is [ConcentratingCondition.end] for a hold ended by somebody
// else: the owner's removal is already on the bus, so this takes the children
// and states the reason rather than republishing a fact that has been
// published.
//
// The one ordering difference is forced rather than chosen. On this path the
// owner's removal was published before this condition heard anything, so it
// leads rather than trails — the publisher owns the order of its own facts,
// and what this guarantees is the part it can: exactly one
// [dnd5eEvents.ConcentrationEndedEvent], carrying the reason and every address
// that came off.
func (c *ConcentratingCondition) endFromFact(ctx context.Context, bus events.EventBus, reason string) error {
	if bus == nil || c.ending {
		return nil
	}
	c.ending = true
	defer func() { c.ending = false }()

	if err := c.strip(ctx, bus, reason); err != nil {
		return err
	}

	return c.Remove(ctx, bus)
}

// strip publishes one removal per child address and then the one fact that
// says why the hold ended. Shared by both end paths so the fact cannot be
// published twice, or forgotten on one of them.
func (c *ConcentratingCondition) strip(ctx context.Context, bus events.EventBus, reason string) error {
	removals := dnd5eEvents.ConditionRemovedTopic.On(bus)
	removed := c.Children
	for _, child := range removed {
		if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
			MemberID:     child.MemberID,
			ConditionRef: child.ConditionRef,
			Reason:       reason,
		}); err != nil {
			return rpgerr.Wrapf(err, "failed to publish %s removal for member %s",
				child.ConditionRef, child.MemberID)
		}
	}
	c.Children = nil

	if err := dnd5eEvents.ConcentrationEndedTopic.On(bus).Publish(ctx,
		dnd5eEvents.ConcentrationEndedEvent{
			CasterID:  c.MemberID,
			SpellRef:  c.SpellRef,
			SpellName: c.SpellName,
			Reason:    reason,
			Removed:   removed,
		}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish concentration ended for member %s", c.MemberID)
	}

	return nil
}
