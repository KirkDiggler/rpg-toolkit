// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ActivationInput names one member using something they already carry.
type ActivationInput struct {
	// MemberID is who is activating. Required, and must be a CHARACTER in the
	// cast: a monster's abilities are driven by its behaviour, not declared by
	// a player, and its economy belongs to whoever runs its turn rather than
	// to its sheet — the same line [ErrNoPayer] draws.
	MemberID string

	// Ability is which of the things they carry. Required. A ref rather than
	// an index or a name, because the sheet's own lookup is by ref and a
	// second addressing scheme would be a second place for the two to
	// disagree.
	Ability *core.Ref

	// TargetID is who it lands on, for the one ability of the seven that takes
	// somebody: Help. Empty for Dodge, Dash, Disengage, Hide, Rage and Second
	// Wind, and a populated ID for one of those is a caller defect rather than
	// a value quietly ignored.
	TargetID string

	// ObserverPassivePerceptions is the passive Perception of everyone who
	// could notice this, which Hide's Stealth check is rolled against.
	//
	// Carried rather than computed, for [ErrNoSight]'s reason: who can see
	// whom is the composition's question, and a package that answered it here
	// would be deciding a rule about light it cannot see. Empty for six of the
	// seven — and empty is a legitimate answer for Hide too (nobody is
	// watching), so this cannot be validated by presence.
	ObserverPassivePerceptions []int

	// Roller is the dice roller this activation rolls with. REQUIRED, and
	// refused at [NewActivation] before the world is loaded or anything is
	// paid — the same rule resolve.Input holds: a machine that rolls carries
	// its own roller. Second Wind is the one of the seven that rolls, and a
	// nil here would otherwise fall back to process-global randomness mid-
	// interaction (rpg-toolkit#1427).
	Roller dice.Roller
}

// ActivationEffectKind identifies one closed kind of fact produced while an
// ability runs on its interaction bus.
type ActivationEffectKind string

const (
	// EffectHealingApplied is actual post-clamp healing applied to a target.
	EffectHealingApplied ActivationEffectKind = "healing-applied"

	// EffectConditionApplied is a condition attached to a target.
	EffectConditionApplied ActivationEffectKind = "condition-applied"

	// EffectConditionRemoved is a condition that ended on a target.
	EffectConditionRemoved ActivationEffectKind = "condition-removed"

	// EffectCapacityGranted is capacity banked by an ability, such as Dash's movement.
	EffectCapacityGranted ActivationEffectKind = "capacity-granted"
)

// ActivationEffect is one typed, display-ready fact produced by an activation.
// Fields not used by its Kind remain at their zero value.
type ActivationEffect struct {
	Kind     ActivationEffectKind
	TargetID string
	Ref      string
	Name     string

	Amount    int
	Requested int
	Before    int
	After     int

	// Calculation is the complete sourced roll behind a healing fact — dice
	// trace, sourced modifiers, authoritative total — deep-cloned at capture.
	// It is the one representation of the roll's facts: the legacy scalar
	// Roll/Modifier projection is gone, and a heal published without a
	// calculation (legacy, non-Activate healing such as Hit Dice) is not an
	// activation result and is not captured at all.
	Calculation *dnd5eEvents.RollCalculation

	Description string
	Reason      string
}

// ActivationOutcome is what an activation produced.
//
// Dirty sheets remain the durable rules state. Effects are the interaction
// facts a caller cannot recover from those final sheets: a clamped heal's roll,
// a condition that was removed, and the order in which those facts happened.
type ActivationOutcome struct {
	// Ability is the ref that ran, echoed back. A caller that dispatched by
	// selector rather than by ref gets to learn what the selector meant
	// without parsing it.
	Ability string

	// GrantedCapacity is the ability's own description of what it banked —
	// "1 attack", "30ft movement" — or empty when it banked nothing. Display
	// text authored by the ability, never parsed: Dash's effect on the ledger
	// is already in the ledger.
	GrantedCapacity string

	// Effects are the typed facts emitted on this activation's interaction bus,
	// in publication order, followed by GrantedCapacity when one was produced.
	Effects []ActivationEffect
}

func (ActivationOutcome) isOutcome() {}

// activationEffectCollector observes only the bus handed to one activation
// step. It owns exactly the subscriptions it records in subscriptionIDs.
// These subscriptions pass through resolution's instrumented root surface, so
// successful resolutions include them in [Output.Hooks] with empty Participant
// and Effect attribution; closing the collector revokes them without erasing
// that record of what the resolution granted.
type activationEffectCollector struct {
	bus events.EventBus

	mu      sync.Mutex
	effects []ActivationEffect

	subscriptionIDs []string
	closeOnce       sync.Once
	closeErr        error
}

func newActivationEffectCollector(
	ctx context.Context, bus events.EventBus,
) (*activationEffectCollector, error) {
	if bus == nil {
		return nil, errors.New("activation effect collector: event bus is required")
	}

	collector := &activationEffectCollector{bus: bus}

	id, err := dnd5eEvents.HealingAppliedTopic.On(bus).Subscribe(ctx, collector.captureHealingApplied)
	if err != nil {
		return nil, fmt.Errorf("activation effect collector: subscribe to healing applied: %w", err)
	}
	collector.subscriptionIDs = append(collector.subscriptionIDs, id)

	id, err = dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(ctx, collector.captureConditionApplied)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("activation effect collector: subscribe to condition applied: %w", err),
			collector.Close(ctx),
		)
	}
	collector.subscriptionIDs = append(collector.subscriptionIDs, id)

	id, err = dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx, collector.captureConditionRemoved)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("activation effect collector: subscribe to condition removed: %w", err),
			collector.Close(ctx),
		)
	}
	collector.subscriptionIDs = append(collector.subscriptionIDs, id)

	return collector, nil
}

func (c *activationEffectCollector) captureHealingApplied(
	_ context.Context, event dnd5eEvents.HealingAppliedEvent,
) error {
	if event.SourceRef == nil {
		return errors.New("activation effect collector: healing source ref is required")
	}

	// Clone before validation and projection. A publisher retaining its mutable
	// ref cannot rewrite the identity already captured by this interaction.
	source := *event.SourceRef
	if err := source.IsValid(); err != nil {
		return fmt.Errorf("activation effect collector: healing source ref is invalid: %w", err)
	}
	if strings.TrimSpace(event.SourceName) == "" {
		return errors.New("activation effect collector: healing source name is required")
	}

	// A heal with no calculation is legacy, non-Activate healing — Hit Dice's
	// scalar-only shape. It is not a fact an ability activated, so it is not
	// captured: neither the clamp facts nor the publisher's own scalars are
	// mirrored onto an activation effect.
	if event.Calculation == nil {
		return nil
	}

	// Validate, then deep-clone. An invalid trace is refused whole — no partial
	// effect — and a valid one is owned from here on: a publisher mutating its
	// calculation after publication cannot rewrite what this interaction
	// captured.
	if err := dnd5eEvents.ValidateRollCalculation(event.Calculation); err != nil {
		return fmt.Errorf("activation effect collector: healing calculation is invalid: %w", err)
	}

	c.append(ActivationEffect{
		Kind: EffectHealingApplied, TargetID: event.TargetID,
		Ref: source.String(), Name: event.SourceName,
		Amount: event.Applied, Requested: event.Requested,
		Before: event.HPBefore, After: event.HPAfter,
		Calculation: dnd5eEvents.CloneRollCalculation(event.Calculation),
	})
	return nil
}

func (c *activationEffectCollector) captureConditionApplied(
	_ context.Context, event dnd5eEvents.ConditionAppliedEvent,
) error {
	if event.Target == nil {
		return errors.New("activation effect collector: applied condition target is required")
	}
	if event.Condition == nil {
		return errors.New("activation effect collector: applied condition is required")
	}

	ref, name, err := activationConditionIdentity(event.Condition.Ref())
	if err != nil {
		return fmt.Errorf("activation effect collector: applied condition: %w", err)
	}

	c.append(ActivationEffect{
		Kind: EffectConditionApplied, TargetID: event.Target.GetID(), Ref: ref, Name: name,
	})
	return nil
}

func (c *activationEffectCollector) captureConditionRemoved(
	_ context.Context, event dnd5eEvents.ConditionRemovedEvent,
) error {
	ref, err := core.ParseString(event.ConditionRef)
	if err != nil {
		return fmt.Errorf("activation effect collector: removed condition ref is invalid: %w", err)
	}
	canonical, name, err := activationConditionIdentity(ref)
	if err != nil {
		return fmt.Errorf("activation effect collector: removed condition: %w", err)
	}

	c.append(ActivationEffect{
		Kind: EffectConditionRemoved, TargetID: event.MemberID,
		Ref: canonical, Name: name, Reason: event.Reason,
	})
	return nil
}

func activationConditionIdentity(ref *core.Ref) (string, string, error) {
	if ref == nil {
		return "", "", errors.New("condition ref is required")
	}

	clone := *ref
	if err := clone.IsValid(); err != nil {
		return "", "", fmt.Errorf("condition ref is invalid: %w", err)
	}
	display, ok := conditions.DisplayFor(clone)
	if !ok {
		return "", "", fmt.Errorf("condition ref %s has no display catalog entry", clone.String())
	}

	return clone.String(), display.Name, nil
}

func (c *activationEffectCollector) append(effect ActivationEffect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.effects = append(c.effects, effect)
}

// Effects returns an independently owned snapshot in synchronous publication order.
//
// Independent all the way down: the slice is copied and each effect's
// calculation is deep-cloned, so a caller mutating the snapshot cannot rewrite
// what this interaction stored, and two calls never share a trace.
func (c *activationEffectCollector) Effects() []ActivationEffect {
	c.mu.Lock()
	defer c.mu.Unlock()
	effects := slices.Clone(c.effects)
	for i := range effects {
		effects[i].Calculation = dnd5eEvents.CloneRollCalculation(effects[i].Calculation)
	}
	return effects
}

// Close revokes every subscription the collector recorded. It is idempotent,
// attempts every revocation newest-first, and preserves all cleanup failures.
func (c *activationEffectCollector) Close(ctx context.Context) error {
	c.closeOnce.Do(func() {
		var errs []error
		for i := len(c.subscriptionIDs) - 1; i >= 0; i-- {
			if err := c.bus.Unsubscribe(ctx, c.subscriptionIDs[i]); err != nil {
				errs = append(errs, err)
			}
		}
		c.closeErr = errors.Join(errs...)
	})
	return c.closeErr
}

// NewActivation returns the machine that uses one thing a character carries.
//
// # It is a sibling of NewBoundary, not an arm of NewAction
//
// [NewAction] dispatches on a populated profile of an authored
// actions.Definition, and validates one before it does anything else. The
// abilities and features a character carries have no such definition and do
// not need one: each already declares its own ActionType and enforces its own
// resources, which is the price the door actually charges. Minting a
// Definition for Dodge purely so a dispatcher recognised it would be a
// compiled price nothing charges — the exact fiction Afford's "read the price
// off the same profile" rule exists to prevent (rpg-project#300 §4).
//
// What it shares with [NewBoundary] is the shape that matters: attach
// everyone, do ONE thing on the interaction's own bus, collect dirty sheets.
//
// # Why the bus is the whole point
//
// A feature does not attach its own condition. Rage builds a RagingCondition
// and PUBLISHES it on ConditionAppliedTopic; the owner's SheetKeeper is
// subscribed and applies it. Dodge, Disengage, Hide, Help and Reckless Attack
// all take the same path.
//
// So an activation run off the bus is not an error — it is worse. The publish
// succeeds, returns nil, and the sheet is persisted with no condition on it.
// EffectiveAC silently falls back to base and nothing in the call stack says
// anything happened (the shape that bit the equip path, rpg-api#842). Running
// here means the actor was attached by [attachAll] through
// [character.Attach], so the subscription that applies the condition is live
// for exactly as long as the activation is.
//
// # It charges nothing at the door
//
// [Input.Cost] must stay NIL for an activation, and that is a real constraint
// rather than an omission. The ability spends its own slot — activateFeature
// calls consumeActionType, and a combat ability spends through the toolkit
// economy it is handed — so a Cost passed alongside would charge the same
// ledger twice and the second charge would look exactly like the first.
// "Free action" in Cost's own doc means "this package charges nothing", not
// "this costs nothing".
//
// Refuses a nil input, a member with no ID, a missing or invalid ability ref,
// and a nil roller, all before the world is loaded.
func NewActivation(in *ActivationInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.MemberID == "" {
		return nil, fmt.Errorf("%w: no member is activating", ErrBadActivation)
	}
	if in.Ability == nil {
		return nil, fmt.Errorf("%w: member %q named no ability", ErrBadActivation, in.MemberID)
	}
	if err := in.Ability.IsValid(); err != nil {
		return nil, fmt.Errorf("%w: member %q named an unusable ability: %w",
			ErrBadActivation, in.MemberID, err)
	}
	if in.Roller == nil {
		return nil, fmt.Errorf("%w: member %q rolls with no roller", ErrBadActivation, in.MemberID)
	}

	// BOTH pieces of caller-owned material are copied, not just the obvious
	// one. core.Ref is a mutable struct, so keeping the caller's pointer would
	// let a reused ref change WHICH ABILITY this machine activates after it was
	// constructed — a worse version of the observer-slice hazard, and one that
	// is easy to miss precisely because the slice copy right beside it looks
	// like the defence was already made.
	ability := *in.Ability

	observers := make([]int, len(in.ObserverPassivePerceptions))
	copy(observers, in.ObserverPassivePerceptions)

	return &activationMachine{
		member:    in.MemberID,
		ability:   &ability,
		targetID:  in.TargetID,
		observers: observers,
		roller:    in.Roller,
	}, nil
}

type activationMachine struct {
	member    string
	ability   *core.Ref
	targetID  string
	observers []int
	roller    dice.Roller
}

// Start is pure preflight and runs before payment: it finds the actor and the
// target in the cast and yields the step that does the work, without touching
// either.
//
// The cast lookups happen HERE rather than inside the step for the reason
// Start exists — a member who was never passed in is a caller defect, and
// discovering it after the door has been paid at would charge somebody for an
// interaction that cannot run. It is the same preflight [strikeMachine] does
// for a combatant it was never handed.
func (m *activationMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	actor, ok := cast.Character(m.member)
	if !ok {
		// Named separately from "not in the cast at all" because a monster IS
		// in the cast and still cannot be here: its abilities are driven, not
		// declared, and its economy is not on its sheet.
		if _, isMonster := cast.Monster(m.member); isMonster {
			return nil, fmt.Errorf("%w: %q is a monster; its abilities are driven, not declared",
				ErrBadActivation, m.member)
		}
		return nil, fmt.Errorf("%w: %q is not a participant", ErrBadActivation, m.member)
	}

	if err := m.checkTargetContract(actor); err != nil {
		return nil, err
	}

	var target core.Entity
	if m.targetID != "" {
		found, err := memberEntity(cast, m.targetID)
		if err != nil {
			return nil, err
		}
		target = found
	}

	return m.step(actor, target), nil
}

// checkTargetContract enforces what ActivationInput.TargetID promises: required
// when the ability takes somebody, empty when it does not.
//
// # The rulebook answers, this does not decide
//
// Which abilities take a target is rules knowledge, and re-stating it here
// would be a second table to disagree with the first. So it ASKS: the sheet's
// own AvailableAbilities carries each ability's TargetKind, authored by the
// same table Afford projects into a declaration. This reads that answer and
// enforces it; it does not have an opinion about Help.
//
// # It stays quiet when the sheet cannot answer
//
// AvailableAbilities is empty for a character who is not in combat, and an
// ability this character does not carry is simply absent. NEITHER is a
// malformed call — the first is an actor-state refusal and the second is
// "unknown ability", and both belong to ErrActivationRefused. Refusing them
// here would report a barbarian out of combat as a caller defect, which is the
// exact confusion the two sentinels exist to prevent.
func (m *activationMachine) checkTargetContract(actor *character.Character) error {
	var kind character.TargetKind
	found := false
	for _, available := range actor.AvailableAbilities() {
		if available.Ref != nil && available.Ref.ID == m.ability.ID {
			kind, found = available.TargetKind, true
			break
		}
	}
	if !found {
		return nil
	}

	wantsTarget := kind == character.TargetKindSingleEntity
	switch {
	case wantsTarget && m.targetID == "":
		return fmt.Errorf("%w: %s needs a target and none was named",
			ErrBadActivation, m.ability.String())
	case !wantsTarget && m.targetID != "":
		// Refused rather than ignored. A target quietly dropped is a client
		// that believes it aimed Dodge at somebody and a server that knows
		// better, which is a disagreement nobody finds until it matters.
		return fmt.Errorf("%w: %s takes no target, but %q was named",
			ErrBadActivation, m.ability.String(), m.targetID)
	}
	return nil
}

// step is the one thing this machine does.
//
// A Gather rather than a new Step kind: "do this on the bus and hand me the
// next step" is what Gather already means, and adding a case against one
// example is the mistake this package's own step vocabulary keeps refusing to
// make.
func (m *activationMachine) step(actor *character.Character, target core.Entity) Step {
	return Gather{
		name: fmt.Sprintf("activate %s for %s", m.ability.String(), m.member),
		run: func(ctx context.Context, bus events.EventBus) (next Step, err error) {
			// Subscribe after every participant is attached but before the
			// ability publishes. This bus exists for this Resolve call alone,
			// so the collector cannot observe another interaction's facts.
			collector, err := newActivationEffectCollector(ctx, bus)
			if err != nil {
				return nil, fmt.Errorf("activate %s for %q: collect effects: %w",
					m.ability.String(), m.member, err)
			}
			defer func() {
				if closeErr := collector.Close(ctx); closeErr != nil {
					next = nil
					err = errors.Join(err,
						fmt.Errorf("activate %s for %q: close effect collector: %w",
							m.ability.String(), m.member, closeErr))
				}
			}()

			out, err := actor.ActivateAbility(ctx, &character.ActivateAbilityInput{
				AbilityRef:                 m.ability,
				TargetID:                   m.targetID,
				Target:                     target,
				ObserverPassivePerceptions: m.observers,
				Roller:                     m.roller,
			})
			if err != nil {
				return nil, fmt.Errorf("activate %s for %q: %w", m.ability.String(), m.member, err)
			}

			// THE TRANSLATION, and it happens here rather than three layers
			// up. The sheet answers a refusal as (output{Success:false}, nil):
			// "not in combat", "unknown ability", "no rage charges left", and
			// every feature's own precondition come back as a SUCCESSFUL call
			// carrying a false. A machine that returned Done on that would
			// hand its caller a finished interaction that did nothing, and the
			// dirty sheets would be saved to prove it.
			//
			// Distinct sentinel from ErrBadActivation for ErrCannotPay's
			// reason: one is an actor who cannot do this right now and wants a
			// different verb, the other is content or wiring being wrong and
			// wants a developer. A single sentinel would send the first one
			// looking at the wrong sheet.
			if !out.Success {
				return nil, fmt.Errorf("%w: %s", ErrActivationRefused, out.Error)
			}

			effects := collector.Effects()
			if out.GrantedCapacity != "" {
				// Capacity is returned as a value rather than published. It follows
				// every bus fact because those facts happened during activation.
				effects = append(effects, ActivationEffect{
					Kind: EffectCapacityGranted, TargetID: m.member,
					Description: out.GrantedCapacity,
				})
			}

			return Done{Outcome: ActivationOutcome{
				Ability:         m.ability.String(),
				GrantedCapacity: out.GrantedCapacity,
				Effects:         effects,
			}}, nil
		},
	}
}

// memberEntity finds one member of the cast as an entity, character or monster.
//
// Help is the only one of the seven that takes a target, and it aids an ally —
// which in a dungeon can be a summoned or allied monster as easily as another
// character, so this does not assume the party. An ID that is neither is a
// caller defect and says so rather than passing a nil Target into an ability
// that will dereference it.
func memberEntity(cast *Participants, id string) (core.Entity, error) {
	if ch, ok := cast.Character(id); ok {
		return ch, nil
	}
	if mon, ok := cast.Monster(id); ok {
		return mon, nil
	}
	return nil, fmt.Errorf("%w: target %q is not a participant", ErrBadActivation, id)
}
