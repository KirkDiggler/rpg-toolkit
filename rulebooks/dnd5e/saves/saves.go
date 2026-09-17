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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
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

	// D20Source is the canonical rule/content source that caused the d20 roll.
	D20Source dnd5eEvents.RollSource

	// ModifierSource is the canonical ability source for Modifier.
	ModifierSource dnd5eEvents.RollSource

	// Contributions are already selected by the saving creature's condition
	// owner. This evaluator applies no stacking or source-selection policy.
	//
	// ADVANTAGE ARRIVES ON THE CHAIN AND NOWHERE ELSE. This input used to
	// carry HasAdvantage/HasDisadvantage booleans for advantage "the caller
	// already knows about", recorded as a synthetic source named "Input" with
	// no ref and no entity. A keep record names the rules that met and the
	// entities that brought them (rpg-project#462 R7), which a boolean cannot
	// do — so the door is the SavingThrowChain, where every real source
	// already comes from.
	Contributions []dnd5eEvents.DiceContribution
}

// SavingThrowResult contains the outcome of a saving throw
type SavingThrowResult struct {
	// Roll is the actual d20 roll result used (highest/lowest if advantage/disadvantage)
	Roll int

	// Total is the final checked value after fixed bonuses and dice contributions.
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

	// BonusSources contains the sources that added bonuses to this save
	BonusSources []dnd5eEvents.SaveBonusSource

	// Calculation is the complete sourced arithmetic checked for this result.
	//
	// It is also where advantage and disadvantage are recorded: the d20
	// component's DiceTrace carries every face rolled, which one was kept, and
	// the Keep record naming the sources that granted or imposed it. The
	// result used to carry AdvantageSources/DisadvantageSources beside the
	// dice they described; a parallel list can disagree with its dice and
	// Keep cannot, so the list is gone rather than kept alongside
	// (rpg-project#462 R1).
	Calculation *dnd5eEvents.RollCalculation
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
	if err := validateCalculationSource("d20", input.D20Source); err != nil {
		return nil, err
	}
	if err := validateCalculationSource("modifier", input.ModifierSource); err != nil {
		return nil, err
	}
	if err := rolls.ValidateContributions(input.Contributions); err != nil {
		return nil, rpgerr.Wrap(err, "saving throw contributions are invalid")
	}

	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	var bonusSources []dnd5eEvents.SaveBonusSource

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

	// Collect modifiers from the chain. The advantage and disadvantage sources
	// go straight to the d20 roller as the rules that granted and imposed:
	// nothing in between collapses them into a pair of booleans, so the keep
	// record can name them.
	granted := saveRollSources(result.AdvantageSources)
	imposed := saveRollSources(result.DisadvantageSources)
	bonusSources = append(bonusSources, result.BonusSources...)

	bonusComponents := make([]dnd5eEvents.RollComponent, 0, len(bonusSources))
	for i, source := range bonusSources {
		rollSource := dnd5eEvents.CloneRollSource(source.RollSource())
		if err := validateCalculationSource("bonus", rollSource); err != nil {
			return nil, rpgerr.Wrapf(err, "bonus source %d", i)
		}
		bonus := source.Bonus
		bonusComponents = append(bonusComponents, dnd5eEvents.RollComponent{
			Source: rollSource, Modifier: &bonus,
		})
	}

	// ONE D20 ROLLER for the whole rulebook: it decides how many dice the
	// granted and imposed sources call for, which face counts, and what the
	// keep record says. This package no longer carries its own switch.
	//
	// The d20 is the SAVER'S die (R7): the source's ref and name say the rule
	// that caused the save, and its entity is the creature that rolled it, so
	// a client can draw the die in its owner's style without guessing from the
	// beat's actor.
	d20Source := dnd5eEvents.CloneRollSource(input.D20Source)
	d20Source.SourceID = input.SaverID
	d20, err := rolls.RollD20(ctx, &rolls.RollD20Input{
		Roller: roller, Source: d20Source, Granted: granted, Imposed: imposed,
	})
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to roll saving throw d20")
	}
	roll := d20.Face

	resolved, err := rolls.ResolveContributions(ctx, &rolls.ResolveContributionsInput{
		Roller: roller, Contributions: input.Contributions,
	})
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to resolve saving throw contributions")
	}

	modifier := input.Modifier
	components := make([]dnd5eEvents.RollComponent, 0, 2+len(bonusComponents)+len(resolved.Components))
	components = append(components,
		dnd5eEvents.RollComponent{Source: d20Source, Dice: d20.Trace},
		dnd5eEvents.RollComponent{Source: dnd5eEvents.CloneRollSource(input.ModifierSource), Modifier: &modifier},
	)
	components = append(components, bonusComponents...)
	components = append(components, resolved.Components...)
	calculation := dnd5eEvents.NewRollCalculation(components)
	if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
		return nil, rpgerr.Wrap(err, "saving throw calculation is invalid")
	}

	return &SavingThrowResult{
		Roll:         roll,
		Total:        calculation.Total,
		DC:           input.DC,
		Success:      calculation.Total >= input.DC,
		IsNat1:       roll == 1,
		IsNat20:      roll == 20,
		BonusSources: bonusSources,
		Calculation:  calculation,
	}, nil
}

// saveRollSources maps the chain's modifier sources onto the calculation's
// sourced-fact type, one to one: SourceRef->Ref, Name->Name, SourceType->Label,
// EntityID->SourceID. Whose rule and whose die are different facts and both
// survive the mapping.
func saveRollSources(sources []dnd5eEvents.SaveModifierSource) []dnd5eEvents.RollSource {
	if len(sources) == 0 {
		return nil
	}

	mapped := make([]dnd5eEvents.RollSource, len(sources))
	for i, source := range sources {
		mapped[i] = source.RollSource()
	}
	return mapped
}

func validateCalculationSource(kind string, source dnd5eEvents.RollSource) error {
	zero := 0
	calculation := &dnd5eEvents.RollCalculation{
		Components: []dnd5eEvents.RollComponent{{Source: source, Modifier: &zero}},
	}
	if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
		return rpgerr.Wrapf(err, "%s source is invalid", kind)
	}
	return nil
}
