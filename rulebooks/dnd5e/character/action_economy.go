package character

import "context"

// InCombat returns true if the character is currently in combat.
// Combat is indicated by the action economy being initialized (non-nil).
func (c *Character) InCombat() bool {
	return c.actionEconomy != nil
}

// ExitCombat clears the action economy entirely, removing combat state.
// Call this when the encounter ends, not between turns.
func (c *Character) ExitCombat(_ context.Context, _ *ExitCombatInput) (*ExitCombatOutput, error) {
	c.actionEconomy = nil
	return &ExitCombatOutput{}, nil
}
