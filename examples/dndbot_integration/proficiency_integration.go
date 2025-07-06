// Package dndbot shows proficiency system integration
package dndbot

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

// ProficiencyIntegration shows how to use toolkit's proficiency system
type ProficiencyIntegration struct {
	eventBus      *events.Bus
	proficiencies map[string][]proficiency.Proficiency // Character ID -> proficiencies
}

// NewProficiencyIntegration creates a new proficiency integration
func NewProficiencyIntegration(bus *events.Bus) *ProficiencyIntegration {
	return &ProficiencyIntegration{
		eventBus:      bus,
		proficiencies: make(map[string][]proficiency.Proficiency),
	}
}

// AddCharacterProficiencies adds proficiencies when a character is created/loaded
func (p *ProficiencyIntegration) AddCharacterProficiencies(characterID string, level int) error {
	// Wrap the character
	character := WrapCharacter(characterID, "Fighter", level)

	// Initialize proficiency list for this character
	p.proficiencies[characterID] = []proficiency.Proficiency{}

	// Calculate proficiency bonus
	profBonus := 2 + ((level - 1) / 4) // D&D 5e formula

	// Add weapon proficiencies
	weaponProfs := []string{"simple-weapons", "martial-weapons", "longsword", "shortsword"}
	for _, wpn := range weaponProfs {
		prof := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
			ID:      fmt.Sprintf("%s-prof-%s", characterID, wpn),
			Owner:   character,
			Subject: wpn,
			Source:  "fighter-class",
		})

		// Apply the proficiency to register its effects
		if err := prof.Apply(p.eventBus); err != nil {
			return err
		}

		p.proficiencies[characterID] = append(p.proficiencies[characterID], prof)
	}

	// Add armor proficiencies
	armorProfs := []string{"light-armor", "medium-armor", "heavy-armor", "shields"}
	for _, armor := range armorProfs {
		prof := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
			ID:      fmt.Sprintf("%s-prof-%s", characterID, armor),
			Owner:   character,
			Subject: armor,
			Source:  "fighter-class",
		})

		if err := prof.Apply(p.eventBus); err != nil {
			return err
		}
		p.proficiencies[characterID] = append(p.proficiencies[characterID], prof)
	}

	// Add skill proficiencies (e.g., Athletics, Intimidation)
	skillProfs := []string{"athletics", "intimidation"}
	for _, skill := range skillProfs {
		prof := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
			ID:      fmt.Sprintf("%s-prof-%s", characterID, skill),
			Owner:   character,
			Subject: skill,
			Source:  "fighter-class",
		})

		if err := prof.Apply(p.eventBus); err != nil {
			return err
		}
		p.proficiencies[characterID] = append(p.proficiencies[characterID], prof)
	}

	// Add saving throw proficiencies
	saveProfs := []string{"strength-save", "constitution-save"}
	for _, save := range saveProfs {
		prof := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
			ID:      fmt.Sprintf("%s-prof-%s", characterID, save),
			Owner:   character,
			Subject: save,
			Source:  "fighter-class",
		})

		if err := prof.Apply(p.eventBus); err != nil {
			return err
		}
		p.proficiencies[characterID] = append(p.proficiencies[characterID], prof)
	}

	fmt.Printf("Added proficiencies for character %s (level %d, bonus +%d)\n",
		characterID, level, profBonus)

	return nil
}

// CheckProficiency checks if a character is proficient in something
func (p *ProficiencyIntegration) CheckProficiency(characterID, subject string) bool {
	profs, exists := p.proficiencies[characterID]
	if !exists {
		return false
	}

	// Check all proficiencies for this character
	for _, prof := range profs {
		if prof.Subject() == subject {
			return true
		}

		// Check category proficiencies
		switch prof.Subject() {
		case "simple-weapons":
			if isSimpleWeapon(subject) {
				return true
			}
		case "martial-weapons":
			if isMartialWeapon(subject) {
				return true
			}
		}
	}

	return false
}

// GetProficiencyBonus returns the proficiency bonus for a character level
func GetProficiencyBonus(level int) int {
	return 2 + ((level - 1) / 4)
}

// Helper functions for weapon categories
func isSimpleWeapon(weapon string) bool {
	simpleWeapons := map[string]bool{
		"club": true, "dagger": true, "greatclub": true,
		"handaxe": true, "javelin": true, "light-hammer": true,
		"mace": true, "quarterstaff": true, "sickle": true,
		"spear": true, "light-crossbow": true, "dart": true,
		"shortbow": true, "sling": true,
	}
	return simpleWeapons[weapon]
}

func isMartialWeapon(weapon string) bool {
	martialWeapons := map[string]bool{
		"battleaxe": true, "flail": true, "glaive": true,
		"greataxe": true, "greatsword": true, "halberd": true,
		"lance": true, "longsword": true, "maul": true,
		"morningstar": true, "pike": true, "rapier": true,
		"scimitar": true, "shortsword": true, "trident": true,
		"war-pick": true, "warhammer": true, "whip": true,
		"blowgun": true, "hand-crossbow": true, "heavy-crossbow": true,
		"longbow": true, "net": true,
	}
	return martialWeapons[weapon]
}

// MigrateCharacterProficiencies shows how to migrate existing character proficiencies
func (p *ProficiencyIntegration) MigrateCharacterProficiencies(_ interface{}) error {
	// This would be the actual migration from DND bot's character struct
	// Example pseudo-code:
	/*
		character := char.(*character.Character)

		for profType, profs := range character.Proficiencies {
			for _, prof := range profs {
				// Convert DND bot proficiency to toolkit proficiency
				toolkitProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
					ID:      fmt.Sprintf("%s-prof-%s", character.ID, prof.Key),
					Owner:   WrapCharacter(character),
					Subject: prof.Key,
					Source:  prof.Source,
				})

				manager.AddProficiency(toolkitProf, p.eventBus)
			}
		}
	*/

	return nil
}
