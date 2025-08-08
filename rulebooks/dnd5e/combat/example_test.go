package combat_test

import (
	"context"
	"fmt"
	"log"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/game"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// ExampleCharacter represents a D&D 5e character for combat
type ExampleCharacter struct {
	id       string
	name     string
	dexMod   int
	dexScore int
	ac       int
	hp       int
	maxHP    int
}

func (c *ExampleCharacter) GetID() string             { return c.id }
func (c *ExampleCharacter) GetType() string           { return "character" }
func (c *ExampleCharacter) GetDexterityModifier() int { return c.dexMod }
func (c *ExampleCharacter) GetDexterityScore() int    { return c.dexScore }
func (c *ExampleCharacter) GetArmorClass() int        { return c.ac }
func (c *ExampleCharacter) GetHitPoints() int         { return c.hp }
func (c *ExampleCharacter) GetMaxHitPoints() int      { return c.maxHP }
func (c *ExampleCharacter) IsConscious() bool         { return c.hp > 0 }
func (c *ExampleCharacter) IsDefeated() bool          { return c.hp <= 0 }

// Example demonstrates the complete D&D 5e initiative system
func ExampleCombatState() {
	// Create event bus for combat events
	eventBus := events.NewBus()

	// Subscribe to combat events for logging
	eventBus.SubscribeFunc(combat.EventCombatStarted, 100, func(_ context.Context, e events.Event) error {
		fmt.Println("⚔️  Combat has begun!")
		return nil
	})

	eventBus.SubscribeFunc(combat.EventInitiativeRolled, 100, func(_ context.Context, e events.Event) error {
		if data, ok := e.Context().Get("data"); ok {
			if initData, ok := data.(combat.InitiativeRolledData); ok {
				fmt.Printf("🎲 %s rolled initiative: %d (d20: %d + mod: %d)\n",
					initData.Entity.GetID(), initData.Total, initData.Roll, initData.Modifier)
			}
		}
		return nil
	})

	eventBus.SubscribeFunc(combat.EventTurnStarted, 100, func(_ context.Context, e events.Event) error {
		if data, ok := e.Context().Get("data"); ok {
			if turnData, ok := data.(combat.TurnStartedData); ok {
				fmt.Printf("👤 %s's turn (Round %d, Initiative: %d)\n",
					turnData.Entity.GetID(), turnData.Round, turnData.Initiative)
			}
		}
		return nil
	})

	// Create combat encounter
	combatState := combat.NewCombatState(combat.CombatStateConfig{
		ID:       "tavern-brawl-001",
		Name:     "Tavern Brawl",
		EventBus: eventBus,
		Settings: combat.CombatSettings{
			InitiativeRollMode: combat.InitiativeRollModeRoll,
			TieBreakingMode:    combat.TieBreakingModeDexterity,
		},
		Roller: dice.DefaultRoller,
	})

	// Create party members
	fighter := &ExampleCharacter{
		id:       "korrath-fighter",
		name:     "Korrath the Bold",
		dexMod:   1,
		dexScore: 13,
		ac:       18,
		hp:       58,
		maxHP:    58,
	}

	rogue := &ExampleCharacter{
		id:       "silara-rogue",
		name:     "Silara Shadowstep",
		dexMod:   4,
		dexScore: 18,
		ac:       15,
		hp:       42,
		maxHP:    42,
	}

	wizard := &ExampleCharacter{
		id:       "eldrin-wizard",
		name:     "Eldrin Starweaver",
		dexMod:   2,
		dexScore: 14,
		ac:       12,
		hp:       28,
		maxHP:    28,
	}

	// Add combatants to the encounter
	combatants := []combat.Combatant{fighter, rogue, wizard}
	for _, combatant := range combatants {
		if err := combatState.AddCombatant(combatant); err != nil {
			log.Fatalf("Failed to add combatant: %v", err)
		}
	}

	fmt.Println("🏰 Setting up Tavern Brawl encounter...")
	fmt.Printf("   Combatants: %s, %s, %s\n\n",
		fighter.name, rogue.name, wizard.name)

	// Roll initiative
	fmt.Println("🎲 Rolling initiative...")
	rollInput := &combat.RollInitiativeInput{
		Combatants: combatants,
		RollMode:   combat.InitiativeRollModeRoll,
	}

	rollOutput, err := combatState.RollInitiative(rollInput)
	if err != nil {
		log.Fatalf("Failed to roll initiative: %v", err)
	}

	// Show initiative order
	fmt.Println("\n📋 Initiative Order:")
	for i, entry := range rollOutput.InitiativeEntries {
		fmt.Printf("   %d. %s - %d (DEX: %d)\n",
			i+1, entry.EntityID, entry.Total, entry.DexterityScore)
	}

	// Handle any ties
	if len(rollOutput.UnresolvedTies) > 0 {
		fmt.Printf("\n⚖️  Resolving %d tie(s)...\n", len(rollOutput.UnresolvedTies))

		tieInput := &combat.ResolveTiesInput{
			TiedGroups:        rollOutput.UnresolvedTies,
			InitiativeEntries: rollOutput.InitiativeEntries,
			TieBreakingMode:   combat.TieBreakingModeDexterity,
		}

		tieOutput, err := combatState.ResolveTies(tieInput)
		if err != nil {
			log.Fatalf("Failed to resolve ties: %v", err)
		}

		if len(tieOutput.RemainingTies) == 0 {
			fmt.Println("   ✅ All ties resolved!")
		}
	}

	// Start combat
	fmt.Println("\n⚔️  Starting combat...")
	if err := combatState.StartCombat(); err != nil {
		log.Fatalf("Failed to start combat: %v", err)
	}

	// Demonstrate turn progression
	fmt.Println("\n🔄 Turn progression:")
	for i := 0; i < 6; i++ { // Show first few turns
		_, err := combatState.GetCurrentTurn()
		if err != nil {
			log.Fatalf("Failed to get current turn: %v", err)
		}

		if i > 0 { // Don't advance before first turn
			if err := combatState.NextTurn(); err != nil {
				log.Fatalf("Failed to advance turn: %v", err)
			}
		}

		if i == 3 {
			fmt.Println("   ... (combat continues)")
			break
		}
	}

	// Demonstrate persistence
	fmt.Println("\n💾 Demonstrating persistence...")

	// Convert to data for saving
	combatData := combatState.ToData()
	fmt.Printf("   Combat state serialized (ID: %s, Status: %s, Round: %d)\n",
		combatData.ID, combatData.Status, combatData.Round)

	// Load from context (simulate loading from database)
	newEventBus := events.NewBus()
	gameCtx, err := game.NewContext(newEventBus, combatData)
	if err != nil {
		log.Fatalf("Failed to create game context: %v", err)
	}

	loadedCombat, err := combat.LoadCombatStateFromContext(context.Background(), gameCtx)
	if err != nil {
		log.Fatalf("Failed to load combat from context: %v", err)
	}

	fmt.Printf("   Combat state loaded successfully (Round: %d)\n", loadedCombat.GetRound())

	// Output will vary due to dice rolls, but demonstrates:
	// - Combat setup with multiple combatants
	// - Initiative rolling with modifiers
	// - Turn order based on initiative
	// - Combat progression through rounds
	// - State persistence and loading
}
