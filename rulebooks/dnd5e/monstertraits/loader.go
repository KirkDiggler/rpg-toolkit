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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// LoadJSON loads a monster trait from its JSON representation.
// The game server stores traits as opaque JSON blobs;
// this function deserializes them into strongly-typed structs.
//
// Note: This loader requires a dice.Roller for traits like Undead Fortitude
// that need to make saving throws.
func LoadJSON(data json.RawMessage, roller dice.Roller) (dnd5eEvents.ConditionBehavior, error) {
	// Peek at the ref to determine trait type
	var peek struct {
		Ref core.Ref `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, rpgerr.Wrap(err, "failed to peek at monster trait ref")
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
		// each subscription. The ref is the one LoadJSON just routed on, peeked
		// again here because a ConditionBehavior cannot name itself.
		traitBus := dnd5eEvents.BusForEffect(bus, peekTraitRef(data))

		// Apply the condition so it subscribes to events
		if err := condition.Apply(ctx, traitBus); err != nil {
			// Clean up any partial subscriptions
			_ = condition.Remove(ctx, traitBus)
			return rpgerr.Wrap(err, "failed to apply monster condition")
		}

		m.AddCondition(condition)
	}
	return nil
}

// LoadMonster is the whole pure load of a monster: the sheet, its actions, and
// the trait blobs it was persisted with. No event bus is involved and nothing
// is applied — [AttachMonster] is what puts the result on a bus.
//
// This lives here for a mechanical reason rather than a conceptual one: the
// monster package cannot import either loader (both import it), and this
// package is the only one that can see both without a cycle. What it buys is
// worth the odd address. The three-call assembly this replaces could lose a
// monster's actions or conditions by forgetting a call — ToData serializes
// both, so a monster loaded by two of the three calls is written back with the
// third one's contents gone. One call cannot be two-thirds made.
//
// LoadMonster(d).ToData() is the data it was given, with the caveats in
// [monster.Load]'s godoc: Data.Features and Data.Inventory have nowhere to
// live on a monster and no loader carries them.
func LoadMonster(ctx context.Context, d *monster.Data) (*monster.Monster, error) {
	m, err := monster.Load(ctx, d)
	if err != nil {
		return nil, err
	}

	if err := actions.LoadMonsterActions(m, d.Actions); err != nil {
		return nil, rpgerr.Wrap(err, "failed to load monster actions")
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

	return LoadMonsterConditions(ctx, m, m.TakeUnappliedConditions(), bus, roller)
}

// peekTraitRef reads the ref a persisted trait routes on, which is the same
// field LoadJSON routes on. It returns the zero Ref for a blob that has none
// rather than an error: LoadJSON has already accepted the blob by this point,
// and the only thing a missing ref costs is attribution.
func peekTraitRef(data json.RawMessage) core.Ref {
	var peek struct {
		Ref core.Ref `json:"ref"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return core.Ref{}
	}

	return peek.Ref
}
