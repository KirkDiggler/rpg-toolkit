// Package monster provides monster/enemy entity types for D&D 5e combat
package monster

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Monster represents a simple hostile creature for combat encounters
// Purpose: Provides a target entity for attacks without complex behavior
type Monster struct {
	id            string
	name          string
	hp            int
	maxHP         int
	ac            int
	abilityScores shared.AbilityScores
}

// Config provides initialization values for creating a monster
type Config struct {
	ID            string
	Name          string
	HP            int
	AC            int
	AbilityScores shared.AbilityScores
}

// New creates a new monster with the specified configuration
func New(config Config) *Monster {
	return &Monster{
		id:            config.ID,
		name:          config.Name,
		hp:            config.HP,
		maxHP:         config.HP,
		ac:            config.AC,
		abilityScores: config.AbilityScores,
	}
}

// GetID returns the unique identifier for this monster
func (m *Monster) GetID() string {
	return m.id
}

// GetType returns the entity type (monster)
func (m *Monster) GetType() core.EntityType {
	return dnd5e.EntityTypeMonster
}

// Name returns the monster's name
func (m *Monster) Name() string {
	return m.name
}

// HP returns the monster's current hit points
func (m *Monster) HP() int {
	return m.hp
}

// MaxHP returns the monster's maximum hit points
func (m *Monster) MaxHP() int {
	return m.maxHP
}

// AC returns the monster's armor class
func (m *Monster) AC() int {
	return m.ac
}

// AbilityScores returns the monster's ability scores
func (m *Monster) AbilityScores() shared.AbilityScores {
	return m.abilityScores
}

// TakeDamage reduces the monster's HP by the specified amount
// Returns the actual damage taken (won't go below 0 HP)
func (m *Monster) TakeDamage(amount int) int {
	if amount < 0 {
		amount = 0
	}

	previousHP := m.hp
	m.hp -= amount

	if m.hp < 0 {
		m.hp = 0
	}

	// Return actual damage taken
	return previousHP - m.hp
}

// Heal restores the monster's HP by the specified amount
// Returns the actual HP restored (won't exceed max HP)
func (m *Monster) Heal(amount int) int {
	if amount < 0 {
		amount = 0
	}

	previousHP := m.hp
	m.hp += amount

	if m.hp > m.maxHP {
		m.hp = m.maxHP
	}

	// Return actual HP restored
	return m.hp - previousHP
}

// IsAlive returns true if the monster has HP remaining
func (m *Monster) IsAlive() bool {
	return m.hp > 0
}

// NewGoblin creates a standard goblin enemy
// Stats from D&D 5e SRD: Small humanoid, CR 1/4
func NewGoblin(id string) *Monster {
	return New(Config{
		ID:   id,
		Name: "Goblin",
		HP:   7,  // 2d6 average
		AC:   15, // Leather armor + DEX modifier
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,  // -1 modifier
			abilities.DEX: 14, // +2 modifier
			abilities.CON: 10, // +0 modifier
			abilities.INT: 10, // +0 modifier
			abilities.WIS: 8,  // -1 modifier
			abilities.CHA: 8,  // -1 modifier
		},
	})
}
