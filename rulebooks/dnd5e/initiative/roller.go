package initiative

import (
	"math/rand"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Entry represents someone's initiative with their entity
type Entry struct {
	Entity   core.Entity
	Roll     int
	Modifier int
	Total    int
}

// RollForOrder rolls initiative and returns entities in turn order
func RollForOrder(entities map[core.Entity]int) []core.Entity {
	// Roll for each entity
	entries := make([]Entry, 0, len(entities))
	for entity, modifier := range entities {
		roll := rand.Intn(20) + 1
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