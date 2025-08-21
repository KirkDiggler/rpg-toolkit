// Package backgrounds provides D&D 5e background constants and definitions
package backgrounds

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Background represents a D&D 5e character background
type Background string

// Core backgrounds from Player's Handbook
const (
	Acolyte      Background = "acolyte"
	Criminal     Background = "criminal"
	FolkHero     Background = "folk-hero"
	Noble        Background = "noble"
	Sage         Background = "sage"
	Soldier      Background = "soldier"
	Charlatan    Background = "charlatan"
	Entertainer  Background = "entertainer"
	GuildArtisan Background = "guild-artisan"
	Hermit       Background = "hermit"
	Outlander    Background = "outlander"
	Sailor       Background = "sailor"
	Urchin       Background = "urchin"
)

// Criminal variant
const (
	Spy Background = "spy" // Variant of Criminal
)

// Sailor variant
const (
	Pirate Background = "pirate" // Variant of Sailor
)

// Noble variant
const (
	Knight Background = "knight" // Variant of Noble
)

// Guild Artisan variant
const (
	GuildMerchant Background = "guild-merchant" // Variant of Guild Artisan
)

// All provides map lookup for backgrounds
var All = map[string]Background{
	"acolyte":       Acolyte,
	"criminal":      Criminal,
	"folk-hero":     FolkHero,
	"noble":         Noble,
	"sage":          Sage,
	"soldier":       Soldier,
	"charlatan":     Charlatan,
	"entertainer":   Entertainer,
	"guild-artisan": GuildArtisan,
	"hermit":        Hermit,
	"outlander":     Outlander,
	"sailor":        Sailor,
	"urchin":        Urchin,
	// Variants
	"spy":            Spy,
	"pirate":         Pirate,
	"knight":         Knight,
	"guild-merchant": GuildMerchant,
}

// GetByID returns a background by its ID
func GetByID(id string) (Background, error) {
	bg, ok := All[id]
	if !ok {
		validBackgrounds := make([]string, 0, len(All))
		for k := range All {
			validBackgrounds = append(validBackgrounds, k)
		}
		return "", rpgerr.New(rpgerr.CodeInvalidArgument, "invalid background",
			rpgerr.WithMeta("provided", id),
			rpgerr.WithMeta("valid_options", validBackgrounds))
	}
	return bg, nil
}

// IsVariant returns true if this is a variant background
func (b Background) IsVariant() bool {
	switch b {
	case Spy, Pirate, Knight, GuildMerchant:
		return true
	default:
		return false
	}
}

// BaseBackground returns the base background for variants
func (b Background) BaseBackground() Background {
	switch b {
	case Spy:
		return Criminal
	case Pirate:
		return Sailor
	case Knight:
		return Noble
	case GuildMerchant:
		return GuildArtisan
	default:
		return b
	}
}
