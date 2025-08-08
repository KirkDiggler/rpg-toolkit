package initiative_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
)

func Example() {
	// Game service has characters and monsters
	// Create participants with their DEX modifiers
	participants := map[core.Entity]int{
		initiative.NewParticipant("ranger-123", "character"):   +3,  // 16 DEX
		initiative.NewParticipant("wizard-456", "character"):   +1,  // 12 DEX  
		initiative.NewParticipant("goblin-001", "monster"):     +2,  // 14 DEX
		initiative.NewParticipant("goblin-002", "monster"):     +2,  // 14 DEX
	}
	
	// Roll initiative to get turn order
	order := initiative.RollForOrder(participants)
	
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
	
	// Output might be:
	// Round 1: ranger-123 (character) goes first
	// Round 1: goblin-002 (monster) goes next  
	// Round 1: goblin-001 (monster) goes next
	// Round 1: wizard-456 (character) goes next
}

func ExampleGameService() {
	// In your game service, when it's someone's turn:
	participants := map[core.Entity]int{
		initiative.NewParticipant("ranger-123", "character"): +3,
		initiative.NewParticipant("goblin-001", "monster"):   +2,
	}
	
	order := initiative.RollForOrder(participants)
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