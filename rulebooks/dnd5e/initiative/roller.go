package initiative

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// Entry represents someone's initiative with their entity
type Entry struct {
	Entity   core.Entity
	Roll     int
	Modifier int
	Total    int
}

// RollForOrder rolls initiative and returns entities in turn order
func RollForOrder(entities map[core.Entity]int, roller dice.Roller) []core.Entity {
	// Use default roller if none provided
	if roller == nil {
		roller = dice.DefaultRoller
	}

	// Roll for each entity
	entries := make([]Entry, 0, len(entities))
	for entity, modifier := range entities {
		roll, _ := roller.Roll(20)
		entries = append(entries, Entry{
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

	// Extract just the entities in order
	order := make([]core.Entity, len(entries))
	for i, entry := range entries {
		order[i] = entry.Entity
	}

	return order
}
