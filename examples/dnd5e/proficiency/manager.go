package proficiency

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

// Manager manages D&D 5e proficiencies for entities
type Manager struct {
	eventBus      *events.Bus
	proficiencies map[string][]proficiency.Proficiency // entity ID -> proficiencies
}

// NewManager creates a new D&D 5e proficiency manager
func NewManager(eventBus *events.Bus) *Manager {
	return &Manager{
		eventBus:      eventBus,
		proficiencies: make(map[string][]proficiency.Proficiency),
	}
}

// AddWeaponProficiency adds a weapon proficiency to an entity
func (m *Manager) AddWeaponProficiency(owner core.Entity, weapon string, source string, level int) error {
	prof := NewWeaponProficiency(owner, weapon, source, level)
	return m.addProficiency(owner.GetID(), prof)
}

// AddSkillProficiency adds a skill proficiency to an entity
func (m *Manager) AddSkillProficiency(owner core.Entity, skill Skill, source string, level int) error {
	prof := NewSkillProficiency(owner, skill, source, level)
	return m.addProficiency(owner.GetID(), prof)
}

// AddSkillExpertise adds expertise to a skill (double proficiency bonus)
func (m *Manager) AddSkillExpertise(owner core.Entity, skill Skill, source string, level int) error {
	prof := NewSkillProficiency(owner, skill, source, level)
	prof.SetExpertise(true)
	return m.addProficiency(owner.GetID(), prof)
}

// AddSavingThrowProficiency adds a saving throw proficiency
func (m *Manager) AddSavingThrowProficiency(owner core.Entity, save SavingThrow, source string, level int) error {
	prof := NewSavingThrowProficiency(owner, save, source, level)
	return m.addProficiency(owner.GetID(), prof)
}

// AddClassProficiencies adds all proficiencies for a D&D 5e class
func (m *Manager) AddClassProficiencies(owner core.Entity, className string, level int) error {
	switch strings.ToLower(className) {
	case "fighter":
		return m.addFighterProficiencies(owner, level)
	case "rogue":
		return m.addRogueProficiencies(owner, level)
	case "wizard":
		return m.addWizardProficiencies(owner, level)
	case "cleric":
		return m.addClericProficiencies(owner, level)
	// Add more classes as needed
	default:
		return fmt.Errorf("unknown class: %s", className)
	}
}

// IsProficient checks if an entity is proficient with something
func (m *Manager) IsProficient(entityID, subject string) bool {
	profs, exists := m.proficiencies[entityID]
	if !exists {
		return false
	}

	for _, prof := range profs {
		if prof.Subject() == subject {
			return true
		}

		// Check weapon categories
		if wp, ok := prof.(*WeaponProficiency); ok {
			if wp.isProficientWith(subject) {
				return true
			}
		}
	}

	return false
}

// RemoveAllProficiencies removes all proficiencies for an entity
func (m *Manager) RemoveAllProficiencies(entityID string) error {
	profs, exists := m.proficiencies[entityID]
	if !exists {
		return nil
	}

	// Remove each proficiency
	for _, prof := range profs {
		if err := prof.Remove(m.eventBus); err != nil {
			return fmt.Errorf("failed to remove proficiency %s: %w", prof.Subject(), err)
		}
	}

	delete(m.proficiencies, entityID)
	return nil
}

// Private helper methods

func (m *Manager) addProficiency(entityID string, prof proficiency.Proficiency) error {
	// Apply the proficiency to register event handlers
	if err := prof.Apply(m.eventBus); err != nil {
		return fmt.Errorf("failed to apply proficiency: %w", err)
	}

	// Track the proficiency
	if m.proficiencies[entityID] == nil {
		m.proficiencies[entityID] = []proficiency.Proficiency{}
	}
	m.proficiencies[entityID] = append(m.proficiencies[entityID], prof)

	return nil
}

// Class-specific proficiency sets

func (m *Manager) addFighterProficiencies(owner core.Entity, level int) error {
	// Armor proficiencies
	m.AddWeaponProficiency(owner, "simple-weapons", "fighter-class", level)
	m.AddWeaponProficiency(owner, "martial-weapons", "fighter-class", level)

	// Saving throws
	m.AddSavingThrowProficiency(owner, SavingThrowStrength, "fighter-class", level)
	m.AddSavingThrowProficiency(owner, SavingThrowConstitution, "fighter-class", level)

	// Skills (player would choose 2)
	// This is just an example - real implementation would handle choices
	m.AddSkillProficiency(owner, SkillAthletics, "fighter-class", level)
	m.AddSkillProficiency(owner, SkillIntimidation, "fighter-class", level)

	return nil
}

func (m *Manager) addRogueProficiencies(owner core.Entity, level int) error {
	// Weapons
	m.AddWeaponProficiency(owner, "simple-weapons", "rogue-class", level)
	m.AddWeaponProficiency(owner, "hand-crossbow", "rogue-class", level)
	m.AddWeaponProficiency(owner, "longsword", "rogue-class", level)
	m.AddWeaponProficiency(owner, "rapier", "rogue-class", level)
	m.AddWeaponProficiency(owner, "shortsword", "rogue-class", level)

	// Saving throws
	m.AddSavingThrowProficiency(owner, SavingThrowDexterity, "rogue-class", level)
	m.AddSavingThrowProficiency(owner, SavingThrowIntelligence, "rogue-class", level)

	// Skills (player would choose 4)
	// Expertise (player would choose 2) - example with Stealth
	m.AddSkillExpertise(owner, SkillStealth, "rogue-class", level)
	m.AddSkillProficiency(owner, SkillAcrobatics, "rogue-class", level)
	m.AddSkillProficiency(owner, SkillDeception, "rogue-class", level)
	m.AddSkillProficiency(owner, SkillPerception, "rogue-class", level)

	return nil
}

func (m *Manager) addWizardProficiencies(owner core.Entity, level int) error {
	// Weapons (limited)
	m.AddWeaponProficiency(owner, "dagger", "wizard-class", level)
	m.AddWeaponProficiency(owner, "dart", "wizard-class", level)
	m.AddWeaponProficiency(owner, "sling", "wizard-class", level)
	m.AddWeaponProficiency(owner, "quarterstaff", "wizard-class", level)
	m.AddWeaponProficiency(owner, "light-crossbow", "wizard-class", level)

	// Saving throws
	m.AddSavingThrowProficiency(owner, SavingThrowIntelligence, "wizard-class", level)
	m.AddSavingThrowProficiency(owner, SavingThrowWisdom, "wizard-class", level)

	// Skills (player would choose 2)
	m.AddSkillProficiency(owner, SkillArcana, "wizard-class", level)
	m.AddSkillProficiency(owner, SkillHistory, "wizard-class", level)

	return nil
}

func (m *Manager) addClericProficiencies(owner core.Entity, level int) error {
	// Weapons and armor
	m.AddWeaponProficiency(owner, "simple-weapons", "cleric-class", level)

	// Saving throws
	m.AddSavingThrowProficiency(owner, SavingThrowWisdom, "cleric-class", level)
	m.AddSavingThrowProficiency(owner, SavingThrowCharisma, "cleric-class", level)

	// Skills (player would choose 2)
	m.AddSkillProficiency(owner, SkillMedicine, "cleric-class", level)
	m.AddSkillProficiency(owner, SkillReligion, "cleric-class", level)

	return nil
}
