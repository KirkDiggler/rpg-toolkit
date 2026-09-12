// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// SpellPayment combines a declared casting with its independent resource price.
// Turn identifies the active creature's turn, not the payer's last turn start.
// The composition must validate targets and refresh the economy when appropriate
// before asking for payment. This operation never refreshes it implicitly.
type SpellPayment struct {
	Turn    string
	Casting combat.SpellCasting
	Price   *combat.SpendProfile
}

// CanPaySpell checks spell-turn legality and affordability without mutation.
func (c *Character) CanPaySpell(input SpellPayment) error {
	if c.actionEconomy == nil {
		return fmt.Errorf("spell payment requires a combat action economy")
	}
	if _, err := c.actionEconomy.Spellcasting.AfterCast(input.Turn, input.Casting); err != nil {
		return err
	}
	if err := input.Price.Validate(); err != nil {
		return err
	}
	if !combat.CanPay(c, input.Price) {
		return fmt.Errorf("cannot afford spell cost")
	}
	return nil
}

// PaySpell checks the same-turn rule, pays atomically through the existing gate,
// then records the cast and marks the sheet dirty. Neither failed payment nor a
// rule refusal consumes resources or alters casting history. A paid cast counts
// even when delivery subsequently has no effect, such as healing an undead.
func (c *Character) PaySpell(input SpellPayment) error {
	if c.actionEconomy == nil {
		return fmt.Errorf("spell payment requires a combat action economy")
	}
	next, err := c.actionEconomy.Spellcasting.AfterCast(input.Turn, input.Casting)
	if err != nil {
		return err
	}
	if err := combat.Pay(c, input.Price); err != nil {
		return err
	}
	c.actionEconomy.Spellcasting = next
	c.economyChanged()
	return nil
}
