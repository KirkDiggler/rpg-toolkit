// Package main shows a simplified integration example
package main

import (
	"fmt"
)

func main() {
	fmt.Println("=== D&D Bot → RPG Toolkit Integration ===")
	fmt.Println()

	fmt.Println("The rpg-toolkit provides these key components:")
	fmt.Println("1. Event Bus - Replace bot's event system")
	fmt.Println("2. Resources - Manage spell slots, rage uses, ki points")
	fmt.Println("3. Conditions - Status effects like poisoned, stunned")
	fmt.Println("4. Dice - Dice rolling with modifiers")
	fmt.Println("5. Proficiency - Track proficiencies")
	fmt.Println()

	fmt.Println("Key Integration Points:")
	fmt.Println()

	fmt.Println("1. Entity Wrapper:")
	fmt.Println("   type CharacterWrapper struct {")
	fmt.Println("       *character.Character // Your existing character")
	fmt.Println("   }")
	fmt.Println("   func (c *CharacterWrapper) GetID() string { return c.Character.ID }")
	fmt.Println("   func (c *CharacterWrapper) GetType() string { return \"character\" }")
	fmt.Println()

	fmt.Println("2. Spell Slots as Resources:")
	fmt.Println("   - Each spell level is a resource")
	fmt.Println("   - Automatically restore on long rest")
	fmt.Println("   - Track current/max for each level")
	fmt.Println()

	fmt.Println("3. Conditions for Status Effects:")
	fmt.Println("   - Poisoned: Disadvantage on attacks/checks")
	fmt.Println("   - Stunned: Can't take actions")
	fmt.Println("   - Prone: Melee advantage, ranged disadvantage")
	fmt.Println()

	fmt.Println("4. Event-Driven Combat:")
	fmt.Println("   - Publish \"attack.roll\" events")
	fmt.Println("   - Conditions modify attack rolls")
	fmt.Println("   - Resources consumed on ability use")
	fmt.Println()

	fmt.Println("5. Proficiency Bonus:")
	fmt.Println("   profBonus := 2 + ((level - 1) / 4)")
	fmt.Println()

	fmt.Println("Migration Strategy:")
	fmt.Println("- Start small: Just spell slots")
	fmt.Println("- Test in parallel with existing system")
	fmt.Println("- Gradually replace components")
	fmt.Println("- Keep bot's structure intact")

	fmt.Println("\nSee integration_guide.md for detailed examples!")
}
