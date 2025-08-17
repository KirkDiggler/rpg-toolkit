// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Event refs for combat events
var (
	TurnEndEventRef     = mustParseRef("dnd5e:combat:turn-end")
	AttackEventRef      = mustParseRef("dnd5e:combat:attack")
	DamageTakenEventRef = mustParseRef("dnd5e:combat:damage-taken")
)

func mustParseRef(s string) *core.Ref {
	ref, err := core.ParseString(s)
	if err != nil {
		panic(err)
	}
	return ref
}

// TurnEndEvent is published when an entity's turn ends
type TurnEndEvent struct {
	events.BaseEvent
	EntityID string
}

// AttackEvent is published when an entity makes an attack
type AttackEvent struct {
	events.BaseEvent
	AttackerID string
	TargetID   string
	WeaponRef  *core.Ref
}

// DamageTakenEvent is published when an entity takes damage
type DamageTakenEvent struct {
	events.BaseEvent
	TargetID   string
	SourceID   string
	Amount     int
	DamageType string
}

