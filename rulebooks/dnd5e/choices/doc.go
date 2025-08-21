// Package choices provides D&D 5e character creation choice infrastructure.
//
// THE MAGIC: Choices that resolve themselves - from "choose 2 martial weapons" to actual swords in inventory.
//
// Example:
//
//	choice := FighterEquipment1 // "Choose martial weapon and shield OR two martial weapons"
//	selected := []string{"longsword", "shield"}
//	equipment := choice.Resolve(selected) // Returns actual weapon/armor objects, not just IDs
//
// KEY INSIGHT: The choice system transforms abstract decisions ("pick a martial weapon")
// into concrete game objects (Weapon{ID: "longsword", Damage: "1d8"}) automatically.
// Players never see category IDs or bundles - just the final items they own.
//
// The journey of a choice:
//  1. Game presents: "Choose 2 skills from: Athletics, Intimidation, Survival..."
//  2. Player selects: ["Athletics", "Survival"]
//  3. System validates: Are these valid options? Does count match?
//  4. Resolution: Transforms selections into actual skill proficiencies
//
// This is infrastructure, not implementation. The choice system doesn't know
// what skills do or how weapons work - it just ensures valid selections become
// real game objects.
package choices
