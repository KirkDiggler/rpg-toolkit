package initiative_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
)

func Example() {
	// For deterministic example output, create tracker with known order
	order := []core.Entity{
		initiative.NewParticipant("ranger-123", dnd5e.EntityTypeCharacter),
		initiative.NewParticipant("goblin-002", dnd5e.EntityTypeMonster),
		initiative.NewParticipant("goblin-001", dnd5e.EntityTypeMonster),
		initiative.NewParticipant("wizard-456", dnd5e.EntityTypeCharacter),
	}

	// Create tracker with that order
	tracker := initiative.New(order)

	// Start the encounter
	current := tracker.Current()
	fmt.Printf("Round %d: %s (%s) goes first\n",
		tracker.Round(), current.GetID(), current.GetType())

	// Move through turns
	for i := 0; i < 3; i++ {
		next := tracker.Next()
		fmt.Printf("Round %d: %s (%s) goes next\n",
			tracker.Round(), next.GetID(), next.GetType())
	}

	// Output:
	// Round 1: ranger-123 (character) goes first
	// Round 1: goblin-002 (monster) goes next
	// Round 1: goblin-001 (monster) goes next
	// Round 1: wizard-456 (character) goes next
}

func Example_gameService() {
	// In your game service, when it's someone's turn:

	// For deterministic example, create tracker with known order
	order := []core.Entity{
		initiative.NewParticipant("ranger-123", dnd5e.EntityTypeCharacter),
		initiative.NewParticipant("goblin-001", dnd5e.EntityTypeMonster),
	}
	tracker := initiative.New(order)

	// Get current turn
	entity := tracker.Current()

	// Now you can look up what this entity is
	switch entity.GetType() {
	case "character":
		// Load character from database
		fmt.Printf("Loading character %s from database\n", entity.GetID())
		// character := loadCharacter(entity.GetID())

	case "monster":
		// Load monster data
		fmt.Printf("Loading monster %s from database\n", entity.GetID())
		// monster := loadMonster(entity.GetID())
	}

	// Output:
	// Loading character ranger-123 from database
}
