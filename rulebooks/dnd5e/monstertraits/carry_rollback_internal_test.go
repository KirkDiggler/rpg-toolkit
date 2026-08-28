// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

var errUnwritable = errors.New("this condition cannot be written down")

// unwritableCondition is a free reaction whose blob cannot be produced, which is
// the only way into carryingFreeReactions' failure path.
type unwritableCondition struct{}

func (unwritableCondition) Ref() *core.Ref  { return refs.Conditions.OpportunityAttack() }
func (unwritableCondition) IsApplied() bool { return false }

func (unwritableCondition) Apply(context.Context, events.EventBus) error  { return nil }
func (unwritableCondition) Remove(context.Context, events.EventBus) error { return nil }

func (unwritableCondition) ToJSON() (json.RawMessage, error) { return nil, errUnwritable }

// By the time the carry runs, the trait blobs have been DRAINED off the monster
// and the sheet keeper has been APPLIED. Returning from there without undoing
// both leaves the monster mutated by an attach that failed: its traits gone and
// its keeper still listening to the world.
//
// Every other failure in AttachMonster rolls back. This one used to be the
// exception, which is exactly the kind of gap a rollback contract cannot have
// one of.
func TestAFailedCarryRollsTheWholeAttachBack(t *testing.T) {
	ctx := context.Background()

	restore := freeReactions
	freeReactions = []freeReaction{{
		ref:   refs.Conditions.OpportunityAttack(),
		build: func(string) dnd5eEvents.ConditionBehavior { return unwritableCondition{} },
	}}
	defer func() { freeReactions = restore }()

	authored, err := Immunity("gob-1", "poison").ToJSON()
	require.NoError(t, err)

	m, err := monster.Load(ctx, &monster.Data{
		ID: "gob-1", Name: "Goblin", HitPoints: 7, MaxHitPoints: 7, ArmorClass: 15,
		Conditions: []json.RawMessage{authored},
	})
	require.NoError(t, err)
	before := m.ToData()

	bus := events.NewEventBus()
	err = AttachMonster(ctx, m, bus, nil)

	require.ErrorIs(t, err, errUnwritable)
	require.Equal(t, before.Conditions, m.ToData().Conditions,
		"the drained trait blobs are back on the sheet")
	require.Empty(t, m.GetConditions(), "and nothing was attached")

	// The keeper came back off too: a monster that is not attached does not
	// hear the world.
	require.NoError(t, dnd5eEvents.DamageReceivedTopic.On(bus).Publish(ctx,
		dnd5eEvents.DamageReceivedEvent{TargetID: m.GetID(), Amount: 3}))
	require.Equal(t, 7, m.ToData().HitPoints, "a rolled-back keeper is not still listening")
}
