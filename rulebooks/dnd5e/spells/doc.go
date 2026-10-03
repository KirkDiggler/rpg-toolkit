// Package spells owns spell identities, catalogue metadata and executable cast
// content for the D&D 5e rulebook.
//
// SpellData owns the names and descriptions of catalogued spells. GetData,
// GetSpellsByLevel, Name and Description read that metadata; CastDefinition
// uses the same name when it compiles an executable action. Name also retains
// name-only identifiers that do not yet have catalogue entries, without a
// second authored name for any catalogued spell.
//
// Catalogue reads require no character, cast, save DC, roller or session. They
// do not grant spell access or establish that a spell can be executed. Castable
// and Selectable remain the entry points for those separate content policies.
// Executable mechanics are authored in the cast profiles and bound to the
// caster's facts by CastDefinition; catalogue metadata is not a second rules
// implementation.
package spells
