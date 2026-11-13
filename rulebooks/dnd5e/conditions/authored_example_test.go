// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// The table chooses test actions, not effect implementations. The same public
// loader/binder and tiny source drive every row; output comes from real folds.
func cookbookTranscript() ([]string, error) {
	defs, err := loadCookbook()
	if err != nil {
		return nil, err
	}
	rows := []string{}
	for _, tc := range []struct {
		id, user string
		end      func(*definitionWorld) error
	}{
		{"rage-ring", "A", func(w *definitionWorld) error { return w.rest("A") }},
		{"rage-scroll", "A", func(w *definitionWorld) error {
			if err := w.turnEnd("B"); err != nil {
				return err
			}
			return w.turnEnd("B")
		}},
		{"physical-ward", "A", func(w *definitionWorld) error { return w.rest("A") }},
		{"damage-gift", "A", func(w *definitionWorld) error { return w.rest("A") }},
		{"kitchen-sausage", "B", func(w *definitionWorld) error { return w.turnStart("B") }},
	} {
		row, err := func() (string, error) {
			w, err := newDefinitionWorld()
			if err != nil {
				return "", err
			}
			defer func() { _ = w.close() }()
			source := newFixtureSource("item-"+tc.id, tc.user, tc.id)
			grant, err := useDefinition(context.Background(), definitionUseInput{Source: source, RecipientID: "B", ApplicationID: "use", Definitions: defs, Participants: bindingParticipants(w.sheets)})
			if err != nil {
				return "", err
			}
			if err := w.apply(grant.Prepared); err != nil {
				return "", err
			}
			owner, err := w.coordinator("use")
			if err != nil {
				return "", err
			}
			damageOut, err := w.damageBonus("B", "C", abilities.STR, true, true)
			if err != nil {
				return "", err
			}
			resistance, err := w.incoming("B", damage.Slashing, 9)
			if err != nil {
				return "", err
			}
			own, err := w.frame("B", "C")
			if err != nil {
				return "", err
			}
			ownOut, err := w.attack(own)
			if err != nil {
				return "", err
			}
			target, err := w.frame("C", "B")
			if err != nil {
				return "", err
			}
			targetOut, err := w.attack(target)
			if err != nil {
				return "", err
			}
			if err := tc.end(w); err != nil {
				return "", err
			}
			if err := w.binding.CheckComplete(); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s: owner=%s damage=%d physical=%d attacks=%d/%d uses=%d ended=%t", tc.id, owner, totalFixtureDamage(damageOut), resistance.Types[0].Taken, len(ownOut.AdvantageSources), len(targetOut.AdvantageSources), source.Budget.Current, len(w.holder("B").GetConditions()) == 0), nil
		}()
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
func Example_dataAuthoredEffects() {
	rows, err := cookbookTranscript()
	if err != nil {
		panic(err)
	}
	for _, row := range rows {
		fmt.Println(row)
	}
	// Output:
	// rage-ring: owner=A damage=12 physical=4 attacks=0/0 uses=0 ended=true
	// rage-scroll: owner=B damage=12 physical=4 attacks=0/0 uses=0 ended=true
	// physical-ward: owner=A damage=10 physical=4 attacks=0/0 uses=0 ended=true
	// damage-gift: owner=A damage=12 physical=9 attacks=0/0 uses=0 ended=true
	// kitchen-sausage: owner=B damage=10 physical=9 attacks=1/1 uses=0 ended=true
}
