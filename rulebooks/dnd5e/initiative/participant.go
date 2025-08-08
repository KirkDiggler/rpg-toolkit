package initiative

import "github.com/KirkDiggler/rpg-toolkit/core"

// Participant wraps any entity to participate in initiative
type Participant struct {
	id         string
	entityType string
}

// NewParticipant creates a participant from ID and type
func NewParticipant(id, entityType string) *Participant {
	return &Participant{
		id:         id,
		entityType: entityType,
	}
}

// GetID returns the entity's ID
func (p *Participant) GetID() string {
	return p.id
}

// GetType returns the entity's type
func (p *Participant) GetType() string {
	return p.entityType
}

// Verify it implements core.Entity
var _ core.Entity = (*Participant)(nil)