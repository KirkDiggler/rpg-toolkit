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

// All provides map lookup for backgrounds.
//
// Deprecated: Use BackgroundData directly - it now contains ID field and Name()/Description() methods.
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

// Name returns the display name of the background
func (b Background) Name() string {
	switch b {
	case Acolyte:
		return "Acolyte"
	case Criminal:
		return "Criminal"
	case FolkHero:
		return "Folk Hero"
	case Noble:
		return "Noble"
	case Sage:
		return "Sage"
	case Soldier:
		return "Soldier"
	case Charlatan:
		return "Charlatan"
	case Entertainer:
		return "Entertainer"
	case GuildArtisan:
		return "Guild Artisan"
	case Hermit:
		return "Hermit"
	case Outlander:
		return "Outlander"
	case Sailor:
		return "Sailor"
	case Urchin:
		return "Urchin"
	// Variants
	case Spy:
		return "Spy"
	case Pirate:
		return "Pirate"
	case Knight:
		return "Knight"
	case GuildMerchant:
		return "Guild Merchant"
	default:
		return string(b)
	}
}

// Description returns a brief description of the background
func (b Background) Description() string {
	switch b {
	case Acolyte:
		return "You have spent your life in the service of a temple"
	case Criminal:
		return "You are an experienced criminal with a history of breaking the law"
	case FolkHero:
		return "You come from a humble social rank, but are destined for much more"
	case Noble:
		return "You understand wealth, power, and privilege"
	case Sage:
		return "You spent years learning the lore of the multiverse"
	case Soldier:
		return "War has been your life for as long as you care to remember"
	case Charlatan:
		return "You have always had a way with people"
	case Entertainer:
		return "You thrive in front of an audience"
	case GuildArtisan:
		return "You are a member of an artisan's guild"
	case Hermit:
		return "You lived in seclusion for a formative part of your life"
	case Outlander:
		return "You grew up in the wilds, far from civilization"
	case Sailor:
		return "You sailed on a seagoing vessel for years"
	case Urchin:
		return "You grew up on the streets alone, orphaned, and poor"
	// Variants
	case Spy:
		return "You secretly gather information for a living"
	case Pirate:
		return "You spent your youth under the sway of a dread pirate"
	case Knight:
		return "You are a noble warrior sworn to a code of honor"
	case GuildMerchant:
		return "You are a member of a merchant's guild"
	default:
		return ""
	}
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
