package validation

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/items"
)

// BasicValidatorConfig holds configuration for the basic validator
type BasicValidatorConfig struct {

	// DefaultAttunementLimit if character doesn't specify
	DefaultAttunementLimit int

	// ClassRestrictions maps item IDs to allowed classes
	ClassRestrictions map[string][]string

	// RaceRestrictions maps item IDs to allowed races
	RaceRestrictions map[string][]string

	// AlignmentRestrictions maps item IDs to allowed alignments
	AlignmentRestrictions map[string][]string
}

// BasicValidator provides standard equipment validation
type BasicValidator struct {
	defaultAttunementLimit int
	classRestrictions      map[string][]string
	raceRestrictions       map[string][]string
	alignmentRestrictions  map[string][]string
}

// NewBasicValidator creates a new basic equipment validator
func NewBasicValidator(config BasicValidatorConfig) *BasicValidator {
	limit := config.DefaultAttunementLimit
	if limit <= 0 {
		limit = 3 // D&D 5e default
	}

	return &BasicValidator{
		defaultAttunementLimit: limit,
		classRestrictions:      config.ClassRestrictions,
		raceRestrictions:       config.RaceRestrictions,
		alignmentRestrictions:  config.AlignmentRestrictions,
	}
}

// CanEquip checks if a character can equip an item to a specific slot
func (v *BasicValidator) CanEquip(character Character, item items.EquippableItem, slot string) error {
	// Check if slot is valid for this item
	if err := v.validateSlotCompatibility(item, slot); err != nil {
		return err
	}

	// Check if slot is already occupied
	if err := v.validateSlotAvailability(character, item, slot); err != nil {
		return err
	}

	// Check class/race/alignment restrictions
	if err := v.validateRestrictions(character, item); err != nil {
		return err
	}

	// Check specific item type requirements
	switch typedItem := item.(type) {
	case items.WeaponItem:
		if err := v.validateWeaponRequirements(character, typedItem); err != nil {
			return err
		}
	case items.ArmorItem:
		if err := v.validateArmorRequirements(character, typedItem); err != nil {
			return err
		}
	}

	// Check attunement if required
	if item.RequiresAttunement() {
		if err := v.validateAttunementAvailable(character); err != nil {
			return err
		}
	}

	return nil
}

// CanUnequip checks if a character can unequip an item from a specific slot
func (v *BasicValidator) CanUnequip(character Character, slot string) error {
	equipped := character.GetEquippedItems()
	if _, exists := equipped[slot]; !exists {
		return core.NewEquipmentError("unequip", character.GetID(), "", slot,
			core.ErrIncompatibleSlot)
	}

	// In basic implementation, items can always be unequipped
	// Games might add cursed items or other restrictions
	return nil
}

// CanAttune checks if a character can attune to an item
func (v *BasicValidator) CanAttune(character Character, item items.EquippableItem) error {
	if !item.IsAttunable() {
		return core.NewEquipmentError("attune", character.GetID(), item.GetID(), "",
			core.ErrRequiresAttunement)
	}

	// Check attunement limit
	if err := v.validateAttunementAvailable(character); err != nil {
		return err
	}

	// Check restrictions
	if err := v.validateRestrictions(character, item); err != nil {
		return err
	}

	return nil
}

// CanUseWeapon checks if a character can effectively use a weapon
func (v *BasicValidator) CanUseWeapon(character Character, weapon items.WeaponItem) error {
	return v.validateWeaponRequirements(character, weapon)
}

// CanWearArmor checks if a character can effectively wear armor
func (v *BasicValidator) CanWearArmor(character Character, armor items.ArmorItem) error {
	return v.validateArmorRequirements(character, armor)
}

// ValidateEquipmentSet checks if the entire equipment set is valid
func (v *BasicValidator) ValidateEquipmentSet(character Character) []error {
	var errors []error

	equipped := character.GetEquippedItems()

	// Check for two-handed conflicts
	if err := v.validateTwoHandedConflicts(equipped); err != nil {
		errors = append(errors, err)
	}

	// Validate each equipped item
	for slot, item := range equipped {
		if equippable, ok := item.(items.EquippableItem); ok {
			if err := v.CanEquip(character, equippable, slot); err != nil {
				errors = append(errors, err)
			}
		}
	}

	return errors
}

// validateSlotCompatibility checks if an item can go in the specified slot
func (v *BasicValidator) validateSlotCompatibility(item items.EquippableItem, slot string) error {
	validSlots := item.GetValidSlots()
	for _, validSlot := range validSlots {
		if validSlot == slot {
			return nil
		}
	}

	return core.NewEquipmentError("equip", "", item.GetID(), slot,
		core.ErrIncompatibleSlot)
}

// validateSlotAvailability checks if required slots are available
func (v *BasicValidator) validateSlotAvailability(character Character, item items.EquippableItem, _ string) error {
	equipped := character.GetEquippedItems()
	requiredSlots := item.GetRequiredSlots()

	// Check each required slot
	for _, reqSlot := range requiredSlots {
		if existing, occupied := equipped[reqSlot]; occupied {
			// If it's the same item, that's ok (re-equipping)
			if existing.GetID() != item.GetID() {
				return core.NewEquipmentError("equip", character.GetID(), item.GetID(), reqSlot,
					core.ErrSlotOccupied)
			}
		}
	}

	return nil
}

// validateWeaponRequirements checks weapon-specific requirements
func (v *BasicValidator) validateWeaponRequirements(character Character, weapon items.WeaponItem) error {
	// Check proficiency
	requiredProf := weapon.GetRequiredProficiency()
	if requiredProf != "" {
		if !v.hasProficiency(character, requiredProf) {
			return core.NewEquipmentError("equip", character.GetID(), weapon.GetID(), "",
				core.ErrMissingProficiency)
		}
	}

	return nil
}

// validateArmorRequirements checks armor-specific requirements
func (v *BasicValidator) validateArmorRequirements(character Character, armor items.ArmorItem) error {
	// Check strength requirement
	if armor.GetStrengthRequirement() > 0 {
		if character.GetStrength() < armor.GetStrengthRequirement() {
			return core.NewEquipmentError("equip", character.GetID(), armor.GetID(), "",
				core.ErrInsufficientStrength)
		}
	}

	// Check proficiency
	requiredProf := armor.GetRequiredProficiency()
	if requiredProf != "" {
		if !v.hasProficiency(character, requiredProf) {
			return core.NewEquipmentError("equip", character.GetID(), armor.GetID(), "",
				core.ErrMissingProficiency)
		}
	}

	return nil
}

// validateAttunementAvailable checks if character can attune to another item
func (v *BasicValidator) validateAttunementAvailable(character Character) error {
	limit := character.GetAttunementLimit()
	if limit <= 0 {
		limit = v.defaultAttunementLimit
	}

	attuned := character.GetAttunedItems()
	if len(attuned) >= limit {
		return core.NewEquipmentError("attune", character.GetID(), "", "",
			core.ErrAttunementLimit)
	}

	return nil
}

// validateRestrictions checks class/race/alignment restrictions
func (v *BasicValidator) validateRestrictions(character Character, item items.Item) error {
	itemID := item.GetID()

	// Check class restrictions
	if classes, exists := v.classRestrictions[itemID]; exists {
		if !v.contains(classes, character.GetClass()) {
			return core.NewEquipmentError("equip", character.GetID(), itemID, "",
				core.ErrClassRestriction)
		}
	}

	// Check race restrictions
	if races, exists := v.raceRestrictions[itemID]; exists {
		if !v.contains(races, character.GetRace()) {
			return core.NewEquipmentError("equip", character.GetID(), itemID, "",
				core.ErrRaceRestriction)
		}
	}

	// Check alignment restrictions
	if alignments, exists := v.alignmentRestrictions[itemID]; exists {
		if !v.contains(alignments, character.GetAlignment()) {
			return core.NewEquipmentError("equip", character.GetID(), itemID, "",
				core.ErrAlignmentRestriction)
		}
	}

	return nil
}

// validateTwoHandedConflicts checks for two-handed weapon conflicts
func (v *BasicValidator) validateTwoHandedConflicts(equipped map[string]items.Item) error {
	var mainHand, offHand items.Item
	var twoHandedWeapon items.WeaponItem

	// Find weapons in hand slots
	if item, exists := equipped["main_hand"]; exists {
		mainHand = item
		if weapon, ok := item.(items.WeaponItem); ok && weapon.IsTwoHanded() {
			twoHandedWeapon = weapon
		}
	}

	if item, exists := equipped["off_hand"]; exists {
		offHand = item
	}

	// Check for conflicts
	if twoHandedWeapon != nil && offHand != nil {
		return core.NewEquipmentError("validate", "", twoHandedWeapon.GetID(), "main_hand",
			core.ErrTwoHandedConflict)
	}

	// Check if off-hand has two-handed weapon
	if offWeapon, ok := offHand.(items.WeaponItem); ok && offWeapon.IsTwoHanded() {
		return core.NewEquipmentError("validate", "", offWeapon.GetID(), "off_hand",
			core.ErrTwoHandedConflict)
	}

	// If main hand weapon exists and something is in off-hand
	if mainWeapon, ok := mainHand.(items.WeaponItem); ok && mainWeapon.IsTwoHanded() && offHand != nil {
		return core.NewEquipmentError("validate", "", mainWeapon.GetID(), "main_hand",
			core.ErrTwoHandedConflict)
	}

	return nil
}

// hasProficiency checks if character has a proficiency
func (v *BasicValidator) hasProficiency(character Character, proficiency string) bool {
	profs := character.GetProficiencies()
	for _, prof := range profs {
		if prof == proficiency {
			return true
		}
	}
	return false
}

// contains checks if a slice contains a value
func (v *BasicValidator) contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
