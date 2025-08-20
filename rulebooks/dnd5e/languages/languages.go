// Package languages provides D&D 5e language constants and utilities
package languages

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Language represents a D&D 5e language
type Language string

// Standard languages that most characters can learn
const (
	Common   Language = "common"
	Dwarvish Language = "dwarvish"
	Elvish   Language = "elvish"
	Giant    Language = "giant"
	Gnomish  Language = "gnomish"
	Goblin   Language = "goblin"
	Halfling Language = "halfling"
	Orc      Language = "orc"
)

// Exotic languages that typically require special circumstances to learn
const (
	Abyssal     Language = "abyssal"
	Celestial   Language = "celestial"
	Draconic    Language = "draconic"
	DeepSpeech  Language = "deep-speech"
	Infernal    Language = "infernal"
	Primordial  Language = "primordial"
	Sylvan      Language = "sylvan"
	Undercommon Language = "undercommon"
)

// All contains all languages mapped by ID for O(1) lookup
var All = map[string]Language{
	"common":      Common,
	"dwarvish":    Dwarvish,
	"elvish":      Elvish,
	"giant":       Giant,
	"gnomish":     Gnomish,
	"goblin":      Goblin,
	"halfling":    Halfling,
	"orc":         Orc,
	"abyssal":     Abyssal,
	"celestial":   Celestial,
	"draconic":    Draconic,
	"deep-speech": DeepSpeech,
	"infernal":    Infernal,
	"primordial":  Primordial,
	"sylvan":      Sylvan,
	"undercommon": Undercommon,
}

// StandardLanguages returns all standard languages
func StandardLanguages() []Language {
	return []Language{
		Common,
		Dwarvish,
		Elvish,
		Giant,
		Gnomish,
		Goblin,
		Halfling,
		Orc,
	}
}

// ExoticLanguages returns all exotic languages
func ExoticLanguages() []Language {
	return []Language{
		Abyssal,
		Celestial,
		Draconic,
		DeepSpeech,
		Infernal,
		Primordial,
		Sylvan,
		Undercommon,
	}
}

// GetByID returns a language by its ID
func GetByID(id string) (Language, error) {
	lang, ok := All[id]
	if !ok {
		validLanguages := make([]string, 0, len(All))
		for k := range All {
			validLanguages = append(validLanguages, k)
		}
		return "", rpgerr.New(rpgerr.CodeInvalidArgument, "invalid language",
			rpgerr.WithMeta("provided", id),
			rpgerr.WithMeta("valid_options", validLanguages))
	}
	return lang, nil
}

// IsStandard returns true if the language is a standard language
func IsStandard(lang Language) bool {
	switch lang {
	case Common, Dwarvish, Elvish, Giant, Gnomish, Goblin, Halfling, Orc:
		return true
	default:
		return false
	}
}

// IsExotic returns true if the language is an exotic language
func IsExotic(lang Language) bool {
	switch lang {
	case Abyssal, Celestial, Draconic, DeepSpeech, Infernal, Primordial, Sylvan, Undercommon:
		return true
	default:
		return false
	}
}

// Display returns the human-readable name of the language
func (l Language) Display() string {
	switch l {
	case Common:
		return "Common"
	case Dwarvish:
		return "Dwarvish"
	case Elvish:
		return "Elvish"
	case Giant:
		return "Giant"
	case Gnomish:
		return "Gnomish"
	case Goblin:
		return "Goblin"
	case Halfling:
		return "Halfling"
	case Orc:
		return "Orc"
	case Abyssal:
		return "Abyssal"
	case Celestial:
		return "Celestial"
	case Draconic:
		return "Draconic"
	case DeepSpeech:
		return "Deep Speech"
	case Infernal:
		return "Infernal"
	case Primordial:
		return "Primordial"
	case Sylvan:
		return "Sylvan"
	case Undercommon:
		return "Undercommon"
	default:
		return string(l)
	}
}
