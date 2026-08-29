// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
)

// TestRequireNameableRefusesAConditionThatCannotNameItself covers the door
// shared by both of this sheet's admission paths — addCondition on the bus
// path, and loadEffects on the load path.
//
// The BUS path is driven end to end by
// TestSheetKeeperSuite/TestARefLessConditionIsRefusedAtTheDoor. The LOAD path
// cannot be: conditions.LoadJSON is the only constructor loadEffects uses, and
// conditions.TestEveryConditionRefMatchesItsToJSON pins a non-nil ref for all
// 22 types it routes, so no persisted blob can produce a nameless condition to
// feed it. The check is kept anyway — a load path that trusts another
// package's test to hold its invariant is exactly the arrangement that let
// monstertraits' four self-built trait types sit outside that same argument
// until review caught them — and this is the honest test for it: the function,
// directly, rather than a fixture that cannot exist.
func TestRequireNameableRefusesAConditionThatCannotNameItself(t *testing.T) {
	require.NoError(t, requireNameable(&namedConditionStub{}, "char-1"))

	err := requireNameable(&anonymousConditionStub{}, "char-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "anonymousConditionStub",
		"the type is the only identification available for something that cannot name itself")
	require.Contains(t, err.Error(), "char-1", "and the error says whose sheet refused it")

	require.Error(t, requireNameable(nil, "char-1"))
}

// TestAddConditionRefusesAtTheDoor pins that the sheet's own admission method
// asks, not merely the handler in front of it.
func TestAddConditionRefusesAtTheDoor(t *testing.T) {
	c := &Character{id: "char-1"}

	require.Error(t, c.addCondition(&anonymousConditionStub{}))
	require.Empty(t, c.conditions, "nothing was admitted")

	require.NoError(t, c.addCondition(&namedConditionStub{}))
	require.Len(t, c.conditions, 1, "and a condition that can name itself is admitted")
}

type namedConditionStub struct{}

func (*namedConditionStub) Ref() *core.Ref { return refs.Conditions.Dodging() }

func (*namedConditionStub) IsApplied() bool { return false }

func (*namedConditionStub) Apply(_ context.Context, _ events.EventBus) error { return nil }

func (*namedConditionStub) Remove(_ context.Context, _ events.EventBus) error { return nil }

func (*namedConditionStub) ToJSON() (json.RawMessage, error) { return json.RawMessage(`{}`), nil }

type anonymousConditionStub struct{}

func (*anonymousConditionStub) Ref() *core.Ref { return nil }

func (*anonymousConditionStub) IsApplied() bool { return false }

func (*anonymousConditionStub) Apply(_ context.Context, _ events.EventBus) error { return nil }

func (*anonymousConditionStub) Remove(_ context.Context, _ events.EventBus) error { return nil }

func (*anonymousConditionStub) ToJSON() (json.RawMessage, error) { return json.RawMessage(`{}`), nil }

var (
	_ dnd5eEvents.ConditionBehavior = (*namedConditionStub)(nil)
	_ dnd5eEvents.ConditionBehavior = (*anonymousConditionStub)(nil)
)
