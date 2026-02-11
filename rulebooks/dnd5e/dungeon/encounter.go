package dungeon

import (
	"context"
	"math/rand"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
)

// encounterAllocation holds the CR budget distribution across rooms.
// Boss room receives ~40% of total budget, remaining rooms share 60%.
//
//nolint:unused // Used by generator.go (session 2026-01-11-005)
type encounterAllocation struct {
	// RoomBudgets maps room IDs to their allocated CR budget
	RoomBudgets map[string]float64
	// BossRoomID identifies which room contains the boss encounter
	BossRoomID string
}

// allocateBudgetInput configures budget allocation across rooms.
//
//nolint:unused // Used by generator.go (session 2026-01-11-005)
type allocateBudgetInput struct {
	// RoomIDs lists all rooms that need encounters
	RoomIDs []string
	// BossRoomID identifies the boss room (gets ~40% of budget)
	BossRoomID string
	// TargetCR is the total challenge rating budget for the dungeon
	TargetCR float64
	// DifficultyRamp makes earlier rooms easier, later rooms harder
	DifficultyRamp bool
}

// allocateBudget distributes the total CR budget across rooms.
//
// Budget distribution:
// - Boss room gets ~40% of total budget
// - Remaining 60% distributed across other rooms
// - If DifficultyRamp is true, earlier rooms get less CR, later rooms get more
// - Minimum CR 0.25 per room (at least one weak monster possible)
//
//nolint:unused // Used by generator.go (session 2026-01-11-005)
func allocateBudget(input *allocateBudgetInput) *encounterAllocation {
	if input == nil || len(input.RoomIDs) == 0 {
		return &encounterAllocation{
			RoomBudgets: make(map[string]float64),
		}
	}

	allocation := &encounterAllocation{
		RoomBudgets: make(map[string]float64),
		BossRoomID:  input.BossRoomID,
	}

	// Single room case: give it the full budget
	if len(input.RoomIDs) == 1 {
		allocation.RoomBudgets[input.RoomIDs[0]] = input.TargetCR
		return allocation
	}

	// Constants for budget distribution
	const (
		bossRoomPercent = 0.40 // Boss room gets 40% of budget
		minRoomCR       = 0.25 // Minimum CR per room
	)

	// Calculate boss room budget
	bossRoomBudget := input.TargetCR * bossRoomPercent
	if bossRoomBudget < minRoomCR {
		bossRoomBudget = minRoomCR
	}

	// Remaining budget for non-boss rooms
	remainingBudget := input.TargetCR - bossRoomBudget
	nonBossRooms := make([]string, 0, len(input.RoomIDs)-1)
	for _, roomID := range input.RoomIDs {
		if roomID != input.BossRoomID {
			nonBossRooms = append(nonBossRooms, roomID)
		}
	}

	// If no non-boss rooms, give everything to boss
	if len(nonBossRooms) == 0 {
		allocation.RoomBudgets[input.BossRoomID] = input.TargetCR
		return allocation
	}

	// Set boss room budget
	allocation.RoomBudgets[input.BossRoomID] = bossRoomBudget

	// Distribute remaining budget across non-boss rooms
	if input.DifficultyRamp {
		// Progressive difficulty: earlier rooms easier, later rooms harder
		// Use triangular distribution where room i gets weight (i+1)
		totalWeight := 0.0
		for i := range nonBossRooms {
			totalWeight += float64(i + 1)
		}

		for i, roomID := range nonBossRooms {
			weight := float64(i + 1)
			roomBudget := remainingBudget * (weight / totalWeight)
			if roomBudget < minRoomCR {
				roomBudget = minRoomCR
			}
			allocation.RoomBudgets[roomID] = roomBudget
		}
	} else {
		// Even distribution across non-boss rooms
		evenBudget := remainingBudget / float64(len(nonBossRooms))
		if evenBudget < minRoomCR {
			evenBudget = minRoomCR
		}

		for _, roomID := range nonBossRooms {
			allocation.RoomBudgets[roomID] = evenBudget
		}
	}

	return allocation
}

// generateEncounterInput configures encounter generation for a single room.
//
//nolint:unused // Used by generator.go (session 2026-01-11-005)
type generateEncounterInput struct {
	// Budget is the CR budget to fill for this room
	Budget float64
	// MonsterPool is the weighted selection table for regular monsters
	MonsterPool selectables.SelectionTable[MonsterRef]
	// IsBossRoom indicates whether this is the boss room
	IsBossRoom bool
	// BossPool is the weighted selection table for boss monsters
	BossPool selectables.SelectionTable[MonsterRef]
	// Seed for reproducible random generation
	Seed int64
}

// seededRoller implements dice.Roller using math/rand for reproducible generation.
// Used internally for encounter generation with deterministic seeding.
//
//nolint:unused // Used by newSeededRoller
type seededRoller struct {
	// #nosec G404 - Using math/rand for seeded, reproducible procedural generation
	rng *rand.Rand
}

// newSeededRoller creates a new seeded roller for reproducible random generation.
//
//nolint:unused // Used by generateEncounter
func newSeededRoller(seed int64) dice.Roller {
	// #nosec G404 - Using math/rand for seeded, reproducible procedural generation
	return &seededRoller{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// Roll returns a random number from 1 to size (inclusive).
//
//nolint:unused // Implements dice.Roller interface
func (s *seededRoller) Roll(_ context.Context, size int) (int, error) {
	if size <= 0 {
		return 0, nil
	}
	return s.rng.Intn(size) + 1, nil
}

// RollN rolls count dice of the given size.
//
//nolint:unused // Implements dice.Roller interface
func (s *seededRoller) RollN(ctx context.Context, count, size int) ([]int, error) {
	if size <= 0 || count < 0 {
		return nil, nil
	}
	results := make([]int, count)
	for i := 0; i < count; i++ {
		result, _ := s.Roll(ctx, size)
		results[i] = result
	}
	return results, nil
}

// generateEncounter selects monsters to fill a room's CR budget.
//
// Encounter logic:
// - If boss room: select one boss from BossPool, fill remaining budget with minions
// - Otherwise: select monsters from MonsterPool until budget is filled
// - Don't exceed budget by more than 25% (allow slight overage for balance)
//
//nolint:unused // Used by generator.go (session 2026-01-11-005)
func generateEncounter(input *generateEncounterInput) *EncounterData {
	if input == nil || input.Budget <= 0 {
		return &EncounterData{
			Monsters: []MonsterPlacementData{},
			TotalCR:  0,
		}
	}

	encounter := &EncounterData{
		Monsters: make([]MonsterPlacementData, 0),
		TotalCR:  0,
	}

	// Create a seeded context for reproducible selection
	roller := newSeededRoller(input.Seed)
	ctx := selectables.NewSelectionContextWithRoller(roller)

	// Calculate maximum allowed budget (25% overage allowed)
	maxBudget := input.Budget * 1.25

	if input.IsBossRoom && input.BossPool != nil && !input.BossPool.IsEmpty() {
		// Boss room: select one boss, then fill with minions
		bossRef, err := input.BossPool.Select(ctx)
		if err == nil {
			encounter.Monsters = append(encounter.Monsters, MonsterPlacementData{
				ID:        generateMonsterID(len(encounter.Monsters)),
				MonsterID: bossRef.Ref.ID,
				Role:      bossRef.Role,
				CR:        bossRef.CR,
			})
			encounter.TotalCR += bossRef.CR
		}

		// Fill remaining budget with minions from monster pool
		if input.MonsterPool != nil && !input.MonsterPool.IsEmpty() {
			fillBudgetWithMonsters(encounter, input.MonsterPool, ctx, maxBudget)
		}
	} else if input.MonsterPool != nil && !input.MonsterPool.IsEmpty() {
		// Regular room: fill budget with monsters
		fillBudgetWithMonsters(encounter, input.MonsterPool, ctx, maxBudget)
	}

	return encounter
}

// fillBudgetWithMonsters adds monsters to the encounter until budget is filled.
// It stops when adding another monster would exceed maxBudget.
//
//nolint:unused // Used by generateEncounter
func fillBudgetWithMonsters(
	encounter *EncounterData,
	monsterPool selectables.SelectionTable[MonsterRef],
	ctx selectables.SelectionContext,
	maxBudget float64,
) {
	// Maximum iterations to prevent infinite loops
	const maxIterations = 100

	for i := 0; i < maxIterations && encounter.TotalCR < maxBudget; i++ {
		monsterRef, err := monsterPool.Select(ctx)
		if err != nil {
			break
		}

		// Check if adding this monster would exceed the max budget
		if encounter.TotalCR+monsterRef.CR > maxBudget {
			// Try to find a smaller monster that fits
			// For simplicity, we just stop here - future improvement could retry
			break
		}

		encounter.Monsters = append(encounter.Monsters, MonsterPlacementData{
			ID:        generateMonsterID(len(encounter.Monsters)),
			MonsterID: monsterRef.Ref.ID,
			Role:      monsterRef.Role,
			CR:        monsterRef.CR,
		})
		encounter.TotalCR += monsterRef.CR
	}
}

// generateMonsterID creates a unique ID for a monster placement.
//
//nolint:unused // Used by generateEncounter and fillBudgetWithMonsters
func generateMonsterID(index int) string {
	return "monster_" + itoa(index)
}

// itoa is a simple int to string conversion.
//
//nolint:unused // Used by generateMonsterID
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
