// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// scriptedRoller answers every d20 with one face and counts how often it was
// asked, so a test can see whose dice a trait rolled.
type scriptedRoller struct {
	face  int
	calls int
}

var _ dice.Roller = (*scriptedRoller)(nil)

func (r *scriptedRoller) Roll(context.Context, int) (int, error) {
	r.calls++
	return r.face, nil
}

func (r *scriptedRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	r.calls++
	faces := make([]int, count)
	for i := range faces {
		faces[i] = r.face
	}
	return faces, nil
}

func (r *scriptedRoller) RollWithAdvantage(context.Context, int) (int, int, int, error) {
	r.calls++
	return r.face, r.face, r.face, nil
}

// zombieWithUndeadFortitude is a monster whose sheet holds one Undead
// Fortitude blob. No stat block authors the trait today, so the fixture
// marshals the blob itself.
func zombieWithUndeadFortitude(t *testing.T) *monster.Data {
	t.Helper()
	blob, err := json.Marshal(conditions.UndeadFortitudeData{
		Ref: refs.MonsterTraits.UndeadFortitude(), OwnerID: "zombie-1", ConModifier: 3,
	})
	require.NoError(t, err)

	return &monster.Data{
		ID: "zombie-1", Name: "Zombie", HitPoints: 22, MaxHitPoints: 22, ArmorClass: 8,
		Conditions: []json.RawMessage{blob},
	}
}

// dropsToZero publishes the damage that takes zombie-1 to 0 hit points.
func dropsToZero(ctx context.Context, bus events.EventBus) error {
	return dnd5eEvents.DamageReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: "zombie-1", SourceID: "pc-1", Amount: 22, DamageType: damage.Slashing,
	})
}

// TestUndeadFortitudeRollsTheAttachRoller: a loaded trait carries no roller, so
// attach binds the one it was handed, and the save rolls that roller's face.
func TestUndeadFortitudeRollsTheAttachRoller(t *testing.T) {
	ctx := context.Background()
	m, err := LoadMonster(ctx, zombieWithUndeadFortitude(t))
	require.NoError(t, err)

	roller := &scriptedRoller{face: 17}
	bus := events.NewEventBus()
	require.NoError(t, AttachMonster(ctx, m, bus, roller))

	require.NoError(t, dropsToZero(ctx, bus))
	require.Equal(t, 1, roller.calls, "the save is rolled with the roller attach bound")
}

// TestUndeadFortitudeWithNoRollerRefusesItsSave: attached with no roller the
// trait has none to roll, and the same damage is an error rather than a roll of
// a default nobody can see.
func TestUndeadFortitudeWithNoRollerRefusesItsSave(t *testing.T) {
	ctx := context.Background()
	m, err := LoadMonster(ctx, zombieWithUndeadFortitude(t))
	require.NoError(t, err)

	bus := events.NewEventBus()
	require.NoError(t, AttachMonster(ctx, m, bus, nil))

	err = dropsToZero(ctx, bus)
	require.Error(t, err)
	require.ErrorContains(t, err, "no roller")
}
