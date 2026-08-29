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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// LoadJSON loads one persisted entry of a monster's sheet — a monster TRAIT or
// an ordinary CONDITION — from its JSON representation.
//
// Both, because monster.Data.Conditions holds both: the four traits this
// package owns (immunity, vulnerability, pack tactics, undead fortitude), which
// are authored on the stat block, and the runtime conditions its own field
// comment has always named — poisoned, hidden, and now the universal
// opportunity attack. A condition is recognised by its ref TYPE and handed to
// conditions.LoadJSON; anything else routes through this package's own dispatch
// and an unknown ref is refused.
//
// Note: this loader requires a dice.Roller for traits like Undead Fortitude
// that need to make saving throws.
func LoadJSON(data json.RawMessage, roller dice.Roller) (dnd5eEvents.ConditionBehavior, error) {
	// Peek at the ref to determine trait type
	var peek struct {
		Ref core.Ref `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, rpgerr.Wrap(err, "failed to peek at monster trait ref")
	}

	// A CONDITION IS NOT A TRAIT, and this loader used to answer for neither.
	//
	// monster.Data.Conditions has always been documented as "runtime state:
	// poisoned, hidden, etc." while this switch knew four traits and errored on
	// everything else — so a monster that ever persisted an ordinary condition
	// could not be loaded again. Nothing had put one there, which is why the
	// gap was invisible; seating the universal opportunity attack is what puts
	// one there (rpg-project#316), and without this the FIRST interaction
	// writes an OA blob onto a wolf and the SECOND fails to load it.
	//
	// Routed by ref TYPE rather than by adding one more ID case, because the
	// question "is this a condition" is answered by the ref itself, and the
	// alternative is this switch growing a copy of the conditions package's own
	// dispatch — the drift risk AllTraitRefs already documents, doubled.
	if peek.Ref.Type == refs.TypeConditions {
		condition, err := conditions.LoadJSON(data)
		if err != nil {
			return nil, rpgerr.Wrapf(err, "failed to load monster condition %s", peek.Ref.ID)
		}

		return condition, nil
	}

	// Route based on ref ID
	switch peek.Ref.ID {
	case refs.MonsterTraits.Immunity().ID:
		trait := &immunityCondition{}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load immunity trait")
		}
		return trait, nil

	case refs.MonsterTraits.Vulnerability().ID:
		trait := &vulnerabilityCondition{}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load vulnerability trait")
		}
		return trait, nil

	case refs.MonsterTraits.PackTactics().ID:
		trait := &packTacticsCondition{}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load pack tactics trait")
		}
		return trait, nil

	case refs.MonsterTraits.UndeadFortitude().ID:
		if roller == nil {
			return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "roller is required for undead fortitude trait")
		}
		trait := &undeadFortitudeCondition{roller: roller}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load undead fortitude trait")
		}
		return trait, nil

	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown monster trait ref: %s", peek.Ref.ID)
	}
}

// AllTraitRefs returns the canonical ref strings for every monster trait
// type this package knows how to load (rpg-toolkit#778) — mirrors LoadJSON's
// dispatch switch exactly; a trait ref added as a new LoadJSON case must be
// added here too, or it won't be recognized as structurally permanent
// (never live-announced) by encounter's ActiveConditions snapshot
// projection. Monster traits (Immunity/Vulnerability/PackTactics/
// UndeadFortitude) are attached at monster construction — the monster-side
// equivalent of a character's Grant.Conditions — and never go through
// Encounter.ActivateFeature's live broker bridge, so they belong in the
// same excluded set as character.StructurallyPermanentConditionRefs.
//
// This hand-mirror is a real, documented drift risk: TestAllTraitRefs_NoPhantomEntries
// (loader_test.go) catches a phantom entry (a ref listed here LoadJSON
// doesn't actually recognize), but cannot catch a missing entry (a new
// LoadJSON case forgotten here) — that direction needs LoadJSON
// restructured around an enumerable registry this function could derive
// from instead of mirroring. Tracked as rpg-toolkit#780.
func AllTraitRefs() []string {
	return []string{
		refs.MonsterTraits.Immunity().String(),
		refs.MonsterTraits.Vulnerability().String(),
		refs.MonsterTraits.PackTactics().String(),
		refs.MonsterTraits.UndeadFortitude().String(),
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
		condition, err := LoadJSON(data, roller)
		if err != nil {
			return rpgerr.Wrap(err, "failed to load monster condition")
		}

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
// definitions, and the trait blobs it was persisted with. No event bus is
// involved and nothing is applied — [AttachMonster] puts the result on a bus.
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

	return m, nil
}

// AttachMonster puts a loaded monster on an event bus: its sheet keeper first,
// then every trait it is carrying, each through the bus scoped to that trait's
// own ref.
//
// The traits it applies are the blobs the monster came back from [LoadMonster]
// holding — taken off it, so a second attach cannot apply them twice and
// ToData cannot write them twice. That is the difference from
// [LoadMonsterConditions], which is handed its blobs by a caller who read them
// out of the data itself.
//
// Ordering is fixed rather than incidental: the monster's own hooks are
// attached before any trait's, so two attaches over identical data grant
// identical registrations in identical order (ADR-0038 R4).
//
// **A failed attach is a no-op**, and here that is the whole point rather than
// tidiness. The blobs come off the monster before anything is known to work, so
// an error partway through — an unroutable ref, a bus that will not take a
// subscription — would otherwise leave every unprocessed blob nowhere at all,
// and the next ToData would write the monster back without conditions nobody
// removed. That is the silent loss rpg-toolkit#948 named and this PR exists to
// close, so: nothing is added to the monster until every trait has loaded and
// applied, and any failure puts the blobs back, takes off whatever went on, and
// removes the keeper. ToData after a failed attach writes what it wrote before
// it, and the attach can be retried.
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

	if err := m.SheetKeeper().Apply(ctx, bus); err != nil {
		return rpgerr.Wrap(err, "failed to attach monster sheet keeper")
	}

	// CARRIED separately from what the sheet actually had, because a rollback
	// must put back what this attach was handed and nothing else. Every
	// unattachMonster below is given `carried`, never `blobs`.
	carried := m.TakeUnappliedConditions()
	blobs, err := carryingFreeReactions(carried, m.GetID())
	if err != nil {
		// The blobs are already DRAINED and the keeper is already APPLIED, so
		// returning here without undoing both would leave the monster mutated
		// by an attach that failed: its traits gone from the sheet and its
		// keeper still listening. Every other failure in this function rolls
		// back; this one has to as well.
		unattachMonster(ctx, m, bus, nil, carried)

		return err
	}
	attached := make([]attachedTrait, 0, len(blobs))

	for i, blob := range blobs {
		condition, err := LoadJSON(blob, roller)
		if err != nil {
			unattachMonster(ctx, m, bus, attached, carried)

			return rpgerr.Wrapf(err, "failed to load monster condition %d: %s", i, blob)
		}

		// Asked of the condition rather than peeked back out of the blob — see
		// LoadMonsterConditions for why, and refOf for what happens when a
		// condition breaks its own contract.
		traitBus := dnd5eEvents.BusForEffect(bus, refOf(condition))

		if err := condition.Apply(ctx, traitBus); err != nil {
			// Clean up any partial subscriptions from the failed Apply.
			_ = condition.Remove(ctx, traitBus)
			unattachMonster(ctx, m, bus, attached, carried)

			return rpgerr.Wrapf(err, "failed to apply monster condition %d: %s", i, blob)
		}

		attached = append(attached, attachedTrait{condition: condition, bus: traitBus})
	}

	// Only now, when every one of them worked. A trait added to the monster
	// mid-loop would have to be taken back out again on the next failure, and
	// the monster has no verb for that — which is exactly the kind of missing
	// undo that makes partial writes permanent.
	for _, trait := range attached {
		if err := m.AddLoadedCondition(trait.condition); err != nil {
			unattachMonster(ctx, m, bus, attached, carried)

			return rpgerr.Wrap(err, "failed to record monster trait")
		}
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
// every trait that went on comes back off newest first, the keeper is removed,
// and the blobs go back where they were taken from, in order.
func unattachMonster(
	ctx context.Context,
	m *monster.Monster,
	bus events.EventBus,
	attached []attachedTrait,
	blobs []json.RawMessage,
) {
	// Newest first, the order resolution tears down in (ADR-0038 R5).
	for i := len(attached) - 1; i >= 0; i-- {
		_ = attached[i].condition.Remove(ctx, attached[i].bus)
	}

	_ = m.SheetKeeper().Remove(ctx, bus)

	// All of them, including the one that failed and every one never reached.
	// Nothing was added to the monster, so putting the blobs back is the whole
	// of the undo, and ToData writes exactly what it wrote before the attach.
	for _, blob := range blobs {
		m.AddTraitData(blob)
	}
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

// freeReactionRefs are the reactions a combatant carries by existing. ONE
// ENTRY, and the list is the rule rather than an optimisation of it: a COSTED
// reaction is not had by existing and does not belong here.
var freeReactions = []freeReaction{
	{
		ref:   refs.Conditions.OpportunityAttack(),
		build: func(id string) dnd5eEvents.ConditionBehavior { return conditions.NewOpportunityAttackCondition(id) },
	},
}

// freeReaction pairs the ref a combatant carries with the way to build one.
//
// It carries its own constructor rather than being a bare ref so the failure
// path below is reachable from a test: writing a blob can only fail if a
// condition's ToJSON does, which the opportunity attack's never will, and a
// rollback guard no test can enter is the built-and-unwired shape this whole
// slice exists to undo.
type freeReaction struct {
	ref   *core.Ref
	build func(id string) dnd5eEvents.ConditionBehavior
}

// carryingFreeReactions gives a monster the reactions every combatant has, the
// way a class grant gives a character a class feature.
//
// Kirk ruled the shape 2026-08-28 (rpg-project#316): "when we initialize
// monsters they should have the condition on them like a character grant... and
// only dirty if they fire the OA."
//
// # Here rather than in monster.Load
//
// Because Load must stay a pure read. Carrying the blob there made
// Load(d).ToData() stop being byte-identical to d, and
// TestRoundTripsByteIdenticalWithNoBus said so immediately — for the monster
// sheet exactly as it did for the character one. Attach is where a sheet becomes
// a participant, which is when having a reaction starts to mean anything.
//
// # A blob, and it DOES join the sheet
//
// A blob rather than a live condition because that is how a monster carries
// every other one, and this loop is the single path that turns trait data into
// behaviour. It joins the sheet through AddLoadedCondition below, which does not
// mark it dirty — gaining a reaction is not something that happened in the
// world — and from then on ToData serializes it, which is what lets a SPENT
// meter survive to the next call.
//
// That last part is where a monster differs from a character, and the asymmetry
// is the ruling rather than an oversight: a character's meter is
// ActionEconomy.ReactionsRemaining, already persisted and already what
// Protection fighting style competes for, so character.Attach carries the
// condition live and never writes it down. A monster has no action economy at
// all, so its UsedThisTurn is the only meter there is and it has to be written.
func carryingFreeReactions(blobs []json.RawMessage, id string) ([]json.RawMessage, error) {
	for _, reaction := range freeReactions {
		if carriesRef(blobs, reaction.ref) {
			continue
		}

		blob, err := reaction.build(id).ToJSON()
		if err != nil {
			return nil, rpgerr.Wrapf(err, "failed to write the %s a combatant carries", reaction.ref)
		}
		blobs = append(blobs, blob)
	}

	return blobs, nil
}

// carriesRef reports whether one of these blobs names this ref, so a reattach
// does not stack a second copy on the persisted one.
func carriesRef(blobs []json.RawMessage, ref *core.Ref) bool {
	for _, raw := range blobs {
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil {
			continue
		}
		if peek.Ref.String() == ref.String() {
			return true
		}
	}

	return false
}
