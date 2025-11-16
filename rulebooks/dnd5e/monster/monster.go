// Package monster provides monster/enemy entity types for D&D 5e combat
package monster

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Monster represents a simple hostile creature for combat encounters
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

// GetID implements core.Entity
func (m *Monster) GetID() string {
	return m.id
}

// GetType implements core.Entity
func (m *Monster) GetType() core.EntityType {
	return dnd5e.EntityTypeMonster
}

// Name returns the monster's name
func (m *Monster) Name() string {
	return m.name
}

// HP returns current hit points
func (m *Monster) HP() int {
	return m.hp
}

// MaxHP returns maximum hit points
func (m *Monster) MaxHP() int {
	return m.maxHP
}

// AC returns armor class
func (m *Monster) AC() int {
	return m.ac
}

// AbilityScores returns the monster's ability scores
func (m *Monster) AbilityScores() shared.AbilityScores {
	return m.abilityScores
}

// TakeDamage reduces HP (returns actual damage taken)
func (m *Monster) TakeDamage(amount int) int {
	if amount < 0 {
		amount = 0
	}
	previousHP := m.hp
	m.hp -= amount
	if m.hp < 0 {
		m.hp = 0
	}
	return previousHP - m.hp
}

// IsAlive returns true if HP > 0
func (m *Monster) IsAlive() bool {
	return m.hp > 0
}

// NewGoblin creates a standard goblin (CR 1/4, D&D 5e SRD stats)
func NewGoblin(id string) *Monster {
	return New(Config{
		ID:   id,
		Name: "Goblin",
		HP:   7,  // 2d6 average
		AC:   15, // Leather armor + DEX
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,  // -1
			abilities.DEX: 14, // +2
			abilities.CON: 10, // +0
			abilities.INT: 10, // +0
			abilities.WIS: 8,  // -1
			abilities.CHA: 8,  // -1
		},
	})
}
