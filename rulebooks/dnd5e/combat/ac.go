// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ACSourceType categorizes where armor class bonuses come from
type ACSourceType string

// AC source type constants
const (
	ACSourceBase      ACSourceType = "base"      // Base AC (10 for unarmored)
	ACSourceArmor     ACSourceType = "armor"     // Armor worn
	ACSourceShield    ACSourceType = "shield"    // Shield equipped
	ACSourceAbility   ACSourceType = "ability"   // Ability modifier (DEX, etc.)
	ACSourceFeature   ACSourceType = "feature"   // Class features (Unarmored Defense, etc.)
	ACSourceSpell     ACSourceType = "spell"     // Spell effects (Mage Armor, Shield, etc.)
	ACSourceItem      ACSourceType = "item"      // Magic items (Ring of Protection, etc.)
	ACSourceCondition ACSourceType = "condition" // Conditions (Cover, etc.)
)

// ACComponent represents an AC bonus from one source
type ACComponent struct {
	Type   ACSourceType // Category of the AC source
	Source *core.Ref    // Specific source reference (e.g., dnd5e:armor:chain_mail)
	Value  int          // AC bonus (can be negative for debuffs)
}

// ACBreakdown provides detailed component breakdown of armor class calculation
type ACBreakdown struct {
	Total      int           // Final AC value
	Components []ACComponent // All AC sources
}

// AddComponent adds a component to the breakdown and updates the total
func (b *ACBreakdown) AddComponent(component ACComponent) {
	b.Components = append(b.Components, component)
	b.Total += component.Value
}

// ACChainEvent represents armor class calculation flowing through the modifier chain
type ACChainEvent struct {
	CharacterID string       // Which character's AC is being calculated
	Breakdown   *ACBreakdown // Detailed AC breakdown
	HasArmor    bool         // Whether character is wearing armor
	HasShield   bool         // Whether character is using a shield
}

// ACChain provides typed chained topic for armor class modifiers
var ACChain = events.DefineChainedTopic[*ACChainEvent]("dnd5e.combat.ac.chain")
