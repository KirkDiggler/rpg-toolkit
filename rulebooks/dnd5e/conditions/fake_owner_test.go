// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"

// fakeConditionOwner stands in for the *character.Character a condition is
// handed at attach time.
//
// This is a legitimate fake where the gamectx.CharacterRegistry it replaced
// was not, and the difference is worth stating because these very tests are
// why the bug survived: they built a registry by hand, production never built
// one, and so Unarmored Defense passed here while returning an error into
// every real AC fold. The owner handle is supplied on every path that attaches
// a condition, so standing in for it tests the same wiring the game uses.
//
// The end-to-end proof that it is wired lives in the session module, which is
// the only place resolution.Resolve can be driven from.
type fakeConditionOwner struct {
	scores shared.AbilityScores
	shield bool

	// dirtied counts MarkDirty calls, so a test can assert that a condition
	// which changed its own persisted state said so.
	dirtied int
}

func (f *fakeConditionOwner) AbilityScores() shared.AbilityScores { return f.scores }
func (f *fakeConditionOwner) HasShieldEquipped() bool             { return f.shield }
func (f *fakeConditionOwner) MarkDirty()                          { f.dirtied++ }
