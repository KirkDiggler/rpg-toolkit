// Package proficiency provides infrastructure for tracking and querying
// entity proficiencies without defining what those proficiencies represent.
//
// Purpose:
// This package manages proficiency states (trained, expert, etc.) for
// various entity capabilities while remaining agnostic to what those
// proficiencies mean mechanically or how they affect gameplay.
//
// Scope:
//   - Proficiency levels and states
//   - Proficiency categories and grouping
//   - Proficiency queries and checks
//   - Proficiency acquisition tracking
//   - Proficiency prerequisites
//   - Temporary proficiency grants
//
// Non-Goals:
//   - Mechanical bonuses: What proficiency adds to rolls is game-specific
//   - Proficiency lists: What can be proficient in is game-specific
//   - Training rules: How proficiencies are gained is game logic
//   - Expertise rules: When/how to gain expertise is game-specific
//   - Class/race proficiencies: Sources of proficiency are game-specific
//   - Skill systems: The relationship to skills is game-specific
//
// Integration:
// This package integrates with:
//   - features: Features may grant proficiencies
//   - events: Publishes proficiency gained/lost events
//
// Games define what proficiencies exist and how they work mechanically,
// while this package tracks which ones an entity has.
//
// Example:
//
//	// Define proficiency levels (game-specific)
//	type ProficiencyLevel int
//	const (
//	    NotProficient ProficiencyLevel = iota
//	    Proficient
//	    Expertise
//	    Mastery
//	)
//
//	// Create proficiency tracker
//	prof := proficiency.NewTracker()
//
//	// Add proficiencies
//	prof.Add("longsword", Proficient)
//	prof.Add("athletics", Expertise)
//	prof.AddCategory("simple_weapons", Proficient)
//
//	// Query proficiencies
//	if prof.IsProficient("longsword") {
//	    // Add proficiency bonus to attack roll
//	}
//
//	level := prof.GetLevel("athletics")
//	if level >= Expertise {
//	    // Double proficiency bonus
//	}
//
//	// Temporary proficiency
//	prof.AddTemporary("thieves_tools", Proficient, time.Hour)
//
//	// Check category proficiency
//	if prof.HasCategoryProficiency("martial_weapons") {
//	    // Can use any martial weapon
//	}
package proficiency
