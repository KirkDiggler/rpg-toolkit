// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// bindRoller hands the attach roller to a condition that rolls with one. A
// condition that does not implement [conditions.RollerBinder] is untouched, and
// a nil roller changes nothing: a trait that needs a roller refuses to roll
// without one rather than falling back to a default.
//
// Every entry of a monster's sheet loads through [conditions.LoadJSON], because
// a stat-block trait (immunity, vulnerability, pack tactics, undead fortitude)
// is a condition: it is authored on the stat block, lives in the one registry,
// and a ref nothing registers is refused, never skipped.
func bindRoller(condition dnd5eEvents.ConditionBehavior, roller dice.Roller) {
	if binder, ok := condition.(conditions.RollerBinder); ok {
		binder.BindRoller(roller)
	}
}

// LoadMonsterConditions is a helper function that loads conditions/traits from JSON data
// and applies them to a monster. This is needed because the monster package
// cannot import the monstertraits package directly (import cycle).
//
// Usage:
//
//	mon, err := monster.LoadFromData(ctx, data, bus)
//	if err := monstertraits.LoadMonsterConditions(ctx, mon, data.Conditions, bus, roller); err != nil {
//	    // handle error
//	}
//
// New code should use [AttachMonster] instead. It is this function plus the
// monster's own sheet keeper, over the blobs the monster is already carrying
// rather than blobs the caller has to remember to pass — which is the
// difference between forgetting a call and being unable to.
func LoadMonsterConditions(
	ctx context.Context,
	m *monster.Monster,
	conditionData []json.RawMessage,
	bus events.EventBus,
	roller dice.Roller,
) error {
	for _, data := range conditionData {
		condition, err := conditions.LoadJSON(data)
		if err != nil {
			return rpgerr.Wrap(err, "failed to load monster condition")
		}
		bindRoller(condition, roller)

		// Apply through the bus this particular trait should be attributed to.
		// A plain bus returns itself, so this is a no-op for every caller that
		// is not an attach site keeping a registration list; a bus that
		// implements dnd5eEvents.EffectScoper gets to record which trait made
		// each subscription.
		//
		// ASKED OF THE CONDITION, not peeked back out of its JSON.
		// ConditionBehavior.Ref exists precisely so a live effect can name
		// itself honestly (rpg-toolkit#971), and the comment that used to sit
		// here — "a ConditionBehavior cannot name itself" — predated it.
		traitBus := dnd5eEvents.BusForEffect(bus, refOf(condition))

		// Apply the condition so it subscribes to events
		if err := condition.Apply(ctx, traitBus); err != nil {
			// Clean up any partial subscriptions
			_ = condition.Remove(ctx, traitBus)
			return rpgerr.Wrap(err, "failed to apply monster condition")
		}

		if err := m.AddLoadedCondition(condition); err != nil {
			_ = condition.Remove(ctx, traitBus)

			return rpgerr.Wrap(err, "failed to record monster condition")
		}

	}
	return nil
}

// LoadMonster is the whole pure load of a monster: the sheet, its shared action
// definitions, and every condition reconstituted but not applied. No event bus
// is involved. Unknown/malformed conditions fail here, before cross-holder
// binding; AttachMonster later applies these same prepared behaviors.
//
// The monster package now loads inert action definitions directly. This wrapper
// remains the composition entry point paired with [AttachMonster].
//
// LoadMonster(d).ToData() is the data it was given, with the caveats in
// [monster.Load]'s godoc: Data.Features and Data.Inventory have nowhere to
// live on a monster and no loader carries them.
func LoadMonster(ctx context.Context, d *monster.Data) (*monster.Monster, error) {
	m, err := monster.Load(ctx, d)
	if err != nil {
		return nil, err
	}

	if err := prepareMonsterConditions(m); err != nil {
		return nil, err
	}
	return m, nil
}

// AttachMonster puts a loaded monster on an event bus: its sheet keeper first,
// then every trait it is carrying, each through the bus scoped to that trait's
// own ref.
//
// It applies the behaviors already prepared by LoadMonster, so an application
// binding installed between load and attach points at the very same instances.
// A factory-created monster still carrying raw traits uses the same preparation
// before any subscription; no raw and loaded copies are retained together.
//
// Ordering is fixed rather than incidental: the monster's own hooks are
// attached before any trait's, so two attaches over identical data grant
// identical registrations in identical order (ADR-0038 R4).
//
// A failed attach revokes every subscription it granted and removes the keeper.
// Prepared behaviors remain on the sheet, detached, so ToData retains their
// state exactly once and attachment can be retried. Preparation itself validates
// every carried blob before recording any behavior; a failure restores the raw
// input, without publishing or leaving a partially loaded condition set.
func AttachMonster(
	ctx context.Context,
	m *monster.Monster,
	bus events.EventBus,
	roller dice.Roller,
) error {
	if m == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "monster is required")
	}
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	// Factory-created monsters may still carry raw traits. Both entry paths use
	// the same pure preparation; an already prepared monster is unchanged.
	if err := prepareMonsterConditions(m); err != nil {
		return err
	}
	if err := m.SheetKeeper().Apply(ctx, bus); err != nil {
		return rpgerr.Wrap(err, "failed to attach monster sheet keeper")
	}
	loaded := m.GetConditions()
	attached := make([]attachedTrait, 0, len(loaded))
	for i, condition := range loaded {
		bindRoller(condition, roller)

		// Asked of the condition rather than peeked back out of the blob — see
		// LoadMonsterConditions for why, and refOf for what happens when a
		// condition breaks its own contract.
		traitBus := dnd5eEvents.BusForEffect(bus, refOf(condition))

		if err := condition.Apply(ctx, traitBus); err != nil {
			// Clean up any partial subscriptions from the failed Apply.
			_ = condition.Remove(ctx, traitBus)
			unattachMonster(ctx, m, bus, attached)

			return rpgerr.Wrapf(err, "failed to apply monster condition %d (%s)", i, condition.Ref().String())
		}

		attached = append(attached, attachedTrait{condition: condition, bus: traitBus})
	}

	return nil
}

// attachedTrait is a trait this attach put on a bus, with the bus it went on.
// Remembered rather than recomputed: asking an EffectScoper for a scope is how
// it learns an effect exists, and a rollback that asked again would leave a
// registration ledger describing an attach that did not happen.
type attachedTrait struct {
	condition dnd5eEvents.ConditionBehavior
	bus       events.EventBus
}

// unattachMonster returns the monster to the state [AttachMonster] found it in:
// every trait that went on comes back off newest first and the keeper is removed.
// Prepared behaviors remain detached on the sheet, with no duplicate raw copy.
func unattachMonster(
	ctx context.Context,
	m *monster.Monster,
	bus events.EventBus,
	attached []attachedTrait,
) {
	// Newest first, the order resolution tears down in (ADR-0038 R5).
	for i := len(attached) - 1; i >= 0; i-- {
		_ = attached[i].condition.Remove(ctx, attached[i].bus)
	}

	_ = m.SheetKeeper().Remove(ctx, bus)

	// Prepared behaviors stay on the sheet, detached, for an exact retry. No
	// second raw copy is restored alongside them.
}

// requireNameableTrait refuses a condition that cannot name itself, at the
// point in an attach where refusing is still free.
//
// It asks the same question Monster.AddLoadedCondition asks. Asking twice is
// not redundancy: the sheet's door is the invariant, and this is the earlier
// checkpoint that keeps a refusal from ever arriving mid-write.
func requireNameableTrait(condition dnd5eEvents.ConditionBehavior) error {
	if condition == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "nil condition")
	}

	if condition.Ref() == nil {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"trait %T returns a nil Ref, so a sheet would refuse it", condition)
	}

	return nil
}

// refOf is ConditionBehavior.Ref with the nil case answered.
//
// The interface says Ref "must never return nil", and this is what happens when
// one does anyway: the zero Ref, which is exactly what the JSON peek this
// replaced returned for a blob with no ref, and costs the same thing —
// attribution, not correctness. A panic here would take down an attach over a
// label.
//
// It is only ever a LABEL, and the distinction is load-bearing: the same
// condition is offered to Monster.AddLoadedCondition below, which refuses a
// nil ref outright. So a nameless condition gets an anonymous bus here and
// then fails to reach the sheet at all — this function tolerating nil is not
// the sheet tolerating it. rpg-project#319 Phase 6 review caught that gap:
// naming the bus and admitting to the sheet are different questions, and only
// the second one is an invariant.
func refOf(condition dnd5eEvents.ConditionBehavior) core.Ref {
	if ref := condition.Ref(); ref != nil {
		return *ref
	}

	return core.Ref{}
}
