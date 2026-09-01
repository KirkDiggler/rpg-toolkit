// Package saves implements D&D 5e saving throw mechanics.
//
// Two total functions, never one partial one (rpg-toolkit#1357):
//
//   - MakeSavingThrow is the full save. It requires an event bus and fires
//     SavingThrowChain so conditions and features can modify the roll.
//   - MakeUnaidedSavingThrow consults no conditions. It has no bus
//     parameter, so nothing is silently skipped because nothing was promised.
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

// SavingThrowInput contains all parameters needed to make a full saving throw
type SavingThrowInput struct {
	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	// Pass a mock roller here for testing.
	Roller dice.Roller

	// EventBus is the event bus the SavingThrowChain fires on, so that
	// conditions like Dodging can grant advantage on DEX saves. Required —
	// a full saving throw consults the chain. A caller with no bus makes
	// that choice by name with MakeUnaidedSavingThrow instead of passing
	// nil here.
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

// UnaidedSavingThrowInput contains all parameters needed to make an unaided
// saving throw: roll, modifier, advantage/disadvantage, DC. There is no
// EventBus and no SaverID because no chain fires — see MakeUnaidedSavingThrow.
type UnaidedSavingThrowInput struct {
	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	// Pass a mock roller here for testing.
	Roller dice.Roller

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

// MakeSavingThrow executes a full saving throw: the SavingThrowChain fires
// on the supplied bus so conditions and features can grant advantage, impose
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
// EventBus and SaverID are required — supplied, never defaulted. A full
// saving throw consults the chain; a nil bus here is refused rather than
// quietly skipping every condition (rpg-toolkit#1357). A caller that has no
// bus wants MakeUnaidedSavingThrow, which states that choice by name.
//
// If input.Roller is nil, a default CryptoRoller is used.
// Returns an error if the dice roller fails or chain execution fails.
func MakeSavingThrow(ctx context.Context, input *SavingThrowInput) (*SavingThrowResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	if input.EventBus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"EventBus is required: a full saving throw consults the SavingThrowChain — "+
				"a caller with no bus rolls unaided, by name, with MakeUnaidedSavingThrow")
	}
	if input.SaverID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"SaverID is required: chain subscribers key off the saver's id")
	}

	tally := newSaveTally(input.HasAdvantage, input.HasDisadvantage)

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
		tally.hasAdvantage = true
		tally.advantageSources = append(tally.advantageSources, result.AdvantageSources...)
	}
	if result.HasDisadvantage() {
		tally.hasDisadvantage = true
		tally.disadvantageSources = append(tally.disadvantageSources, result.DisadvantageSources...)
	}
	tally.bonus = result.TotalBonus()
	tally.bonusSources = append(tally.bonusSources, result.BonusSources...)

	return rollSave(ctx, input.Roller, input.DC, input.Modifier, tally)
}

// MakeUnaidedSavingThrow executes a saving throw that consults no conditions:
// d20 with advantage/disadvantage cancellation, plus the caller-supplied
// modifier, against the DC, with natural 1/20 detection — and nothing else.
// It has no bus parameter, so no chain fires and none is promised: the
// absence of condition modifiers is this function's stated contract, never a
// silent degradation (rpg-toolkit#1357).
//
// A caller holding a bus wants MakeSavingThrow, the full save. A caller that
// folds the saving-throw chain on its own bus (the resolution module's save
// machine) hands in what the fold settled on and takes back the arithmetic
// here — the same fold-outside shape as combat.FinalDamage. Checks made at
// bus-free seams meet conditions at the resolution rung, the layer that owns
// interaction machinery (rpg-project#351,
// ideas/living-world/concealed-door/design.md, "the resolver and the
// no-bus law").
//
// If input.Roller is nil, a default CryptoRoller is used.
// Returns an error if the dice roller fails.
func MakeUnaidedSavingThrow(ctx context.Context, input *UnaidedSavingThrowInput) (*SavingThrowResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}

	tally := newSaveTally(input.HasAdvantage, input.HasDisadvantage)

	return rollSave(ctx, input.Roller, input.DC, input.Modifier, tally)
}

// saveTally is what a save has gathered before the die is rolled: the
// effective advantage/disadvantage flags, the chain bonus, and the sources
// behind each. The full save merges chain output into it; the unaided save
// carries only the caller's own flags.
type saveTally struct {
	hasAdvantage        bool
	hasDisadvantage     bool
	bonus               int
	advantageSources    []dnd5eEvents.SaveModifierSource
	disadvantageSources []dnd5eEvents.SaveModifierSource
	bonusSources        []dnd5eEvents.SaveBonusSource
}

// newSaveTally seeds a tally from caller-supplied advantage/disadvantage,
// tracking each as an "Input" source for auditability.
func newSaveTally(hasAdvantage, hasDisadvantage bool) saveTally {
	tally := saveTally{
		hasAdvantage:    hasAdvantage,
		hasDisadvantage: hasDisadvantage,
	}
	if hasAdvantage {
		tally.advantageSources = append(tally.advantageSources, dnd5eEvents.SaveModifierSource{
			Name:       "Input",
			SourceType: "input",
		})
	}
	if hasDisadvantage {
		tally.disadvantageSources = append(tally.disadvantageSources, dnd5eEvents.SaveModifierSource{
			Name:       "Input",
			SourceType: "input",
		})
	}
	return tally
}

// rollSave is the one implementation of the roll arithmetic both public
// functions share: advantage/disadvantage cancellation, the d20, the total
// against the DC, natural 1/20 detection. The unaided save is the full save
// minus the chain, structurally, not a copy.
func rollSave(
	ctx context.Context, roller dice.Roller, dc, modifier int, tally saveTally,
) (*SavingThrowResult, error) {
	if roller == nil {
		roller = dice.NewRoller()
	}

	var roll int
	var err error

	// D&D 5e Rule: Advantage and Disadvantage cancel each other out
	effectiveAdvantage := tally.hasAdvantage && !tally.hasDisadvantage
	effectiveDisadvantage := tally.hasDisadvantage && !tally.hasAdvantage

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
	total := roll + modifier + tally.bonus

	return &SavingThrowResult{
		Roll:                roll,
		Total:               total,
		DC:                  dc,
		Success:             total >= dc,
		IsNat1:              roll == 1,
		IsNat20:             roll == 20,
		AdvantageSources:    tally.advantageSources,
		DisadvantageSources: tally.disadvantageSources,
		BonusSources:        tally.bonusSources,
	}, nil
}
