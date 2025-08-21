package choices

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// SingleOption represents a single selectable item
type SingleOption struct {
	ItemType ItemType
	ItemID   string
	Display  string // Optional display name
}

// GetID returns the option's ID
func (o SingleOption) GetID() string {
	return o.ItemID
}

// GetType returns the option type
func (o SingleOption) GetType() OptionType {
	return OptionTypeSingle
}

// Validate checks if the option is valid
func (o SingleOption) Validate() error {
	if o.ItemID == "" {
		return fmt.Errorf("item ID is required")
	}
	if o.ItemType == "" {
		return fmt.Errorf("item type is required")
	}
	return nil
}

// BundleOption represents multiple items as one choice
type BundleOption struct {
	ID    string
	Items []CountedItem
}

// CountedItem represents an item with quantity
type CountedItem struct {
	ItemType ItemType
	ItemID   string
	Quantity int
}

// GetID returns the option's ID
func (o BundleOption) GetID() string {
	return o.ID
}

// GetType returns the option type
func (o BundleOption) GetType() OptionType {
	return OptionTypeBundle
}

// Validate checks if the option is valid
func (o BundleOption) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("bundle ID is required")
	}
	if len(o.Items) == 0 {
		return fmt.Errorf("bundle must contain at least one item")
	}
	for i, item := range o.Items {
		if item.ItemID == "" {
			return fmt.Errorf("item %d: ID is required", i)
		}
		if item.Quantity < 1 {
			return fmt.Errorf("item %d: quantity must be at least 1", i)
		}
	}
	return nil
}

// SkillListOption represents choosing from a list of skills
type SkillListOption struct {
	Skills []skills.Skill
}

// GetID returns the option's ID
func (o SkillListOption) GetID() string {
	return "skill-list"
}

// GetType returns the option type
func (o SkillListOption) GetType() OptionType {
	return OptionTypeCategory
}

// Validate checks if the option is valid
func (o SkillListOption) Validate() error {
	if len(o.Skills) == 0 {
		return fmt.Errorf("skill list cannot be empty")
	}
	return nil
}

// LanguageListOption represents choosing from a list of languages
type LanguageListOption struct {
	Languages []languages.Language
	AllowAny  bool // If true, allows any language including exotic
}

// GetID returns the option's ID
func (o LanguageListOption) GetID() string {
	if o.AllowAny {
		return "any-language"
	}
	return "language-list"
}

// GetType returns the option type
func (o LanguageListOption) GetType() OptionType {
	return OptionTypeCategory
}

// Validate checks if the option is valid
func (o LanguageListOption) Validate() error {
	if !o.AllowAny && len(o.Languages) == 0 {
		return fmt.Errorf("language list cannot be empty unless AllowAny is true")
	}
	return nil
}

// WeaponCategoryOption represents choosing from a weapon category
type WeaponCategoryOption struct {
	Category weapons.WeaponCategory // simple, martial, etc.
	Count    int                    // How many to choose (default 1)
}

// GetID returns the option's ID
func (o WeaponCategoryOption) GetID() string {
	return fmt.Sprintf("%s-weapons", o.Category)
}

// GetType returns the option type
func (o WeaponCategoryOption) GetType() OptionType {
	return OptionTypeCategory
}

// Validate checks if the option is valid
func (o WeaponCategoryOption) Validate() error {
	if o.Category == "" {
		return fmt.Errorf("weapon category is required")
	}
	// Count defaults to 1 if not set
	return nil
}
