package initiative

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Tracker tracks turn order for any encounter (combat, social, exploration, etc.)
// It doesn't know or care what kind of encounter it is.
type Tracker struct {
	order   []core.Entity // Entities in initiative order
	current int           // Index of whose turn it is
	round   int           // What round we're on
}

// New creates a tracker with the given turn order
func New(initiativeOrder []core.Entity) *Tracker {
	return &Tracker{
		order:   initiativeOrder,
		current: 0,
		round:   1,
	}
}

// Current returns whose turn it is.
// Returns nil if the order is empty or current index is invalid.
func (t *Tracker) Current() core.Entity {
	if len(t.order) == 0 {
		return nil
	}
	if t.current >= len(t.order) {
		// Defensive: should not happen, but return nil if out of bounds
		return nil
	}
	return t.order[t.current]
}

// Next advances to the next turn
func (t *Tracker) Next() core.Entity {
	t.current++

	// If we've gone through everyone, start a new round
	if t.current >= len(t.order) {
		t.current = 0
		t.round++
	}

	return t.Current()
}

// Round returns the current round number
func (t *Tracker) Round() int {
	return t.round
}

// Remove takes someone out of the turn order
func (t *Tracker) Remove(entityID string) error {
	newOrder := make([]core.Entity, 0)
	removed := false

	for i, entity := range t.order {
		if entity.GetID() == entityID {
			removed = true
			// If we're removing someone who already went, adjust current index
			if i < t.current {
				t.current--
			}
		} else {
			newOrder = append(newOrder, entity)
		}
	}

	if !removed {
		return fmt.Errorf("entity %s not found", entityID)
	}

	t.order = newOrder

	// Make sure current is still valid
	// If we removed someone at or after the current position and we're now
	// past the end of the order, wrap to the beginning of next round
	if t.current >= len(t.order) && len(t.order) > 0 {
		t.current = 0
		t.round++
	}

	return nil
}
