package constants

import "strings"

// Language represents a D&D 5e language
type Language string

// Standard language constants
const (
	LanguageCommon      Language = "common"
	LanguageDwarvish    Language = "dwarvish"
	LanguageElvish      Language = "elvish"
	LanguageGiant       Language = "giant"
	LanguageGnomish     Language = "gnomish"
	LanguageGoblin      Language = "goblin"
	LanguageHalfling    Language = "halfling"
	LanguageOrc         Language = "orc"
	LanguageAbyssal     Language = "abyssal"
	LanguageCelestial   Language = "celestial"
	LanguageDraconic    Language = "draconic"
	LanguageDeepSpeech  Language = "deep speech"
	LanguageInfernal    Language = "infernal"
	LanguagePrimordial  Language = "primordial"
	LanguageSylvan      Language = "sylvan"
	LanguageUndercommon Language = "undercommon"
)

// Display returns the human-readable name of the language
func (l Language) Display() string {
	switch l {
	case LanguageCommon:
		return "Common"
	case LanguageDwarvish:
		return "Dwarvish"
	case LanguageElvish:
		return "Elvish"
	case LanguageGiant:
		return "Giant"
	case LanguageGnomish:
		return "Gnomish"
	case LanguageGoblin:
		return "Goblin"
	case LanguageHalfling:
		return "Halfling"
	case LanguageOrc:
		return "Orc"
	case LanguageAbyssal:
		return "Abyssal"
	case LanguageCelestial:
		return "Celestial"
	case LanguageDraconic:
		return "Draconic"
	case LanguageDeepSpeech:
		return "Deep Speech"
	case LanguageInfernal:
		return "Infernal"
	case LanguagePrimordial:
		return "Primordial"
	case LanguageSylvan:
		return "Sylvan"
	case LanguageUndercommon:
		return "Undercommon"
	default:
		// Capitalize first letter as fallback
		if len(l) > 0 {
			return strings.ToUpper(string(l[0])) + string(l[1:])
		}
		return string(l)
	}
}

// IsStandard returns true if this is a standard language (vs exotic)
func (l Language) IsStandard() bool {
	switch l {
	case LanguageCommon, LanguageDwarvish, LanguageElvish, LanguageGiant,
		LanguageGnomish, LanguageGoblin, LanguageHalfling, LanguageOrc:
		return true
	default:
		return false
	}
}

// StandardLanguages returns all standard languages
func StandardLanguages() []Language {
	return []Language{
		LanguageCommon,
		LanguageDwarvish,
		LanguageElvish,
		LanguageGiant,
		LanguageGnomish,
		LanguageGoblin,
		LanguageHalfling,
		LanguageOrc,
	}
}

// ExoticLanguages returns all exotic languages
func ExoticLanguages() []Language {
	return []Language{
		LanguageAbyssal,
		LanguageCelestial,
		LanguageDraconic,
		LanguageDeepSpeech,
		LanguageInfernal,
		LanguagePrimordial,
		LanguageSylvan,
		LanguageUndercommon,
	}
}
