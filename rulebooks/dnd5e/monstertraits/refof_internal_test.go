// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// namelessCondition breaks ConditionBehavior's "Ref must never return nil"
// contract, which is the only way to reach refOf's guard.
type namelessCondition struct{ ref *core.Ref }

func (n *namelessCondition) Ref() *core.Ref                                { return n.ref }
func (n *namelessCondition) IsApplied() bool                               { return false }
func (n *namelessCondition) Apply(context.Context, events.EventBus) error  { return nil }
func (n *namelessCondition) Remove(context.Context, events.EventBus) error { return nil }
func (n *namelessCondition) ToJSON() (json.RawMessage, error)              { return json.RawMessage(`{}`), nil }

// The guard is defensive and therefore easy to leave unexercised — mutation
// testing caught exactly that, with "return *condition.Ref()" surviving the
// whole suite.
//
// What it buys is stated in refOf's own doc and is worth holding: attribution is
// a LABEL. A condition that breaks its contract should cost the label, not take
// down an attach that would otherwise have worked, and a nil dereference here
// would panic through AttachMonster with no indication that the cause was a
// misbehaving effect rather than the monster.
func TestRefOfAnswersForAConditionThatWillNotNameItself(t *testing.T) {
	require.NotPanics(t, func() {
		require.Equal(t, core.Ref{}, refOf(&namelessCondition{ref: nil}),
			"a nameless condition costs attribution, not the attach")
	})

	named := refs.MonsterTraits.PackTactics()
	require.Equal(t, *named, refOf(&namelessCondition{ref: named}),
		"and a condition that does name itself is passed straight through")
}
