// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/stretchr/testify/require"
)

// TestAPartlyLoadedAttachRecordsNothing pins the invariant unattachMonster's
// own doc asserts — "Nothing was added to the monster, so putting the blobs
// back is the whole of the undo" — for a failure that happens AFTER an earlier
// trait already loaded and applied successfully.
//
// That sentence became false for one commit during rpg-project#319 Phase 6.
// Giving Monster.AddLoadedCondition an error return made the final recording
// loop fallible, and a refusal there would have left an earlier trait recorded
// on the sheet AND its blob restored — persisted twice by the next ToData,
// from an attach documented as a no-op. Review caught it.
//
// The fix was prevention: every condition is checked for nameability BEFORE it
// is applied, so a refusal can no longer arrive mid-record. This test pins the
// property that fix preserves, using the failure that is reachable — a blob
// the loader cannot read, arriving second.
func TestAPartlyLoadedAttachRecordsNothing(t *testing.T) {
	ctx := context.Background()

	good, err := Immunity("gob-1", "poison").ToJSON()
	require.NoError(t, err)

	m, err := monster.Load(ctx, &monster.Data{
		ID: "gob-1", Name: "Goblin", HitPoints: 7, MaxHitPoints: 7, ArmorClass: 15,
		Conditions: []json.RawMessage{good, json.RawMessage(`{"ref":"dnd5e:monster_traits:nonesuch"}`)},
	})
	require.NoError(t, err)
	before := m.ToData()

	bus := events.NewEventBus()
	err = AttachMonster(ctx, m, bus, nil)

	require.Error(t, err, "an unreadable blob fails the attach")
	require.Empty(t, m.GetConditions(),
		"the trait that DID load must not be recorded: a half-attached monster persists it twice")
	require.Equal(t, before.Conditions, m.ToData().Conditions,
		"and the drained blobs are back exactly as they were")
}

// TestRequireNameableTraitRefusesAnAnonymousCondition pins the early check that
// makes the above true by construction.
//
// It is exercised directly rather than through AttachMonster because it cannot
// be reached from outside: LoadJSON is the only constructor in that loop, and
// both contract tests in this module pin every type it can build to a non-nil
// ref. The check is the guarantee those tests describe, enforced rather than
// argued — the same reason Monster.AddLoadedCondition asks again at the door.
func TestRequireNameableTraitRefusesAnAnonymousCondition(t *testing.T) {
	require.NoError(t, requireNameableTrait(Immunity("gob-1", "poison")))

	err := requireNameableTrait(anonymousTrait{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "anonymousTrait",
		"the type is the only identification available for something that cannot name itself")

	require.Error(t, requireNameableTrait(nil))
}

// anonymousTrait breaks ConditionBehavior's "must never return nil" contract.
type anonymousTrait struct{}

func (anonymousTrait) Ref() *core.Ref { return nil }

func (anonymousTrait) IsApplied() bool { return false }

func (anonymousTrait) Apply(_ context.Context, _ events.EventBus) error { return nil }

func (anonymousTrait) Remove(_ context.Context, _ events.EventBus) error { return nil }

func (anonymousTrait) ToJSON() (json.RawMessage, error) { return json.RawMessage(`{}`), nil }

var _ dnd5eEvents.ConditionBehavior = anonymousTrait{}
