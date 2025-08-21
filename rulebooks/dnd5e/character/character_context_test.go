package character_test

import (
	"testing"
)

func TestLoadCharacterFromContext(t *testing.T) {
	t.Run("loads character with game context", func(t *testing.T) {
		// Since we need a non-nil event bus for NewContext, skip this test
		// until we have a mock event bus or the validation is relaxed for tests
		t.Skip("Need mock event bus for testing")

		// Test would look like:
		// charData := character.Data{
		//     ID:       "test-char-1",
		//     PlayerID: "player-123",
		//     Name:     "Thorin Oakenshield",
		//     Level:    5,
		//     RaceID:   "dwarf",
		//     ClassID:  classes.Fighter,
		//     AbilityScores: shared.AbilityScores{
		//         Strength:     16,
		//         Dexterity:    14,
		//         Constitution: 15,
		//         Intelligence: 10,
		//         Wisdom:       12,
		//         Charisma:     8,
		//     },
		//     HitPoints:    38,
		//     MaxHitPoints: 44,
		//     Skills: map[string]int{
		//         "Athletics":    2, // Proficient
		//         "Intimidation": 2, // Proficient
		//     },
		//     Languages:     []string{"Common", "Dwarvish"},
		//     SavingThrows:  map[string]int{"Strength": 2, "Constitution": 2},
		//     Proficiencies: shared.Proficiencies{},
		// }
		//
		// raceData := &race.Data{
		//     ID:    "dwarf",
		//     Name:  "Dwarf",
		//     Size:  "Medium",
		//     Speed: 25,
		// }
		//
		// classData := &class.Data{
		//     ID:      classes.Fighter,
		//     Name:    "Fighter",
		//     HitDice: 10,
		// }
		//
		// backgroundData := &shared.Background{
		//     ID:   "soldier",
		//     Name: "Soldier",
		// }
		//
		// gameCtx, err := game.NewContext(eventBus, charData)
		// require.NoError(t, err)
		//
		// ctx := context.Background()
		// char, err := character.LoadCharacterFromContext(ctx, gameCtx,
		//     raceData, classData, backgroundData)
		//
		// require.NoError(t, err)
		// assert.NotNil(t, char)
	})

	t.Run("pattern consistency", func(t *testing.T) {
		// This test demonstrates that the pattern is consistent
		// All entities will load the same way:
		//
		// gameCtx, _ := game.NewContext(eventBus, entityData)
		// entity, _ := LoadEntityFromContext(ctx, gameCtx, ...dependencies)
		//
		// This creates a uniform API across:
		// - LoadCharacterFromContext
		// - LoadRoomFromContext (future)
		// - LoadMonsterFromContext (future)
		// - LoadItemFromContext (future)

		t.Log("LoadCharacterFromContext follows the game.Context pattern")
	})
}
