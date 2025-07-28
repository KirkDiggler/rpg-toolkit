// Package constants provides type-safe constants for D&D 5e game elements.
//
// Purpose:
// This package serves as the single source of truth for all D&D 5e constants
// used throughout the rpg-toolkit and any consuming applications like rpg-api.
// It provides both programmatic values and human-readable display names.
//
// Design:
// Each constant type is a string-based type with associated methods:
//   - Display() returns the human-readable form
//   - Additional helper methods provide game-specific relationships
//
// The constants use lowercase values for consistency and readability,
// while Display() methods provide properly formatted names for UI display.
//
// Example:
//
//	ability := constants.STR
//	fmt.Println(ability)          // "str"
//	fmt.Println(ability.Display()) // "Strength"
//
//	language := constants.LanguageElvish
//	fmt.Println(language)          // "elvish"
//	fmt.Println(language.Display()) // "Elvish"
package constants
