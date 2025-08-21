// Package armor provides D&D 5e armor data with automatic AC calculations.
//
// THE MAGIC: Armor that knows its own limits - MaxDexBonus automatically caps your AC contribution.
//
// Example:
//
//	plate := armor.GetByID(armor.Plate)
//	ac := plate.AC // Base 18
//	// plate.MaxDexBonus is 0 - no dex bonus in heavy armor!
//	// System automatically handles: AC = 18 + min(dexMod, 0) = 18
//
// KEY INSIGHT: The MaxDexBonus pointer pattern - nil means unlimited (light armor),
// integer means capped (medium/heavy). This single field drives all AC calculations
// without special cases for armor categories.
//
// The pattern handles:
//   - Light armor: Full Dex bonus (MaxDexBonus = nil)
//   - Medium armor: Capped at +2 (MaxDexBonus = &2)
//   - Heavy armor: No Dex bonus (MaxDexBonus = &0)
//   - Shields: Simple AC bonus (+2)
//
// StealthDisadvantage and Strength requirements are data, not code. Games
// check these properties to enforce rules without the armor package knowing
// what "stealth" or "strength" mean.
package armor
