package initiative

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// Roll represents a single initiative roll
type Roll struct {
	Entity   core.Entity
	Roll     int // d20 result
	Modifier int // DEX modifier
	Total    int // Roll + Modifier
}

// RollForOrder rolls initiative and returns InitiativeRolls in turn order
func RollForOrder(entities map[core.Entity]int, roller dice.Roller) []Roll {
	// Use default roller if none provided
	if roller == nil {
		roller = dice.DefaultRoller
	}

	// TODO(#285): Make iteration deterministic to ensure consistent ordering
	// when multiple entities have the same total (roll + modifier).
	// Currently, map iteration is non-deterministic in Go.
	// Roll for each entity
	entries := make([]Roll, 0, len(entities))
	for entity, modifier := range entities {
		roll, _ := roller.Roll(20)
		entries = append(entries, Roll{
			Entity:   entity,
			Roll:     roll,
			Modifier: modifier,
			Total:    roll + modifier,
		})
	}

	// Sort by total (highest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Total > entries[j].Total
	})

	return entries
}
