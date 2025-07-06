// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Event types for spell casting
const (
	// EventSpellCastAttempt is published when a spell cast is attempted.
	EventSpellCastAttempt = "spell.cast.attempt"
	// EventSpellCastStart is published when a spell cast begins.
	EventSpellCastStart = "spell.cast.start"
	// EventSpellCastComplete is published when a spell cast completes successfully.
	EventSpellCastComplete = "spell.cast.complete"
	// EventSpellCastFailed is published when a spell cast fails.
	EventSpellCastFailed = "spell.cast.failed"
	// EventSpellSave is published when a saving throw is made against a spell.
	EventSpellSave = "spell.save"
	// EventSpellAttack is published when a spell attack is made.
	EventSpellAttack = "spell.attack"
	// EventSpellDamage is published when a spell deals damage.
	EventSpellDamage = "spell.damage"
	// EventSpellHeal is published when a spell heals damage.
	EventSpellHeal = "spell.heal"
	// EventConcentrationCheck is published when concentration must be maintained.
	EventConcentrationCheck = "spell.concentration.check"
	// EventConcentrationBroken is published when concentration is lost.
	EventConcentrationBroken = "spell.concentration.broken"
)

// SpellCastAttemptEvent is published when a spell cast is attempted.
type SpellCastAttemptEvent struct {
	events.GameEvent
	Caster    core.Entity
	Spell     Spell
	SlotLevel int
}

// SpellCastStartEvent is published when a spell cast begins.
type SpellCastStartEvent struct {
	events.GameEvent
	Caster    core.Entity
	Spell     Spell
	Targets   []core.Entity
	SlotLevel int
}

// SpellCastCompleteEvent is published when a spell cast completes successfully.
type SpellCastCompleteEvent struct {
	events.GameEvent
	Caster    core.Entity
	Spell     Spell
	Targets   []core.Entity
	SlotLevel int
}

// SpellSaveEvent is published when a saving throw is made against a spell.
type SpellSaveEvent struct {
	events.GameEvent
	Target   core.Entity
	Spell    Spell
	SaveType string
	DC       int
	Result   int
	Success  bool
}

// SpellAttackEvent is published when a spell attack is made.
type SpellAttackEvent struct {
	events.GameEvent
	Attacker   core.Entity
	Target     core.Entity
	Spell      Spell
	AttackRoll int
	Hit        bool
	Critical   bool
}

// SpellDamageEvent is published when a spell deals damage.
type SpellDamageEvent struct {
	events.GameEvent
	Source     core.Entity
	Target     core.Entity
	Spell      Spell
	Damage     int
	DamageType string
}

// ConcentrationCheckEvent is published when concentration must be maintained.
type ConcentrationCheckEvent struct {
	events.GameEvent
	Caster  core.Entity
	Spell   Spell
	DC      int
	Damage  int // Damage that triggered the check
	Result  int
	Success bool
}
