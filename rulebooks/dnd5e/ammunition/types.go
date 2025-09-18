package ammunition

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ID is a unique identifier for ammunition
type ID = shared.EquipmentID

// Common ammunition IDs
const (
	// Arrows20 is 20 arrows
	Arrows20 ID = "arrows-20"
	// Arrows50 is 50 arrows
	Arrows50 ID = "arrows-50"
	// Bolts20 is 20 bolts
	Bolts20 ID = "bolts-20"
	// Bolts50 is 50 bolts
	Bolts50 ID = "bolts-50"
	// BlowgunNeedles50 is 50 blowgun needles
	BlowgunNeedles50 ID = "blowgun-needles-50"
	// SlingBullets20 is 20 sling bullets
	SlingBullets20 ID = "sling-bullets-20"
)

// Type represents the type of ammunition (what weapons can use it)
type Type string

const (
	// TypeArrows is for bows
	TypeArrows Type = "arrows"
	// TypeBolts is for crossbows
	TypeBolts Type = "bolts"
	// TypeBullets is for slings
	TypeBullets Type = "bullets"
	// TypeNeedles is for blowguns
	TypeNeedles Type = "needles"
)

// Ammunition represents a type of ammunition in D&D 5e
type Ammunition struct {
	ID       ID      // Unique identifier
	Name     string  // Display name
	Type     Type    // What weapons can use this
	Quantity int     // How many in this bundle
	Cost     string  // Cost for the bundle
	Weight   float64 // Weight of the entire bundle in pounds
}

// EquipmentID returns the unique identifier for this equipment
func (a *Ammunition) EquipmentID() shared.EquipmentID {
	return a.ID
}

// EquipmentType returns the category of equipment
func (a *Ammunition) EquipmentType() shared.EquipmentType {
	return shared.EquipmentTypeAmmunition
}

// EquipmentName returns the display name
func (a *Ammunition) EquipmentName() string {
	return a.Name
}

// EquipmentWeight returns the weight in pounds
func (a *Ammunition) EquipmentWeight() float32 {
	return float32(a.Weight)
}

// EquipmentValue returns the value in copper pieces
func (a *Ammunition) EquipmentValue() int {
	// TODO: Parse cost string and convert to copper
	// For now, return a placeholder
	return 0
}

// EquipmentDescription returns a description of the item
func (a *Ammunition) EquipmentDescription() string {
	return "" // Basic ammunition has no special description
}

// GetQuantity returns how many individual pieces are in this bundle
func (a *Ammunition) GetQuantity() int {
	return a.Quantity
}

// GetAmmunitionType returns what type of weapon can use this ammunition
func (a *Ammunition) GetAmmunitionType() Type {
	return a.Type
}
