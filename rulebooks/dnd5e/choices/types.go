// Package choices provides the D&D 5e character creation choice system
package choices

// Choice represents any character creation choice
type Choice struct {
	ID          ChoiceID
	Category    Category
	Description string
	Choose      int      // How many to choose
	Options     []Option // Available options
	Source      Source   // Where this choice comes from
}

// Category of choice
type Category string

// Category constants define the different types of choices in character creation
const (
	CategorySkill     Category = "skill"
	CategoryLanguage  Category = "language"
	CategoryTool      Category = "tool"
	CategoryEquipment Category = "equipment"
	CategoryAbility   Category = "ability"
	CategorySpell     Category = "spell"
	CategoryCantrip   Category = "cantrip"
	CategoryFeat      Category = "feat"
)

// Source of the choice
type Source string

// Source constants identify where a choice originates from
const (
	SourceClass      Source = "class"
	SourceRace       Source = "race"
	SourceBackground Source = "background"
	SourceSubclass   Source = "subclass"
	SourceSubrace    Source = "subrace"
	SourceFeat       Source = "feat"
)

// Option represents a single selectable option
type Option interface {
	GetID() string
	GetType() OptionType
	Validate() error
}

// OptionType identifies the type of option
type OptionType string

// OptionType constants specify the structure of selectable options
const (
	OptionTypeSingle   OptionType = "single"   // Single item
	OptionTypeBundle   OptionType = "bundle"   // Multiple items together
	OptionTypeCategory OptionType = "category" // Choose from category
)

// ItemType identifies what kind of item this is
type ItemType string

// ItemType constants classify different types of selectable items
const (
	ItemTypeSkill    ItemType = "skill"
	ItemTypeLanguage ItemType = "language"
	ItemTypeTool     ItemType = "tool"
	ItemTypeWeapon   ItemType = "weapon"
	ItemTypeArmor    ItemType = "armor"
	ItemTypeGear     ItemType = "gear"
	ItemTypeSpell    ItemType = "spell"
	ItemTypeFeat     ItemType = "feat"
)
