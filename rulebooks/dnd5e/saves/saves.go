// Package saves implements D&D 5e saving throw mechanics.
//
// Every saving throw consults the SavingThrowChain — the bus is required,
// never defaulted (rpg-toolkit#1357). There is no bus-free entry point: for a
// real character nobody can prove no condition applies, so a save that skips
// the chain is a claim this package refuses to express.
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

	// EventBus is the event bus the SavingThrowChain fires on, so that
	// conditions like Dodging can grant advantage on DEX saves. Required —
	// a saving throw consults the chain, and no caller can prove no
	// condition applies, so nil is refused rather than quietly skipping
	// every condition (rpg-toolkit#1357).
	EventBus events.EventBus

	// SaverID is the ID of the entity making the saving throw.
	// Required — chain subscribers key off this id.
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

// MakeSavingThrow executes a saving throw: the SavingThrowChain fires on the
// supplied bus so conditions and features can grant advantage, impose
// disadvantage, or add bonuses, and the modified roll is scored against the DC.
//
// The function handles:
//   - Normal rolls (single d20)
//   - Advantage (roll 2d20, take higher)
//   - Disadvantage (roll 2d20, take lower)
//   - Advantage + Disadvantage cancellation (single d20)
//   - Natural 1 and natural 20 detection
//   - Chain event modifiers (advantage, disadvantage, bonuses from conditions/features)
//
// EventBus and SaverID are required — supplied, never defaulted, refused
// loudly when absent. A saving throw consults the chain, period: the day a
// condition subscribes, a bus-less call site would be a silent rules bug, so
// that call site cannot be written (rpg-toolkit#1357).
//
// If input.Roller is nil, a default CryptoRoller is used.
// Returns an error if the dice roller fails or chain execution fails.
func MakeSavingThrow(ctx context.Context, input *SavingThrowInput) (*SavingThrowResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	if input.EventBus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"EventBus is required: a saving throw consults the SavingThrowChain, "+
				"and without the bus no condition can reach the roll")
	}
	if input.SaverID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"SaverID is required: chain subscribers key off the saver's id")
	}

	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	// Initialize modifier tracking from input
	hasAdvantage := input.HasAdvantage
	hasDisadvantage := input.HasDisadvantage
	var advantageSources []dnd5eEvents.SaveModifierSource
	var disadvantageSources []dnd5eEvents.SaveModifierSource
	var bonusSources []dnd5eEvents.SaveBonusSource

	// Track input-provided advantage/disadvantage as sources for auditability
	if input.HasAdvantage {
		advantageSources = append(advantageSources, dnd5eEvents.SaveModifierSource{
			Name:       "Input",
			SourceType: "input",
		})
	}
	if input.HasDisadvantage {
		disadvantageSources = append(disadvantageSources, dnd5eEvents.SaveModifierSource{
			Name:       "Input",
			SourceType: "input",
		})
	}

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

	// Collect modifiers from chain (append to input sources)
	if result.HasAdvantage() {
		hasAdvantage = true
		advantageSources = append(advantageSources, result.AdvantageSources...)
	}
	if result.HasDisadvantage() {
		hasDisadvantage = true
		disadvantageSources = append(disadvantageSources, result.DisadvantageSources...)
	}
	bonusFromChain := result.TotalBonus()
	bonusSources = append(bonusSources, result.BonusSources...)

	var roll int

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

	return &SavingThrowResult{
		Roll:                roll,
		Total:               total,
		DC:                  input.DC,
		Success:             total >= input.DC,
		IsNat1:              roll == 1,
		IsNat20:             roll == 20,
		AdvantageSources:    advantageSources,
		DisadvantageSources: disadvantageSources,
		BonusSources:        bonusSources,
	}, nil
}
