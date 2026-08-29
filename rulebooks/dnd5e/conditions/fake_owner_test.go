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

	// hasEconomy says whether this sheet keeps an action economy at all, and
	// reactions is the slot count it keeps if it does.
	//
	// Two fields rather than one because the two kinds answer CanReact for
	// different reasons, and a fake that flattened them could not tell the
	// reasons apart. A character with no slots left refuses; a monster has
	// nothing that could refuse, so it never does. The zero value is the
	// monster — which is also the sheet a test that does not care about
	// reactions should get, since it is the one that never gets in the way.
	hasEconomy bool
	reactions  int

	hp, maxHP        int
	ac               int
	proficiencyBonus int
}

func (f *fakeConditionOwner) GetID() string           { return f.id }
func (f *fakeConditionOwner) GetHitPoints() int       { return f.hp }
func (f *fakeConditionOwner) GetMaxHitPoints() int    { return f.maxHP }
func (f *fakeConditionOwner) AC() int                 { return f.ac }
func (f *fakeConditionOwner) HasShieldEquipped() bool { return f.shield }

// CanReact answers the way the two real sheets do: a character out of its
// slots refuses, and a monster has no economy to refuse with.
func (f *fakeConditionOwner) CanReact() bool {
	if !f.hasEconomy {
		return true
	}

	return f.reactions > 0
}
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
