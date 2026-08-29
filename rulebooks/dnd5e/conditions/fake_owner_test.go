// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// fakeConditionOwner stands in for the *character.Character a condition reads
// itself as.
//
// It is a whole [combat.Combatant], and that is the point rather than
// thoroughness: a condition no longer receives a hand-picked view of its owner,
// it looks itself up in the cast and gets back the same surface it would get
// for anybody else. A fake narrower than the real surface would let a rule read
// something production could not offer, which is the shape of the bug this
// whole migration exists to remove — these very tests once built a registry by
// hand that production never built, so Unarmored Defense passed here while
// contributing nothing to a real AC fold.
//
// The end-to-end proof that the cast is installed lives in the session module,
// which is the only place resolution's door can be driven from.
type fakeConditionOwner struct {
	id     string
	scores shared.AbilityScores
	shield bool

	hp, maxHP        int
	ac               int
	proficiencyBonus int

	// dirtied counts MarkDirty calls, so a test can assert that a condition
	// which changed its own persisted state said so.
	dirtied int
}

func (f *fakeConditionOwner) GetID() string                       { return f.id }
func (f *fakeConditionOwner) GetHitPoints() int                   { return f.hp }
func (f *fakeConditionOwner) GetMaxHitPoints() int                { return f.maxHP }
func (f *fakeConditionOwner) AC() int                             { return f.ac }
func (f *fakeConditionOwner) HasShieldEquipped() bool             { return f.shield }
func (f *fakeConditionOwner) IsDirty() bool                       { return f.dirtied > 0 }
func (f *fakeConditionOwner) MarkClean()                          { f.dirtied = 0 }
func (f *fakeConditionOwner) AbilityScores() shared.AbilityScores { return f.scores }
func (f *fakeConditionOwner) ProficiencyBonus() int               { return f.proficiencyBonus }
func (f *fakeConditionOwner) PassivePerception() int              { return 10 }
func (f *fakeConditionOwner) MarkDirty()                          { f.dirtied++ }

// ApplyDamage is present because Combatant requires it. Nothing in these tests
// swings at the fake, so it reports the hit points unchanged rather than
// inventing arithmetic a test could come to rely on.
func (f *fakeConditionOwner) ApplyDamage(
	_ context.Context, _ *combat.ApplyDamageInput,
) *combat.ApplyDamageResult {
	return &combat.ApplyDamageResult{CurrentHP: f.hp, PreviousHP: f.hp}
}

// Ensure the fake really is the whole surface a condition reads a member
// through. Stated rather than left to the one call site that happens to need
// it, so a Combatant that grows a question fails here.
var _ combat.Combatant = (*fakeConditionOwner)(nil)
