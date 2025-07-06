// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coremock "github.com/KirkDiggler/rpg-toolkit/core/mock"
	eventsmock "github.com/KirkDiggler/rpg-toolkit/events/mock"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/spells"
)

func TestSimpleSpell_BasicProperties(t *testing.T) {
	config := spells.SimpleSpellConfig{
		ID:          "test-spell",
		Name:        "Test Spell",
		Level:       3,
		School:      "evocation",
		CastingTime: 6 * time.Second,
		Range:       120,
		Duration:    nil, // Instantaneous
		Description: "A test spell",
		Components: spells.CastingComponents{
			Verbal:    true,
			Somatic:   true,
			Material:  true,
			Materials: "a pinch of salt",
		},
		TargetType:    spells.TargetCreature,
		MaxTargets:    1,
		Ritual:        true,
		Concentration: false,
		Upcastable:    true,
	}

	spell := spells.NewSimpleSpell(config)

	assert.Equal(t, "test-spell", spell.GetID())
	assert.Equal(t, "spell", spell.GetType())
	assert.Equal(t, "Test Spell", spell.GetName())
	assert.Equal(t, 3, spell.Level())
	assert.Equal(t, "evocation", spell.School())
	assert.Equal(t, 6*time.Second, spell.CastingTime())
	assert.Equal(t, 120, spell.Range())
	assert.Nil(t, spell.Duration())
	assert.Equal(t, "A test spell", spell.Description())
	assert.True(t, spell.IsRitual())
	assert.False(t, spell.RequiresConcentration())
	assert.True(t, spell.CanBeUpcast())
	assert.Equal(t, spells.TargetCreature, spell.TargetType())
	assert.Equal(t, 1, spell.MaxTargets())

	components := spell.Components()
	assert.True(t, components.Verbal)
	assert.True(t, components.Somatic)
	assert.True(t, components.Material)
	assert.Equal(t, "a pinch of salt", components.Materials)
}

func TestSimpleSpell_AreaOfEffect(t *testing.T) {
	config := spells.SimpleSpellConfig{
		ID:     "fireball",
		Name:   "Fireball",
		Level:  3,
		School: "evocation",
		AreaOfEffect: &spells.AreaOfEffect{
			Shape:  spells.AreaSphere,
			Size:   20,
			Origin: "point",
		},
	}

	spell := spells.NewSimpleSpell(config)
	aoe := spell.AreaOfEffect()
	require.NotNil(t, aoe)
	assert.Equal(t, spells.AreaSphere, aoe.Shape)
	assert.Equal(t, 20, aoe.Size)
	assert.Equal(t, "point", aoe.Origin)
}

func TestSimpleSpell_Cast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := eventsmock.NewMockEventBus(ctrl)
	mockCaster := coremock.NewMockEntity(ctrl)
	mockTarget := coremock.NewMockEntity(ctrl)

	// Expect the Cast method to publish events
	mockBus.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil).Times(3) // attempt, start, complete

	castCalled := false
	config := spells.SimpleSpellConfig{
		ID:     "test-spell",
		Name:   "Test Spell",
		Level:  1,
		School: "divination",
		CastFunc: func(ctx spells.CastContext) error {
			castCalled = true
			assert.Equal(t, mockCaster, ctx.Caster)
			assert.Len(t, ctx.Targets, 1)
			assert.Equal(t, mockTarget, ctx.Targets[0])
			assert.Equal(t, 1, ctx.SlotLevel)
			assert.Equal(t, mockBus, ctx.Bus)
			return nil
		},
	}

	spell := spells.NewSimpleSpell(config)

	ctx := spells.CastContext{
		Caster:    mockCaster,
		Targets:   []core.Entity{mockTarget},
		SlotLevel: 1,
		Bus:       mockBus,
		Metadata:  make(map[string]interface{}),
	}

	err := spell.Cast(ctx)
	require.NoError(t, err)
	assert.True(t, castCalled)
}

func TestSimpleSpell_Cantrip(t *testing.T) {
	config := spells.SimpleSpellConfig{
		ID:     "light",
		Name:   "Light",
		Level:  0, // Cantrip
		School: "evocation",
	}

	spell := spells.NewSimpleSpell(config)
	assert.Equal(t, 0, spell.Level())
}
