// Package weapons provides D&D 5e weapon data and categorization infrastructure.
//
// THE MAGIC: Weapons that know their own properties - finesse, versatile, thrown - without you checking.
//
// Example:
//
//	sword := weapons.GetByID(weapons.Longsword)
//	if sword.HasProperty(weapons.PropertyVersatile) {
//	    // Automatically handles 1d8 one-handed or 1d10 two-handed
//	}
//
// KEY INSIGHT: Properties drive mechanics. Instead of hardcoding "if weaponID == 'dagger'",
// the system asks "does it have PropertyFinesse?" This makes homebrew weapons work
// automatically if they declare the right properties.
//
// The pattern enables:
//   - Rogues sneak attacking with ANY finesse weapon
//   - Monks using ANY weapon without the heavy property
//   - Great Weapon Fighting rerolling damage for ANY two-handed weapon
//
// This is pure data infrastructure. The package doesn't implement attack rolls
// or damage calculations - it provides the weapon data that game systems need
// to make those decisions.
package weapons
