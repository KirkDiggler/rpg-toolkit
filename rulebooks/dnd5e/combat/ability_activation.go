// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// These two types lived in turn_manager.go until rpg-project#319 Phase 6
// deleted the TurnManager around them. They describe the character's
// ability-activation surface — [character.Character.ActivateCombatAbility] and
// GetAbilityInfos — which has had no caller since TurnManager was its only one.
//
// KEPT AS AN OPEN DECISION, not as a leftover, and the distinction is the one
// resolution.NewMovement is held to: a seam with no caller is dead code when
// nothing intends to call it, and a driver waiting on its slice when something
// does. Feature activation is rpg-project#300, where the finding is that a
// player cannot activate ANYTHING — every character already carries Dodge,
// Dash, Disengage, Help, Hide and Attack, unreachable since the encounter
// rip-out took their driver. This is the surface those six are reached
// through, so deleting it now would delete the thing that slice has to revive.
//
// What is genuinely unresolved is the SHAPE: ActivateAbilityInput carries
// Bus, Economy, Speed and ExtraAttacks because a TurnManager assembled them,
// and #300 may well ask for something narrower. Whoever takes that slice
// should treat this as a starting point to reshape rather than a contract to
// preserve. If #300 rules otherwise, both types and both Character methods go
// together in one pass — the same pairing this phase just applied to
// TurnManager and Character.EndCombat.

// ActivateAbilityInput provides input for activating a combat ability.
// Defined in the combat package to avoid an import cycle with combatabilities.
type ActivateAbilityInput struct {
	// AbilityRef identifies which combat ability to activate.
	AbilityRef *core.Ref

	// Bus is the event bus for publishing events and granting conditions.
	Bus events.EventBus

	// Economy is the action economy for consuming and setting capacity.
	Economy *ActionEconomy

	// Speed is the character's base movement speed in feet.
	Speed int

	// ExtraAttacks is the number of additional attacks from features like Extra Attack.
	ExtraAttacks int
}

// AbilityInfo provides metadata about an available combat ability.
type AbilityInfo struct {
	// Ref is the reference identifying this ability.
	Ref *core.Ref

	// Name is the display name of the ability.
	Name string

	// ActionType is the action economy cost to use this ability.
	ActionType coreCombat.ActionType
}
