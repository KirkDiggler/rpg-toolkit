// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"reflect"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

func TestNewDamageChainEventCopiesMarkedWeaponMetadata(t *testing.T) {
	got := NewDamageChainEvent(DamageChainInput{
		WeaponDamageDice: "1d8",
		WeaponDamageType: damage.Fire,
	})

	if got.WeaponDamageDice != "1d8" {
		t.Fatalf("WeaponDamageDice = %q, want %q", got.WeaponDamageDice, "1d8")
	}
	if got.WeaponDamageType != damage.Fire {
		t.Fatalf("WeaponDamageType = %q, want %q", got.WeaponDamageType, damage.Fire)
	}
}

func TestNewDamageChainEventCarriesFrame(t *testing.T) {
	frame := contributions.Frame{
		Actor:  "rogue",
		Target: contributions.Known("goblin"),
		Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)},
		Pairs: []contributions.PairFacts{{
			From: "goblin", To: "fighter", DistanceCells: contributions.Known(1.0),
		}},
		Complete: true,
	}

	got := NewDamageChainEvent(DamageChainInput{AttackerID: "rogue", Frame: frame})

	if !reflect.DeepEqual(got.Frame, frame) {
		t.Fatalf("Frame = %+v, want %+v", got.Frame, frame)
	}
	frame.Pairs[0].To = "changed"
	if got.Frame.Pairs[0].To != "fighter" {
		t.Fatalf("event frame shares the input's pairs: %q", got.Frame.Pairs[0].To)
	}
}
