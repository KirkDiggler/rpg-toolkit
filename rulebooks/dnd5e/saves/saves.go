// Package saves implements D&D 5e saving throw mechanics
package saves

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// SavingThrowInput contains all parameters needed to make a saving throw
type SavingThrowInput struct {
	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	// Pass a mock roller here for testing.
	Roller dice.Roller

	// EventBus is the event bus for chain modifiers. If nil, no chain events are fired.
	// This allows conditions like Dodging to grant advantage on DEX saves.
	EventBus events.EventBus

	// SaverID is the ID of the entity making the saving throw.
	// Required when EventBus is provided.
	SaverID string

	// Cause provides context about what triggered this saving throw.
	// Used by conditions/features to determine if they should apply modifiers.
	Cause dnd5eEvents.SaveCause

	// Ability is the ability score being tested (STR, DEX, CON, INT, WIS, CHA)
	Ability abilities.Ability

	// DC is the Difficulty Class that must be met or exceeded
	DC int

	// Modifier is the total bonus/penalty to add to the roll
	// (typically ability modifier + proficiency bonus if proficient)
	Modifier int

	// HasAdvantage indicates rolling two d20s and taking the higher result
	HasAdvantage bool

	// HasDisadvantage indicates rolling two d20s and taking the lower result
	// Note: If both HasAdvantage and HasDisadvantage are true, they cancel out
	// and a single d20 is rolled (D&D 5e rule)
	HasDisadvantage bool
}

// SavingThrowResult contains the outcome of a saving throw
type SavingThrowResult struct {
	// Roll is the actual d20 roll result used (highest/lowest if advantage/disadvantage)
	Roll int

	// Total is the final value (Roll + Modifier + ChainBonuses)
	Total int

	// DC is the Difficulty Class that was tested against
	DC int

	// Success indicates whether the save succeeded (Total >= DC)
	Success bool

	// IsNat1 indicates if the d20 roll was a natural 1
	// Note: Unlike attack rolls, natural 1s don't automatically fail saving throws in D&D 5e
	IsNat1 bool

	// IsNat20 indicates if the d20 roll was a natural 20
	// Note: Unlike attack rolls, natural 20s don't automatically succeed saving throws in D&D 5e
	IsNat20 bool

	// AdvantageSources contains the sources that granted advantage on this save
	AdvantageSources []dnd5eEvents.SaveModifierSource

	// DisadvantageSources contains the sources that imposed disadvantage on this save
	DisadvantageSources []dnd5eEvents.SaveModifierSource

	// BonusSources contains the sources that added bonuses to this save
	BonusSources []dnd5eEvents.SaveBonusSource
}

// MakeSavingThrow executes a saving throw using the input parameters
//
// The function handles:
//   - Normal rolls (single d20)
//   - Advantage (roll 2d20, take higher)
//   - Disadvantage (roll 2d20, take lower)
//   - Advantage + Disadvantage cancellation (single d20)
//   - Natural 1 and natural 20 detection
//   - Chain event modifiers (advantage, disadvantage, bonuses from conditions/features)
//
// If input.Roller is nil, a default CryptoRoller is used.
// If input.EventBus is provided, the SavingThrowChain is fired to collect modifiers.
// Returns an error if the dice roller fails or chain execution fails.
func MakeSavingThrow(ctx context.Context, input *SavingThrowInput) (*SavingThrowResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}

	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	// Initialize modifier tracking from input
	hasAdvantage := input.HasAdvantage
	hasDisadvantage := input.HasDisadvantage
	bonusFromChain := 0
	var advantageSources []dnd5eEvents.SaveModifierSource
	var disadvantageSources []dnd5eEvents.SaveModifierSource
	var bonusSources []dnd5eEvents.SaveBonusSource

	// Fire chain event if EventBus is provided
	if input.EventBus != nil {
		chainEvent := &dnd5eEvents.SavingThrowChainEvent{
			SaverID: input.SaverID,
			Ability: input.Ability,
			DC:      input.DC,
			Cause:   input.Cause,
		}

		// Create chain and fire through subscribers
		saveChain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)
		chainTopic := dnd5eEvents.SavingThrowChain.On(input.EventBus)

		modifiedChain, err := chainTopic.PublishWithChain(ctx, chainEvent, saveChain)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to publish saving throw chain event")
		}

		// Execute chain to apply all modifiers
		result, err := modifiedChain.Execute(ctx, chainEvent)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to execute saving throw chain")
		}

		// Collect modifiers from chain
		if result.HasAdvantage() {
			hasAdvantage = true
			advantageSources = result.AdvantageSources
		}
		if result.HasDisadvantage() {
			hasDisadvantage = true
			disadvantageSources = result.DisadvantageSources
		}
		bonusFromChain = result.TotalBonus()
		bonusSources = result.BonusSources
	}

	var roll int
	var err error

	// D&D 5e Rule: Advantage and Disadvantage cancel each other out
	effectiveAdvantage := hasAdvantage && !hasDisadvantage
	effectiveDisadvantage := hasDisadvantage && !hasAdvantage

	switch {
	case effectiveAdvantage:
		// Roll with advantage: 2d20, take higher
		rolls, rollErr := roller.RollN(ctx, 2, 20)
		if rollErr != nil {
			return nil, rollErr
		}
		roll = max(rolls[0], rolls[1])
	case effectiveDisadvantage:
		// Roll with disadvantage: 2d20, take lower
		rolls, rollErr := roller.RollN(ctx, 2, 20)
		if rollErr != nil {
			return nil, rollErr
		}
		roll = min(rolls[0], rolls[1])
	default:
		// Normal roll: 1d20
		roll, err = roller.Roll(ctx, 20)
		if err != nil {
			return nil, err
		}
	}

	// Calculate total (base modifier + chain bonuses)
	total := roll + input.Modifier + bonusFromChain

	// Determine success
	success := total >= input.DC

	// Detect natural 1 and natural 20
	isNat1 := roll == 1
	isNat20 := roll == 20

	return &SavingThrowResult{
		Roll:                roll,
		Total:               total,
		DC:                  input.DC,
		Success:             success,
		IsNat1:              isNat1,
		IsNat20:             isNat20,
		AdvantageSources:    advantageSources,
		DisadvantageSources: disadvantageSources,
		BonusSources:        bonusSources,
	}, nil
}
