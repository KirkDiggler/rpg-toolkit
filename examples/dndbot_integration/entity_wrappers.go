// Package dndbot shows how to wrap DND bot entities
package dndbot

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// CharacterWrapper wraps the DND bot's Character type to implement core.Entity
// This allows the bot to use toolkit features without changing the Character struct
type CharacterWrapper struct {
	// Embed the actual character - replace with your import path
	// *character.Character

	// For demo purposes, using fields
	CharacterID   string
	CharacterName string
	Level         int
}

// GetID returns the character's unique identifier
func (c *CharacterWrapper) GetID() string {
	return c.CharacterID
}

// GetType returns "character" as the entity type
func (c *CharacterWrapper) GetType() string {
	return "character"
}

// MonsterWrapper wraps the DND bot's Monster type to implement core.Entity
type MonsterWrapper struct {
	// Embed the actual monster - replace with your import path
	// *combat.Monster

	// For demo purposes, using fields
	MonsterID   string
	MonsterName string
	CR          float64 // Challenge Rating
}

// GetID returns the monster's unique identifier
func (m *MonsterWrapper) GetID() string {
	return m.MonsterID
}

// GetType returns "monster" as the entity type
func (m *MonsterWrapper) GetType() string {
	return "monster"
}

// Ensure our wrappers implement core.Entity
var (
	_ core.Entity = (*CharacterWrapper)(nil)
	_ core.Entity = (*MonsterWrapper)(nil)
)

// WrapCharacter creates an entity wrapper for a character
// In real usage: func WrapCharacter(c *character.Character) core.Entity
func WrapCharacter(id, name string, level int) core.Entity {
	return &CharacterWrapper{
		CharacterID:   id,
		CharacterName: name,
		Level:         level,
	}
}

// WrapMonster creates an entity wrapper for a monster
// In real usage: func WrapMonster(m *combat.Monster) core.Entity
func WrapMonster(id, name string, cr float64) core.Entity {
	return &MonsterWrapper{
		MonsterID:   id,
		MonsterName: name,
		CR:          cr,
	}
}
