// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coremock "github.com/KirkDiggler/rpg-toolkit/core/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/spells"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/spells/mock"
)

func TestSpellCastCompleteEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCaster := coremock.NewMockEntity(ctrl)
	mockTarget := coremock.NewMockEntity(ctrl)
	mockSpell := mock.NewMockSpell(ctrl)

	event := &spells.SpellCastCompleteEvent{
		GameEvent: *events.NewGameEvent(spells.EventSpellCastComplete, mockCaster, mockTarget),
		Caster:    mockCaster,
		Spell:     mockSpell,
		Targets:   []core.Entity{mockTarget},
		SlotLevel: 3,
	}

	assert.Equal(t, spells.EventSpellCastComplete, event.Type())
	assert.Equal(t, mockCaster, event.Caster)
	assert.Equal(t, mockSpell, event.Spell)
	assert.Len(t, event.Targets, 1)
	assert.Equal(t, mockTarget, event.Targets[0])
	assert.Equal(t, 3, event.SlotLevel)
}

func TestSpellCastFailedEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCaster := coremock.NewMockEntity(ctrl)
	mockTarget := coremock.NewMockEntity(ctrl)
	mockSpell := mock.NewMockSpell(ctrl)
	testError := errors.New("not enough spell slots")

	event := &spells.SpellCastFailedEvent{
		GameEvent: *events.NewGameEvent(spells.EventSpellCastFailed, mockCaster, mockTarget),
		Caster:    mockCaster,
		Spell:     mockSpell,
		Targets:   []core.Entity{mockTarget},
		SlotLevel: 3,
		Error:     testError,
	}

	assert.Equal(t, spells.EventSpellCastFailed, event.Type())
	assert.Equal(t, mockCaster, event.Caster)
	assert.Equal(t, mockSpell, event.Spell)
	assert.Equal(t, testError, event.Error)
}

func TestSpellAttackEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAttacker := coremock.NewMockEntity(ctrl)
	mockTarget := coremock.NewMockEntity(ctrl)
	mockSpell := mock.NewMockSpell(ctrl)

	event := &spells.SpellAttackEvent{
		GameEvent:  *events.NewGameEvent(spells.EventSpellAttack, mockAttacker, mockTarget),
		Attacker:   mockAttacker,
		Target:     mockTarget,
		Spell:      mockSpell,
		AttackRoll: 18,
		Hit:        true,
		Critical:   false,
	}

	assert.Equal(t, spells.EventSpellAttack, event.Type())
	assert.Equal(t, mockAttacker, event.Attacker)
	assert.Equal(t, mockTarget, event.Target)
	assert.Equal(t, 18, event.AttackRoll)
	assert.True(t, event.Hit)
	assert.False(t, event.Critical)
}

func TestSpellDamageEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSource := coremock.NewMockEntity(ctrl)
	mockTarget := coremock.NewMockEntity(ctrl)
	mockSpell := mock.NewMockSpell(ctrl)

	event := &spells.SpellDamageEvent{
		GameEvent:  *events.NewGameEvent(spells.EventSpellDamage, mockSource, mockTarget),
		Source:     mockSource,
		Target:     mockTarget,
		Spell:      mockSpell,
		Damage:     24,
		DamageType: "fire",
	}

	assert.Equal(t, spells.EventSpellDamage, event.Type())
	assert.Equal(t, mockSource, event.Source)
	assert.Equal(t, mockTarget, event.Target)
	assert.Equal(t, 24, event.Damage)
	assert.Equal(t, "fire", event.DamageType)
}

func TestSpellSaveEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTarget := coremock.NewMockEntity(ctrl)
	mockSpell := mock.NewMockSpell(ctrl)

	event := &spells.SpellSaveEvent{
		GameEvent: *events.NewGameEvent(spells.EventSpellSave, nil, mockTarget),
		Target:    mockTarget,
		Spell:     mockSpell,
		SaveType:  "dexterity",
		DC:        15,
		Result:    12,
		Success:   false,
	}

	assert.Equal(t, spells.EventSpellSave, event.Type())
	assert.Equal(t, mockTarget, event.Target)
	assert.Equal(t, "dexterity", event.SaveType)
	assert.Equal(t, 15, event.DC)
	assert.Equal(t, 12, event.Result)
	assert.False(t, event.Success)
}

func TestConcentrationBrokenEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCaster := coremock.NewMockEntity(ctrl)
	mockSpell := mock.NewMockSpell(ctrl)

	event := &spells.ConcentrationBrokenEvent{
		GameEvent: *events.NewGameEvent(spells.EventConcentrationBroken, mockCaster, nil),
		Caster:    mockCaster,
		Spell:     mockSpell,
		Reason:    "took damage",
	}

	assert.Equal(t, spells.EventConcentrationBroken, event.Type())
	assert.Equal(t, mockCaster, event.Caster)
	assert.Equal(t, mockSpell, event.Spell)
	assert.Equal(t, "took damage", event.Reason)
}

func TestConcentrationCheckEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCaster := coremock.NewMockEntity(ctrl)
	mockSpell := mock.NewMockSpell(ctrl)

	event := &spells.ConcentrationCheckEvent{
		GameEvent: *events.NewGameEvent(spells.EventConcentrationCheck, mockCaster, nil),
		Caster:    mockCaster,
		Spell:     mockSpell,
		DC:        10,
		Result:    15,
		Success:   true,
		Damage:    12,
	}

	assert.Equal(t, spells.EventConcentrationCheck, event.Type())
	assert.Equal(t, mockCaster, event.Caster)
	assert.Equal(t, 10, event.DC)
	assert.True(t, event.Success)
	assert.Equal(t, 12, event.Damage)
}
