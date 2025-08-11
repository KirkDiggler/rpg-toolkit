// Package identifier provides a type-safe, extensible pattern for identifying
// game mechanics like features, proficiencies, skills, and conditions.
//
// The identifier system solves the tension between type safety (using constants)
// and extensibility (allowing external modules to add content). It provides a
// three-part identifier structure: Module, Type, and Value, which serializes to
// a compact string format like "core:feature:rage".
//
// Purpose: Enable both core and third-party modules to define game mechanics
// with unique identifiers while maintaining type safety where possible and
// tracking the source of each identifier (which class, race, or background
// granted it).
//
// Example usage:
//
//	// Define compile-time constants for core features
//	var Rage = identifier.MustNew("rage", "core", "feature")
//
//	// Track where features come from
//	feature := identifier.NewWithSource(Rage, "class:barbarian")
//
//	// Store in character data
//	character.Features = append(character.Features, feature)
package identifier
