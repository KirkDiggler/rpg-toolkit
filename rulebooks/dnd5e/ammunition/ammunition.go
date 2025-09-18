// Package ammunition provides a unified interface for D&D 5e ammunition items
package ammunition

// StandardAmmunition contains all standard ammunition types from the PHB
var StandardAmmunition = map[ID]*Ammunition{
	// Arrow bundles
	Arrows20: {
		ID:       Arrows20,
		Name:     "Arrows (20)",
		Type:     TypeArrows,
		Quantity: 20,
		Cost:     "1 gp",
		Weight:   1, // 20 arrows = 1 lb
	},
	Arrows50: {
		ID:       Arrows50,
		Name:     "Arrows (50)",
		Type:     TypeArrows,
		Quantity: 50,
		Cost:     "2 gp 5 sp",
		Weight:   2.5,
	},

	// Crossbow bolt bundles
	Bolts20: {
		ID:       Bolts20,
		Name:     "Crossbow bolts (20)",
		Type:     TypeBolts,
		Quantity: 20,
		Cost:     "1 gp",
		Weight:   1.5, // Bolts are slightly heavier than arrows
	},
	Bolts50: {
		ID:       Bolts50,
		Name:     "Crossbow bolts (50)",
		Type:     TypeBolts,
		Quantity: 50,
		Cost:     "2 gp 5 sp",
		Weight:   3.75,
	},

	// Blowgun needles
	BlowgunNeedles50: {
		ID:       BlowgunNeedles50,
		Name:     "Blowgun needles (50)",
		Type:     TypeNeedles,
		Quantity: 50,
		Cost:     "1 gp",
		Weight:   1,
	},

	// Sling bullets
	SlingBullets20: {
		ID:       SlingBullets20,
		Name:     "Sling bullets (20)",
		Type:     TypeBullets,
		Quantity: 20,
		Cost:     "4 cp",
		Weight:   1.5,
	},
}

// GetByID retrieves ammunition by its ID
func GetByID(id ID) (*Ammunition, bool) {
	ammo, ok := StandardAmmunition[id]
	return ammo, ok
}
