// Package selectables provides weighted random selection infrastructure
// for probability-based choices without knowledge of what is being selected.
//
// Purpose:
// This package enables probability-based selection from weighted options,
// supporting everything from treasure generation to AI decision weighting
// without knowledge of what is being selected or why.
//
// Scope:
//   - Weighted table creation and management
//   - Probability-based selection algorithms
//   - Table composition and nesting
//   - Conditional selection based on context
//   - Deterministic selection for testing
//   - Statistical validation of weights
//   - Generic type support for any selectable
//   - Event publishing for selection tracking
//
// Non-Goals:
//   - Item definitions: Tables select IDs, not create items
//   - Drop rate balancing: Economic balance is game-specific
//   - Loot generation: Creating items/rewards is game logic
//   - Specific selection tables: Games define their own tables
//   - Selection interpretation: What to do with results is game-specific
//   - Rarity tiers: Item rarity systems are game-specific
//
// Integration:
// This package integrates with:
//   - spawn: For random entity selection from pools
//   - behavior: For weighted action selection in AI
//   - events: Publishes selection events for tracking
//
// The selectables package is purely about mathematical selection from
// weighted options. It has no opinion about what those options represent.
//
// Example:
//
//	// Create a weighted table of strings
//	table := selectables.NewBasicTable[string]()
//	table.Add("common", 70)    // 70% chance
//	table.Add("uncommon", 25)  // 25% chance
//	table.Add("rare", 5)       // 5% chance
//
//	// Select one item
//	result := table.Select()
//
//	// Conditional selection with context
//	contextTable := selectables.NewContextTable[string]()
//	contextTable.AddConditional("magic_sword", 10, func(ctx Context) bool {
//	    return ctx.Get("player_level").(int) >= 5
//	})
//
//	// Nested tables for complex selections
//	monsterTable := selectables.NewBasicTable[string]()
//	monsterTable.Add("goblin", 60)
//	monsterTable.Add("orc", 30)
//	monsterTable.Add("troll", 10)
//
//	encounterTable := selectables.NewBasicTable[*selectables.Table[string]]()
//	encounterTable.Add(monsterTable, 80)  // 80% chance of monsters
//	encounterTable.Add(trapTable, 20)     // 20% chance of traps
package selectables
