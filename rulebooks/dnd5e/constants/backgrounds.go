package constants

// Background represents a D&D 5e character background
type Background string

// Background constants
const (
	BackgroundAcolyte      Background = "acolyte"
	BackgroundCharlatan    Background = "charlatan"
	BackgroundCriminal     Background = "criminal"
	BackgroundEntertainer  Background = "entertainer"
	BackgroundFolkHero     Background = "folk-hero"
	BackgroundGuildArtisan Background = "guild-artisan"
	BackgroundHermit       Background = "hermit"
	BackgroundNoble        Background = "noble"
	BackgroundOutlander    Background = "outlander"
	BackgroundSage         Background = "sage"
	BackgroundSailor       Background = "sailor"
	BackgroundSoldier      Background = "soldier"
	BackgroundUrchin       Background = "urchin"
)

// Display returns the human-readable name of the background
func (b Background) Display() string {
	switch b {
	case BackgroundAcolyte:
		return "Acolyte"
	case BackgroundCharlatan:
		return "Charlatan"
	case BackgroundCriminal:
		return "Criminal"
	case BackgroundEntertainer:
		return "Entertainer"
	case BackgroundFolkHero:
		return "Folk Hero"
	case BackgroundGuildArtisan:
		return "Guild Artisan"
	case BackgroundHermit:
		return "Hermit"
	case BackgroundNoble:
		return "Noble"
	case BackgroundOutlander:
		return "Outlander"
	case BackgroundSage:
		return "Sage"
	case BackgroundSailor:
		return "Sailor"
	case BackgroundSoldier:
		return "Soldier"
	case BackgroundUrchin:
		return "Urchin"
	default:
		return string(b)
	}
}
