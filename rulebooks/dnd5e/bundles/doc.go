// Package bundles provides D&D 5e equipment bundle identifiers.
//
// THE MAGIC: Named bundles that expand into complete equipment sets - "explorer's pack" becomes 10+ items.
//
// Example:
//
//	bundle := bundles.ExplorersPack
//	// Game expands this to: bedroll, mess kit, tinderbox, 10 torches,
//	// 10 days rations, waterskin, 50ft rope...
//
// KEY INSIGHT: Bundles are just IDs - the expansion happens elsewhere. This separation
// means bundles can be referenced in choices ("Choose: explorer's pack OR dungeoneer's pack")
// without the choice system knowing what's inside them.
//
// The pattern enables:
//   - Class equipment choices: "martial weapon and shield" as a single option
//   - Starting packs: All the tedious starting gear in one selection
//   - Custom bundles: Games can define new bundles using the same pattern
//
// This package provides constants only. The actual bundle contents are defined
// where equipment is managed, keeping the bundle concept separate from its
// implementation.
package bundles
