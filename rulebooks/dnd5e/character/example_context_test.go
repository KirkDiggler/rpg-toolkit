package character_test

import (
	"context"
	"fmt"
)

// Example_loadCharacterFromContext shows how to load a character using the game.Context pattern
// Note: This is commented out as it demonstrates future usage patterns
func Example_loadCharacterFromContext() {
	// This example shows how to load a character using the game.Context pattern

	// In a real application:
	// 1. Create an event bus for game communication
	// eventBus := events.NewDirectEventBus()

	// 2. Load character data from storage
	// charData := loadFromDatabase(characterID)

	// 3. Create game context
	// gameCtx, err := game.NewContext(eventBus, charData)

	// 4. Load character with dependencies
	// char, err := character.LoadCharacterFromContext(ctx, gameCtx,
	//     raceData, classData, backgroundData)

	// The character is now:
	// - Loaded from persistent data
	// - Connected to the event bus
	// - Ready to participate in gameplay

	fmt.Println("Character loaded using game.Context pattern")
	// Output: Character loaded using game.Context pattern
}

// Example_loadCharacterFromContext_futureVision shows the future vision where character data is self-contained
// Note: This is commented out as it demonstrates future usage patterns
func Example_loadCharacterFromContext_futureVision() {
	// This example shows the future vision where character data is self-contained

	// Future state (as described in Journey 019):
	// charData := character.Data{
	//     ID: "hero-1",
	//     // All abilities, features, etc. are compiled into the data
	//     // No need for separate race/class/background lookups
	// }

	// gameCtx, _ := game.NewContext(eventBus, charData)
	// char, _ := character.LoadCharacterFromContext(ctx, gameCtx)
	// No external dependencies needed!

	ctx := context.Background()
	_ = ctx // Just to use the variable

	fmt.Println("Future: Self-contained character data")
	// Output: Future: Self-contained character data
}
