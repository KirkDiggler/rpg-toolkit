// Package validation provides equipment validation rules and constraints
// for the RPG toolkit items system.
//
// Purpose:
// This package validates whether items can be equipped, used, or attuned
// based on character attributes, proficiencies, and current equipment state.
//
// Scope:
//   - Equipment slot validation
//   - Proficiency requirements
//   - Attribute requirements (strength, etc.)
//   - Two-handed weapon conflicts
//   - Attunement limits and requirements
//   - Class/race/alignment restrictions
//   - Equipment compatibility checks
//
// Integration:
// This package integrates with:
//   - items: Validates item properties
//   - core: Uses error types for validation failures
//   - events: Publishes validation events
//   - proficiency: Checks character proficiencies
//
// Games define specific validation rules while this package provides
// the framework for enforcing them.
package validation
