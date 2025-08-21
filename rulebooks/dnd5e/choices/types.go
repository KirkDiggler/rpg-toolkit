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

const (
	// CategorySkill represents skill proficiency choices
	CategorySkill Category = "skill"
	// CategoryLanguage represents language choices
	CategoryLanguage Category = "language"
	// CategoryTool represents tool proficiency choices
	CategoryTool Category = "tool"
	// CategoryEquipment represents equipment choices
	CategoryEquipment Category = "equipment"
	// CategoryAbility represents ability score choices
	CategoryAbility Category = "ability"
	// CategorySpell represents spell choices
	CategorySpell Category = "spell"
	// CategoryCantrip represents cantrip choices
	CategoryCantrip Category = "cantrip"
	// CategoryFeat represents feat choices
	CategoryFeat Category = "feat"
)

// Source of the choice
type Source string

const (
	// SourceClass indicates choice comes from character class
	SourceClass Source = "class"
	// SourceRace indicates choice comes from character race
	SourceRace Source = "race"
	// SourceBackground indicates choice comes from character background
	SourceBackground Source = "background"
	// SourceSubclass indicates choice comes from class specialization
	SourceSubclass Source = "subclass"
	// SourceSubrace indicates choice comes from race variant
	SourceSubrace Source = "subrace"
	// SourceFeat indicates choice comes from a feat
	SourceFeat Source = "feat"
)

// Option represents a single selectable option
type Option interface {
	GetID() string
	GetType() OptionType
	Validate() error
}

// OptionType identifies the type of option
type OptionType string

const (
	// OptionTypeSingle represents a single item choice
	OptionTypeSingle OptionType = "single"
	// OptionTypeBundle represents multiple items bundled together
	OptionTypeBundle OptionType = "bundle"
	// OptionTypeCategory represents choosing from a category of items
	OptionTypeCategory OptionType = "category"
)

// ItemType identifies what kind of item this is
type ItemType string

const (
	// ItemTypeSkill represents a skill proficiency
	ItemTypeSkill ItemType = "skill"
	// ItemTypeLanguage represents a language proficiency
	ItemTypeLanguage ItemType = "language"
	// ItemTypeTool represents a tool proficiency
	ItemTypeTool ItemType = "tool"
	// ItemTypeWeapon represents a weapon item
	ItemTypeWeapon ItemType = "weapon"
	// ItemTypeArmor represents an armor item
	ItemTypeArmor ItemType = "armor"
	// ItemTypeGear represents adventuring gear
	ItemTypeGear ItemType = "gear"
	// ItemTypeSpell represents a spell
	ItemTypeSpell ItemType = "spell"
	// ItemTypeFeat represents a feat
	ItemTypeFeat ItemType = "feat"
)
