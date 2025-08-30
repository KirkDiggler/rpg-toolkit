package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// StartingClass represents a valid starting class option for character creation
type StartingClass struct {
	ID           classes.Class            // "fighter" or "life-domain"
	Grants       *classes.AutomaticGrants // What you automatically get
	Requirements *choices.Requirements    // What choices you must make
	Subclass     []*SubclassOption        // Level 1 subclasses if applicable
}

// SubclassOption represents a subclass choice available at character creation
type SubclassOption struct {
	ID           classes.Subclass
	Level        int // When you get this subclass
	Grants       *classes.AutomaticGrants
	Requirements *choices.Requirements
}

// ListStartingClasses returns all valid starting class options
// For classes with level 1 subclasses (Cleric, Sorcerer, Warlock), only subclasses are returned
// For other classes, the base class is returned
func ListStartingClasses() []*StartingClass {
	// Pre-allocate with estimated capacity (12 base classes + ~15 subclasses)
	startingClasses := make([]*StartingClass, 0, 27)

	// Fighter - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Fighter,
		Grants:       classes.GetAutomaticGrants(classes.Fighter),
		Requirements: choices.GetClassRequirements(classes.Fighter),
	})

	// Barbarian - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Barbarian,
		Grants:       classes.GetAutomaticGrants(classes.Barbarian),
		Requirements: choices.GetClassRequirements(classes.Barbarian),
	})

	// Bard - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Bard,
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

	cleric := &StartingClass{
		ID:           classes.Cleric,
		Grants:       classes.GetAutomaticGrants(classes.Cleric),
		Requirements: choices.GetClassRequirements(classes.Cleric),
		Subclass:     make([]*SubclassOption, 0, len(clericSubclasses)),
	}

	for _, subclass := range clericSubclasses {
		cleric.Subclass = append(cleric.Subclass, &SubclassOption{
			ID:           subclass,
			Level:        1,
			Grants:       getSubclassGrants(subclass),
			Requirements: getSubclassRequirements(subclass),
		})
	}

	startingClasses = append(startingClasses, cleric)

	// Druid - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Druid,
		Grants:       classes.GetAutomaticGrants(classes.Druid),
		Requirements: choices.GetClassRequirements(classes.Druid),
	})

	// Monk - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Monk,
		Grants:       classes.GetAutomaticGrants(classes.Monk),
		Requirements: choices.GetClassRequirements(classes.Monk),
	})

	// Paladin - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Paladin,
		Grants:       classes.GetAutomaticGrants(classes.Paladin),
		Requirements: choices.GetClassRequirements(classes.Paladin),
	})

	// Ranger - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Ranger,
		Grants:       classes.GetAutomaticGrants(classes.Ranger),
		Requirements: choices.GetClassRequirements(classes.Ranger),
	})

	// Rogue - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Rogue,
		Grants:       classes.GetAutomaticGrants(classes.Rogue),
		Requirements: choices.GetClassRequirements(classes.Rogue),
	})
	// Sorcerer - subclass at level 1
	sorcererSubclasses := []classes.Subclass{
		classes.DraconicBloodline,
		classes.WildMagic,
		classes.DivineSoul,
	}

	sorcerer := &StartingClass{
		ID:           classes.Sorcerer,
		Grants:       classes.GetAutomaticGrants(classes.Sorcerer),
		Requirements: choices.GetClassRequirements(classes.Sorcerer),
		Subclass:     make([]*SubclassOption, 0, len(sorcererSubclasses)),
	}

	for _, subclass := range sorcererSubclasses {
		sorcerer.Subclass = append(sorcerer.Subclass, &SubclassOption{
			ID:           subclass,
			Level:        1,
			Grants:       getSubclassGrants(subclass),
			Requirements: getSubclassRequirements(subclass),
		})
	}
	startingClasses = append(startingClasses, sorcerer)

	// Warlock - subclass at level 1
	warlockSubclasses := []classes.Subclass{
		classes.Archfey,
		classes.Fiend,
		classes.GreatOldOne,
		classes.Hexblade,
	}

	warlock := &StartingClass{
		ID:           classes.Warlock,
		Grants:       classes.GetAutomaticGrants(classes.Warlock),
		Requirements: choices.GetClassRequirements(classes.Warlock),
		Subclass:     make([]*SubclassOption, 0, len(warlockSubclasses)),
	}

	for _, subclass := range warlockSubclasses {
		warlock.Subclass = append(warlock.Subclass, &SubclassOption{
			ID:           subclass,
			Level:        1,
			Grants:       getSubclassGrants(subclass),
			Requirements: getSubclassRequirements(subclass),
		})
	}
	startingClasses = append(startingClasses, warlock)

	// Wizard - no subclass at level 1
	startingClasses = append(startingClasses, &StartingClass{
		ID:           classes.Wizard,
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
