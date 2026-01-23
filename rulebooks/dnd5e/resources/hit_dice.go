// Package resources provides D&D 5e resource key constants and helpers.
package resources

import (
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// HitDiceResourceConfig contains configuration for creating a hit dice resource
type HitDiceResourceConfig struct {
	// CharacterID is the ID of the character this resource belongs to
	CharacterID string

	// Level is the character's total level (determines maximum hit dice)
	Level int
}

// NewHitDiceResource creates a RecoverableResource configured for hit dice.
// Hit dice have special recovery: on long rest, regain half of maximum (minimum 1).
// This differs from other resources which restore to full.
func NewHitDiceResource(config HitDiceResourceConfig) *combat.RecoverableResource {
	return combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          string(HitDice),
		Maximum:     config.Level,
		CharacterID: config.CharacterID,
		ResetType:   coreResources.ResetLongRest,
		RecoveryFunc: func(r *combat.RecoverableResource) {
			// D&D 5e PHB p. 186: "...regain spent Hit Dice, up to a number
			// of dice equal to half of your total number of them (minimum of one die)"
			amount := r.Maximum() / 2
			if amount < 1 {
				amount = 1
			}
			r.Restore(amount)
		},
	})
}
