// Package armor provides D&D 5e armor constants and definitions
package armor

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// ArmorID represents unique identifier for armor
type ArmorID string

// ArmorCategory represents armor weight classification
type ArmorCategory string

const (
	// CategoryLight represents light armor category
	CategoryLight ArmorCategory = "light"
	// CategoryMedium represents medium armor category
	CategoryMedium ArmorCategory = "medium"
	// CategoryHeavy represents heavy armor category
	CategoryHeavy ArmorCategory = "heavy"
	// CategoryShield represents shield category
	CategoryShield ArmorCategory = "shield"
)

// Light armor
const (
	Padded         ArmorID = "padded"
	Leather        ArmorID = "leather"
	StuddedLeather ArmorID = "studded-leather"
)

// Medium armor
const (
	Hide        ArmorID = "hide"
	ChainShirt  ArmorID = "chain-shirt"
	ScaleMail   ArmorID = "scale-mail"
	Breastplate ArmorID = "breastplate"
	HalfPlate   ArmorID = "half-plate"
)

// Heavy armor
const (
	RingMail  ArmorID = "ring-mail"
	ChainMail ArmorID = "chain-mail"
	Splint    ArmorID = "splint"
	Plate     ArmorID = "plate"
)

// Shield
const (
	Shield ArmorID = "shield"
)

// Armor represents a piece of armor
type Armor struct {
	ID                  ArmorID
	Name                string
	Category            ArmorCategory
	AC                  int  // Base AC or AC bonus (for shields)
	MaxDexBonus         *int // nil means unlimited, 0 means no Dex bonus
	Strength            int  // Minimum strength requirement
	StealthDisadvantage bool
	Weight              int    // in pounds
	Cost                string // e.g., "5 gp"
}

// All armor definitions
var All = map[ArmorID]Armor{
	// Light armor
	Padded: {
		ID:                  Padded,
		Name:                "Padded Armor",
		Category:            CategoryLight,
		AC:                  11,
		MaxDexBonus:         nil, // Unlimited
		StealthDisadvantage: true,
		Weight:              8,
		Cost:                "5 gp",
	},
	Leather: {
		ID:          Leather,
		Name:        "Leather Armor",
		Category:    CategoryLight,
		AC:          11,
		MaxDexBonus: nil, // Unlimited
		Weight:      10,
		Cost:        "10 gp",
	},
	StuddedLeather: {
		ID:          StuddedLeather,
		Name:        "Studded Leather",
		Category:    CategoryLight,
		AC:          12,
		MaxDexBonus: nil, // Unlimited
		Weight:      13,
		Cost:        "45 gp",
	},

	// Medium armor
	Hide: {
		ID:          Hide,
		Name:        "Hide Armor",
		Category:    CategoryMedium,
		AC:          12,
		MaxDexBonus: intPtr(2),
		Weight:      12,
		Cost:        "10 gp",
	},
	ChainShirt: {
		ID:          ChainShirt,
		Name:        "Chain Shirt",
		Category:    CategoryMedium,
		AC:          13,
		MaxDexBonus: intPtr(2),
		Weight:      20,
		Cost:        "50 gp",
	},
	ScaleMail: {
		ID:                  ScaleMail,
		Name:                "Scale Mail",
		Category:            CategoryMedium,
		AC:                  14,
		MaxDexBonus:         intPtr(2),
		StealthDisadvantage: true,
		Weight:              45,
		Cost:                "50 gp",
	},
	Breastplate: {
		ID:          Breastplate,
		Name:        "Breastplate",
		Category:    CategoryMedium,
		AC:          14,
		MaxDexBonus: intPtr(2),
		Weight:      20,
		Cost:        "400 gp",
	},
	HalfPlate: {
		ID:                  HalfPlate,
		Name:                "Half Plate",
		Category:            CategoryMedium,
		AC:                  15,
		MaxDexBonus:         intPtr(2),
		StealthDisadvantage: true,
		Weight:              40,
		Cost:                "750 gp",
	},

	// Heavy armor
	RingMail: {
		ID:                  RingMail,
		Name:                "Ring Mail",
		Category:            CategoryHeavy,
		AC:                  14,
		MaxDexBonus:         intPtr(0),
		StealthDisadvantage: true,
		Weight:              40,
		Cost:                "30 gp",
	},
	ChainMail: {
		ID:                  ChainMail,
		Name:                "Chain Mail",
		Category:            CategoryHeavy,
		AC:                  16,
		MaxDexBonus:         intPtr(0),
		Strength:            13,
		StealthDisadvantage: true,
		Weight:              55,
		Cost:                "75 gp",
	},
	Splint: {
		ID:                  Splint,
		Name:                "Splint Armor",
		Category:            CategoryHeavy,
		AC:                  17,
		MaxDexBonus:         intPtr(0),
		Strength:            15,
		StealthDisadvantage: true,
		Weight:              60,
		Cost:                "200 gp",
	},
	Plate: {
		ID:                  Plate,
		Name:                "Plate Armor",
		Category:            CategoryHeavy,
		AC:                  18,
		MaxDexBonus:         intPtr(0),
		Strength:            15,
		StealthDisadvantage: true,
		Weight:              65,
		Cost:                "1500 gp",
	},

	// Shield
	Shield: {
		ID:       Shield,
		Name:     "Shield",
		Category: CategoryShield,
		AC:       2, // AC bonus
		Weight:   6,
		Cost:     "10 gp",
	},
}

// GetByID returns armor by its ID
func GetByID(id ArmorID) (Armor, error) {
	armor, ok := All[id]
	if !ok {
		validArmor := make([]string, 0, len(All))
		for k := range All {
			validArmor = append(validArmor, string(k))
		}
		return Armor{}, rpgerr.New(rpgerr.CodeInvalidArgument, "invalid armor",
			rpgerr.WithMeta("provided", string(id)),
			rpgerr.WithMeta("valid_options", validArmor))
	}
	return armor, nil
}

// GetByCategory returns all armor in a category
func GetByCategory(cat ArmorCategory) []Armor {
	var result []Armor
	for _, a := range All {
		if a.Category == cat {
			result = append(result, a)
		}
	}
	return result
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
