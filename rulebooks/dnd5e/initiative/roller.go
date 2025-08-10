package initiative

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// InitiativeRoll represents a single initiative roll
type InitiativeRoll struct {
	Entity   core.Entity
	Roll     int // d20 result
	Modifier int // DEX modifier
	Total    int // Roll + Modifier
}

// RollForOrder rolls initiative and returns InitiativeRolls in turn order
func RollForOrder(entities map[core.Entity]int, roller dice.Roller) []InitiativeRoll {
	// Use default roller if none provided
	if roller == nil {
		roller = dice.DefaultRoller
	}

	// Roll for each entity
	entries := make([]InitiativeRoll, 0, len(entities))
	for entity, modifier := range entities {
		roll, _ := roller.Roll(20)
		entries = append(entries, InitiativeRoll{
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
