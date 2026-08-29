// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// fakeConditionOwner stands in for the *character.Character a condition reads
// itself as.
//
// It is a whole [combat.Member], and that is the point rather than
// thoroughness: a condition no longer receives a hand-picked view of its owner,
// it looks itself up in the cast and gets back the same surface it would get
// for anybody else. A fake narrower than the real surface would let a rule read
// something production could not offer, which is the shape of the bug this
// whole migration exists to remove — these very tests once built a registry by
// hand that production never built, so Unarmored Defense passed here while
// contributing nothing to a real AC fold.
//
// # It is no longer a whole Combatant, and that is the same argument
//
// It used to be, carrying an ApplyDamage that no test ever swung at and a dirty
// trio nothing ever read, present only because the interface demanded them.
// The cast hands out [combat.Member] now, so a fake still answering the
// keeper's surface would be WIDER than production — a test could write through
// a sheet the real rule cannot, and pass. Wrong in the other direction is still
// wrong: the fake is the surface, exactly.
//
// The end-to-end proof that the cast is installed lives in the session module,
// which is the only place resolution's door can be driven from.
type fakeConditionOwner struct {
	id     string
	scores shared.AbilityScores
	shield bool

	// canReact is a field rather than a constant for the reason shield is: it
	// is a question the sheet answers differently depending on its state, and
	// a fake that can only give one answer cannot stand in for both. False is
	// the zero on purpose — a character with no fight around it carries no
	// action economy and reports no slots of any kind.
	canReact bool

	hp, maxHP        int
	ac               int
	proficiencyBonus int
}

func (f *fakeConditionOwner) GetID() string                       { return f.id }
func (f *fakeConditionOwner) GetHitPoints() int                   { return f.hp }
func (f *fakeConditionOwner) GetMaxHitPoints() int                { return f.maxHP }
func (f *fakeConditionOwner) AC() int                             { return f.ac }
func (f *fakeConditionOwner) HasShieldEquipped() bool             { return f.shield }
func (f *fakeConditionOwner) CanReact() bool                      { return f.canReact }
func (f *fakeConditionOwner) AbilityScores() shared.AbilityScores { return f.scores }
func (f *fakeConditionOwner) ProficiencyBonus() int               { return f.proficiencyBonus }
func (f *fakeConditionOwner) PassivePerception() int              { return 10 }

// Ensure the fake really is the whole surface a condition reads a member
// through. Stated rather than left to the one call site that happens to need
// it, so a Member that grows a question fails here.
//
// combat.Member, not combat.Combatant, and the difference is now load-bearing:
// this line failing after a widening is how we would learn that the cast had
// started handing rules the keeper's surface again.
var _ combat.Member = (*fakeConditionOwner)(nil)
