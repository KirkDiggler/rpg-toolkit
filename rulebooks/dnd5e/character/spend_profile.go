// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// The cost compilers: where an action's price stops being a class table and
// becomes data.
//
// This is the only place in the economy that knows what a fighter is. The gate
// charges a [combat.SpendProfile] and cannot tell a level-5 fighter's Attack
// action from a level-1 fighter's — it sees a profile that banks two and a
// profile that banks one. Everything class-dynamic lives on this side of that
// line, and the line is what keeps the machinery that spends free of the
// rulebook that prices.
//
// They live in this package rather than in combat for the plainest of reasons:
// a compiler reads the SHEET, and combat cannot see a sheet — character imports
// combat, so the dependency only runs one way. It is the same force that put
// AttackFromCharacter in the resolution module: the compiler goes where the
// sheet is, and hands back something neutral.
//
// The conventions follow that compiler's. A concrete sheet in, a neutral
// profile out, nil refused by name, and only STATIC facts compiled — nothing
// here asks what the character is standing next to.

// CostOfAttack compiles what the Attack action costs this character.
//
// One action, and a bank of attacks to spend it on: 1 + whatever Extra Attack
// grants at this class and level, which is the table
// [Character.GetExtraAttacksCount] already keeps. A level-5 fighter's Attack
// action buys two swings; a level-1 fighter's buys one; a level-20 fighter's
// buys four.
//
// Note what it does NOT cost: the swings themselves. The action banks capacity
// and [CostOfStrike] spends it, which is why the second swing needs no second
// action and the third is refused without anybody counting to three.
func CostOfAttack(c *Character) (*combat.SpendProfile, error) {
	if c == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no character to price the Attack action for")
	}

	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{
			coreCombat.ActionStandard: 1,
		},
		Grants: map[combat.CapacityType]int{
			combat.CapacityAttack: 1 + c.GetExtraAttacksCount(),
		},
	}, nil
}

// CostOfStrike compiles what one swing costs this character: a single banked
// attack and no slot at all.
//
// Those are the same two facts actions.Strike already declares about itself —
// ActionFree, CapacityAttack — restated as a price the gate can charge instead
// of a pair of accessors something has to interpret.
//
// The sheet is not read today. It is taken anyway, because the price of a swing
// is a character question the moment anything makes it one, and a compiler that
// has to grow a parameter is a compiler every caller has to be found for.
func CostOfStrike(c *Character) (*combat.SpendProfile, error) {
	if c == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no character to price a strike for")
	}

	return &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{
			combat.CapacityAttack: 1,
		},
	}, nil
}
