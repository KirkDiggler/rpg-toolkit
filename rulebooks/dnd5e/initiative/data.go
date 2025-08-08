package initiative

import "github.com/KirkDiggler/rpg-toolkit/core"

// TrackerData represents the persistent state of a turn tracker
type TrackerData struct {
	// Entity IDs and types in initiative order
	Order []EntityData `json:"order"`

	// Current turn index
	Current int `json:"current"`

	// Current round number
	Round int `json:"round"`
}

// EntityData represents a participant's data for persistence
type EntityData struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ToData converts the tracker to persistent data
func (t *Tracker) ToData() TrackerData {
	order := make([]EntityData, len(t.order))
	for i, entity := range t.order {
		order[i] = EntityData{
			ID:   entity.GetID(),
			Type: entity.GetType(),
		}
	}

	return TrackerData{
		Order:   order,
		Current: t.current,
		Round:   t.round,
	}
}

// LoadFromData creates a tracker from persistent data
func LoadFromData(data TrackerData) *Tracker {
	// Convert data back to entities
	order := make([]core.Entity, len(data.Order))
	for i, entityData := range data.Order {
		order[i] = NewParticipant(entityData.ID, entityData.Type)
	}

	return &Tracker{
		order:   order,
		current: data.Current,
		round:   data.Round,
	}
}
