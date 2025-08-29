package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// StartingClass represents a valid starting class option for character creation
type StartingClass struct {
	ID           string                   // "fighter" or "life-domain"
	Name         string                   // "Fighter" or "Life Domain"
	Description  string                   // Brief description
	Group        classes.Class            // For UI grouping (Fighter, Cleric, etc.)
	Grants       *classes.AutomaticGrants // What you automatically get
	Requirements *choices.Requirements    // What choices you must make
}

// ListStartingClasses returns all valid starting class options
// For classes with level 1 subclasses (Cleric, Sorcerer, Warlock), only subclasses are returned
// For other classes, the base class is returned
func ListStartingClasses() []StartingClass {
	// Pre-allocate with estimated capacity (12 base classes + ~15 subclasses)
	startingClasses := make([]StartingClass, 0, 27)

	// Fighter - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Fighter),
		Name:         classes.Fighter.String(),
		Description:  classes.Fighter.Description(),
		Group:        classes.Fighter,
		Grants:       classes.GetAutomaticGrants(classes.Fighter),
		Requirements: choices.GetClassRequirements(classes.Fighter),
	})

	// Barbarian - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Barbarian),
		Name:         classes.Barbarian.String(),
		Description:  classes.Barbarian.Description(),
		Group:        classes.Barbarian,
		Grants:       classes.GetAutomaticGrants(classes.Barbarian),
		Requirements: choices.GetClassRequirements(classes.Barbarian),
	})

	// Bard - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Bard),
		Name:         classes.Bard.String(),
		Description:  classes.Bard.Description(),
		Group:        classes.Bard,
		Grants:       classes.GetAutomaticGrants(classes.Bard),
		Requirements: choices.GetClassRequirements(classes.Bard),
	})

	// Cleric - subclass at level 1, so add all subclasses
	clericSubclasses := []classes.Subclass{
		classes.LifeDomain,
		classes.LightDomain,
		classes.NatureDomain,
		classes.TempestDomain,
		classes.TrickeryDomain,
		classes.WarDomain,
		classes.KnowledgeDomain,
		classes.DeathDomain,
	}
	for _, subclass := range clericSubclasses {
		startingClasses = append(startingClasses, StartingClass{
			ID:           string(subclass),
			Name:         subclass.String(),
			Description:  subclass.Description(),
			Group:        classes.Cleric,
			Grants:       getSubclassGrants(subclass),
			Requirements: getSubclassRequirements(subclass),
		})
	}

	// Druid - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Druid),
		Name:         classes.Druid.String(),
		Description:  classes.Druid.Description(),
		Group:        classes.Druid,
		Grants:       classes.GetAutomaticGrants(classes.Druid),
		Requirements: choices.GetClassRequirements(classes.Druid),
	})

	// Monk - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Monk),
		Name:         classes.Monk.String(),
		Description:  classes.Monk.Description(),
		Group:        classes.Monk,
		Grants:       classes.GetAutomaticGrants(classes.Monk),
		Requirements: choices.GetClassRequirements(classes.Monk),
	})

	// Paladin - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Paladin),
		Name:         classes.Paladin.String(),
		Description:  classes.Paladin.Description(),
		Group:        classes.Paladin,
		Grants:       classes.GetAutomaticGrants(classes.Paladin),
		Requirements: choices.GetClassRequirements(classes.Paladin),
	})

	// Ranger - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Ranger),
		Name:         classes.Ranger.String(),
		Description:  classes.Ranger.Description(),
		Group:        classes.Ranger,
		Grants:       classes.GetAutomaticGrants(classes.Ranger),
		Requirements: choices.GetClassRequirements(classes.Ranger),
	})

	// Rogue - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Rogue),
		Name:         classes.Rogue.String(),
		Description:  classes.Rogue.Description(),
		Group:        classes.Rogue,
		Grants:       classes.GetAutomaticGrants(classes.Rogue),
		Requirements: choices.GetClassRequirements(classes.Rogue),
	})

	// Sorcerer - subclass at level 1
	sorcererSubclasses := []classes.Subclass{
		classes.DraconicBloodline,
		classes.WildMagic,
		classes.DivineSoul,
	}
	for _, subclass := range sorcererSubclasses {
		startingClasses = append(startingClasses, StartingClass{
			ID:           string(subclass),
			Name:         subclass.String(),
			Description:  subclass.Description(),
			Group:        classes.Sorcerer,
			Grants:       getSubclassGrants(subclass),
			Requirements: getSubclassRequirements(subclass),
		})
	}

	// Warlock - subclass at level 1
	warlockSubclasses := []classes.Subclass{
		classes.Archfey,
		classes.Fiend,
		classes.GreatOldOne,
		classes.Hexblade,
	}
	for _, subclass := range warlockSubclasses {
		startingClasses = append(startingClasses, StartingClass{
			ID:           string(subclass),
			Name:         subclass.String(),
			Description:  subclass.Description(),
			Group:        classes.Warlock,
			Grants:       getSubclassGrants(subclass),
			Requirements: getSubclassRequirements(subclass),
		})
	}

	// Wizard - no subclass at level 1
	startingClasses = append(startingClasses, StartingClass{
		ID:           string(classes.Wizard),
		Name:         classes.Wizard.String(),
		Description:  classes.Wizard.Description(),
		Group:        classes.Wizard,
		Grants:       classes.GetAutomaticGrants(classes.Wizard),
		Requirements: choices.GetClassRequirements(classes.Wizard),
	})

	return startingClasses
}

// getSubclassGrants returns the complete grants for a subclass (base + subclass specific)
func getSubclassGrants(subclass classes.Subclass) *classes.AutomaticGrants {
	// Use the new GetSubclassGrants function from the classes package
	return classes.GetSubclassGrants(subclass)
}

// getSubclassRequirements returns the complete requirements for a subclass
func getSubclassRequirements(subclass classes.Subclass) *choices.Requirements {
	// Use the new GetSubclassRequirements function from the choices package
	return choices.GetSubclassRequirements(subclass)
}
