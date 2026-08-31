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
// combat, so the dependency only runs one way. It is the same force that puts
// [AssembleAttack] here: compilers stay beside the sheet and hand back neutral
// data.
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
// buys four. When [CanTwoWeaponAttack] is true, the same action also banks the
// one two-weapon bonus attack that a later bonus-action declaration may spend.
//
// Note what it does NOT cost: the swings themselves. The action banks capacity
// and [CostOfStrike] spends it, which is why the second swing needs no second
// action and the third is refused without anybody counting to three.
func CostOfAttack(c *Character) (*combat.SpendProfile, error) {
	if c == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no character to price the Attack action for")
	}

	grants := map[combat.CapacityType]int{
		combat.CapacityAttack: 1 + c.GetExtraAttacksCount(),
	}
	if CanTwoWeaponAttack(c) {
		grants[combat.CapacityOffHandAttack] = 1
	}

	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{
			coreCombat.ActionStandard: 1,
		},
		Grants: grants,
	}, nil
}

// CostOfStrike compiles what one swing costs this character: a single banked
// attack and no slot at all.
//
// This is the capacity half of a shared attack definition's optional price,
// compiled as data the gate can charge rather than behavior an action object
// must expose through accessors.
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

// CostOfTwoWeaponAttack compiles the complete price of the bonus attack
// granted by two-weapon fighting: one bonus-action slot and one granted
// off-hand attack capacity.
func CostOfTwoWeaponAttack(c *Character) (*combat.SpendProfile, error) {
	if c == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no character to price a two-weapon attack for")
	}
	if !CanTwoWeaponAttack(c) {
		return nil, rpgerr.New(
			rpgerr.CodeInvalidArgument, "two-weapon attack requires two light melee weapons",
		)
	}

	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{
			coreCombat.ActionBonus: 1,
		},
		Capacity: map[combat.CapacityType]int{
			combat.CapacityOffHandAttack: 1,
		},
	}, nil
}

// CostOfSwing composes the Attack action and one Strike into the single atomic
// price charged for a swing. An already-banked attack costs only its capacity;
// otherwise the first swing is netted from the Attack action's grant.
func CostOfSwing(c *Character) (*combat.SpendProfile, error) {
	strike, err := CostOfStrike(c)
	if err != nil {
		return nil, err
	}
	if combat.CanPay(c, strike) {
		return strike, nil
	}

	action, err := CostOfAttack(c)
	if err != nil {
		return nil, err
	}
	return asOnePayment(action, strike), nil
}

func asOnePayment(action, strike *combat.SpendProfile) *combat.SpendProfile {
	wants := sum(action.Capacity, strike.Capacity)
	banks := sum(action.Grants, strike.Grants)
	merged := &combat.SpendProfile{
		Slots:    sum(action.Slots, strike.Slots),
		Pools:    sum(action.Pools, strike.Pools),
		Requires: larger(action.Requires, strike.Requires),
	}

	for key, want := range wants {
		if owed := want - banks[key]; owed > 0 {
			merged.Capacity = put(merged.Capacity, key, owed)
		}
	}
	for key, banked := range banks {
		if left := banked - wants[key]; left > 0 {
			merged.Grants = put(merged.Grants, key, left)
		}
	}
	return merged
}

func sum[K ~string](a, b map[K]int) map[K]int {
	var out map[K]int
	for key, quantity := range a {
		out = put(out, key, quantity)
	}
	for key, quantity := range b {
		out = put(out, key, out[key]+quantity)
	}
	return out
}

func larger[K ~string](a, b map[K]int) map[K]int {
	var out map[K]int
	for key, quantity := range a {
		out = put(out, key, quantity)
	}
	for key, quantity := range b {
		if quantity > out[key] {
			out = put(out, key, quantity)
		}
	}
	return out
}

func put[K ~string](values map[K]int, key K, quantity int) map[K]int {
	if values == nil {
		values = make(map[K]int)
	}
	values[key] = quantity
	return values
}
