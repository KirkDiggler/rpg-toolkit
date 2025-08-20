package choices

// ChoiceID represents a unique identifier for a choice
type ChoiceID string

// Fighter choice IDs
const (
	FighterSkills     ChoiceID = "fighter-skills"
	FighterEquipment1 ChoiceID = "fighter-equipment-armor"
	FighterEquipment2 ChoiceID = "fighter-equipment-primary-weapon"
	FighterEquipment3 ChoiceID = "fighter-equipment-secondary"
	FighterEquipment4 ChoiceID = "fighter-equipment-ranged"
	FighterEquipment5 ChoiceID = "fighter-equipment-pack"
)

// Rogue choice IDs
const (
	RogueSkills     ChoiceID = "rogue-skills"
	RogueEquipment1 ChoiceID = "rogue-equipment-primary-weapon"
	RogueEquipment2 ChoiceID = "rogue-equipment-secondary"
	RogueEquipment3 ChoiceID = "rogue-equipment-pack"
	RogueExpertise  ChoiceID = "rogue-expertise"
)

// Wizard choice IDs
const (
	WizardSkills     ChoiceID = "wizard-skills"
	WizardCantrips   ChoiceID = "wizard-cantrips"
	WizardSpells     ChoiceID = "wizard-spells-level-1"
	WizardEquipment1 ChoiceID = "wizard-equipment-primary-weapon"
	WizardEquipment2 ChoiceID = "wizard-equipment-focus"
	WizardEquipment3 ChoiceID = "wizard-equipment-pack"
)

// Cleric choice IDs
const (
	ClericSkills     ChoiceID = "cleric-skills"
	ClericCantrips   ChoiceID = "cleric-cantrips"
	ClericEquipment1 ChoiceID = "cleric-equipment-primary-weapon"
	ClericEquipment2 ChoiceID = "cleric-equipment-armor"
	ClericEquipment3 ChoiceID = "cleric-equipment-secondary"
	ClericEquipment4 ChoiceID = "cleric-equipment-pack"
)
