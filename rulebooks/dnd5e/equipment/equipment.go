// Package equipment provides a unified interface for D&D 5e equipment items
package equipment

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"

// Equipment represents any item that can be owned, carried, or equipped
type Equipment interface {
	// GetID returns the unique identifier for this equipment
	GetID() string

	// GetType returns the category of equipment as a string
	GetType() shared.EquipmentType

	// GetName returns the display name
	GetName() string

	// GetWeight returns the weight in pounds
	GetWeight() float32

	// GetValue returns the value in copper pieces
	GetValue() int

	// GetDescription returns a description of the item
	GetDescription() string
}
