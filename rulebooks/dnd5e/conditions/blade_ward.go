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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// BladeWardName is what a player is shown wherever this condition appears.
const BladeWardName = "Blade Ward"

// BladeWardConditionData is the serializable form of the blade ward condition,
// stored by the game server as an opaque JSON blob.
type BladeWardConditionData struct {
	Ref          *core.Ref `json:"ref"`
	MemberID     string    `json:"member_id"`
	SourceRef    string    `json:"source_ref"`
	TurnEndsLeft int       `json:"turn_ends_left"`
}

// BladeWardCondition is the sigil a caster traces in front of themselves, and
// the only thing in this rulebook that is purely defensive.
//
// # It sits on the CASTER and reads the DEFENDER side
//
// Blade Ward is Range: Self, so the ward and the warded creature are the same
// member. The condition tests event.TargetID rather than AttackerID for that
// reason — it is interested in swings coming IN.
//
// # It halves weapon damage the way rage does, and only weapon damage
//
// Resistance in this rulebook is not a flag or a sheet field: it is a
// [dnd5eEvents.DamageComponent] carrying Multiply(0.5) appended at
// [combat.StageFinal], folded into a number by combat.FinalDamage. Rage was the
// first tenant of that seat; this is the second.
//
// The scope is narrower than rage's, and deliberately so. Rage resists all
// bludgeoning, piercing and slashing whatever dealt it; Blade Ward's RAW scope
// is damage "dealt by weapon attacks", which is expressible because each
// component is stamped [dnd5eEvents.DamageSourceWeapon] or DamageSourceSpell by
// whoever rolled it. Copying rage's broader predicate would be correct TODAY
// only by accident — a cast's damage skips the chain fold entirely, so a strike
// is currently the only thing that could over-resist — and accidental
// correctness is the kind that breaks quietly later.
//
// # One limit, named rather than discovered
//
// A Multiplier scales every component of ITS DAMAGE TYPE, not the single
// component it was derived from. So an event carrying weapon slashing AND
// spell slashing at once would have both halved. Nothing produces that today
// and the alternative — skipping the ward whenever a type is mixed — would
// under-resist instead, which is not more correct. Recorded here so the next
// person meets it as a decision rather than a surprise.
//
// # It holds its own clock, because nothing else does
//
// True Strike is a concentration cantrip and lets the caster's
// [ConcentratingCondition] own its duration. Blade Ward is not concentration,
// so the count lives here. Two turn ends rather than one for the reason
// spells.TrueStrikeTurnEnds is also two: a cantrip costs an action, so the ward
// is always traced DURING the caster's own turn, and the very next turn end on
// the bus is that same turn's. Ending there would mean the ward never survived
// to see a swing.
type BladeWardCondition struct {
	// MemberID is the warded creature, who is also the caster.
	MemberID string

	// SourceRef is what granted this, as a ref string — the spell, not the
	// caster.
	SourceRef string

	// TurnEndsLeft is how many of the warded creature's turn ends remain.
	// Counted here rather than derived from a round, because a TurnEndEvent
	// may legitimately carry no round at all and Round 0 means "unknown"
	// rather than "round zero" — the distinction conditions/raging.go paid for
	// once already.
	TurnEndsLeft int

	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure BladeWardCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*BladeWardCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (b *BladeWardCondition) Ref() *core.Ref { return refs.Conditions.BladeWard() }

// NewBladeWardCondition creates the ward on one creature for a given number of
// its own turn ends.
//
// An empty sourceRef falls back to the Blade Ward spell, which is the only
// thing in this rulebook that applies this condition.
func NewBladeWardCondition(memberID, sourceRef string, turnEnds int) *BladeWardCondition {
	if sourceRef == "" {
		sourceRef = refs.Spells.BladeWard().String()
	}
	return &BladeWardCondition{
		MemberID:     memberID,
		SourceRef:    sourceRef,
		TurnEndsLeft: turnEnds,
	}
}

// IsApplied returns true if this condition is currently applied.
func (b *BladeWardCondition) IsApplied() bool { return b.bus != nil }

// Apply subscribes the ward to the damage chain it softens, to the warded
// creature's turn ends, and to the end of the fight.
func (b *BladeWardCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if b.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "blade ward condition already applied")
	}
	b.bus = bus

	damageChain := dnd5eEvents.DamageChain.On(bus)
	damageSub, err := damageChain.SubscribeWithChain(ctx, b.onDamageChain)
	if err != nil {
		b.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	b.subscriptionIDs = append(b.subscriptionIDs, damageSub)

	turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
	turnSub, err := turnEnds.Subscribe(ctx, b.onTurnEnd)
	if err != nil {
		_ = b.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn end topic")
	}
	b.subscriptionIDs = append(b.subscriptionIDs, turnSub)

	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	combatSub, err := combatEnds.Subscribe(ctx, b.onCombatEnd)
	if err != nil {
		_ = b.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to combat end topic")
	}
	b.subscriptionIDs = append(b.subscriptionIDs, combatSub)

	restSub, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: ConditionAddressOf(b.MemberID, b), Remove: b.Remove,
	})
	if err != nil {
		_ = b.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	b.subscriptionIDs = append(b.subscriptionIDs, restSub)

	return nil
}

// Remove unsubscribes this condition from all events.
func (b *BladeWardCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if b.bus == nil {
		return nil
	}

	total := len(b.subscriptionIDs)
	var errs []error
	for _, subID := range b.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	b.subscriptionIDs = nil
	b.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// onDamageChain halves incoming weapon damage of the three physical types.
//
// The predicate is per COMPONENT — a component with no Multiplier of its own,
// of a physical type, stamped as weapon-sourced — and the multiplier it appends
// is per TYPE, because that is the grain combat.FinalDamage folds on. See the
// limit named on the type doc.
func (b *BladeWardCondition) onDamageChain(
	_ context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	if event.TargetID != b.MemberID {
		return c, nil
	}

	ward := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		warded := make(map[damage.Type]struct{})
		for _, component := range e.Components {
			if component.Multiplier != nil {
				continue
			}
			if component.Source != dnd5eEvents.DamageSourceWeapon {
				continue
			}
			if !component.DamageType.IsPhysical() {
				continue
			}
			warded[component.DamageType] = struct{}{}
		}
		for damageType := range warded {
			e.Components = append(e.Components, dnd5eEvents.DamageComponent{
				Source: dnd5eEvents.DamageSourceCondition,
				Roll: dnd5eEvents.RollComponent{
					Source: dnd5eEvents.RollSource{
						Ref:  refs.Conditions.BladeWard(),
						Name: BladeWardName,
					},
				},
				DamageType: damageType,
				Multiplier: dnd5eEvents.Multiply(0.5),
			})
		}
		return e, nil
	}

	if err := c.Add(combat.StageFinal, "blade_ward_resistance", ward); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add blade ward resistance for member %s", b.MemberID)
	}

	return c, nil
}

// onTurnEnd spends one of the warded creature's own turn ends and ends the ward
// when the count runs out. Other members' turns do not own this clock.
func (b *BladeWardCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	if event.SubjectID != b.MemberID {
		return nil
	}

	b.TurnEndsLeft--
	if b.TurnEndsLeft > 0 {
		return nil
	}
	return b.end(ctx, "expired")
}

// onCombatEnd ends the ward with the fight.
func (b *BladeWardCondition) onCombatEnd(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
	if event.SubjectID != b.MemberID {
		return nil
	}
	return b.end(ctx, "combat ended")
}

// end publishes the removal and detaches. The publish comes first so the
// removal is on the bus while this condition still owns the ward, which is what
// an activation's effect collector reads.
func (b *BladeWardCondition) end(ctx context.Context, reason string) error {
	if b.bus == nil {
		return nil
	}
	bus := b.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     b.MemberID,
		ConditionRef: refs.Conditions.BladeWard().String(),
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish blade ward removal for member %s", b.MemberID)
	}
	return b.Remove(ctx, bus)
}

// ToJSON converts the condition to JSON for persistence.
func (b *BladeWardCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(BladeWardConditionData{
		Ref:          refs.Conditions.BladeWard(),
		MemberID:     b.MemberID,
		SourceRef:    b.SourceRef,
		TurnEndsLeft: b.TurnEndsLeft,
	})
}

// loadJSON loads blade ward condition state from JSON.
func (b *BladeWardCondition) loadJSON(data json.RawMessage) error {
	var stored BladeWardConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal blade ward data")
	}
	b.MemberID = stored.MemberID
	b.SourceRef = stored.SourceRef
	b.TurnEndsLeft = stored.TurnEndsLeft
	return nil
}
