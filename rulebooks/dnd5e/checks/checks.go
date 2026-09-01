// Package checks implements D&D 5e ability check mechanics.
// Mirrors rulebooks/dnd5e/saves: same roller + modifier + chain-sourced
// advantage/disadvantage/bonuses shape, applied to skill checks instead of
// saving throws.
//
// Two total functions, never one partial one (rpg-toolkit#1357):
//
//   - MakeAbilityCheck is the full check. It requires an event bus and fires
//     AbilityCheckChain so conditions and features can modify the roll.
//   - MakeUnaidedAbilityCheck consults no conditions. It has no bus
//     parameter, so nothing is silently skipped because nothing was promised.
package checks

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// AbilityCheckInput contains all parameters needed to make a full ability check.
type AbilityCheckInput struct {
	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	// Pass a mock roller here for testing.
	Roller dice.Roller

	// EventBus is the event bus the AbilityCheckChain fires on, so that
	// conditions and features (guidance, inspiration, a blinded checker's
	// disadvantage) can modify the check. Required — a full ability check
	// consults the chain. A caller with no bus makes that choice by name
	// with MakeUnaidedAbilityCheck instead of passing nil here.
	EventBus events.EventBus

	// CheckerID is the ID of the entity making the check.
	// Required — chain subscribers key off this id.
	CheckerID string

	// Skill is the skill being checked (Stealth, Perception, etc).
	Skill skills.Skill

	// DC is the Difficulty Class that must be met or exceeded (e.g. the
	// highest observer passive Perception for a Hide check).
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

// UnaidedAbilityCheckInput contains all parameters needed to make an unaided
// ability check: roll, modifier, advantage/disadvantage, DC. There is no
// EventBus and no CheckerID because no chain fires — see
// MakeUnaidedAbilityCheck.
type UnaidedAbilityCheckInput struct {
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

// AbilityCheckResult contains the outcome of an ability check.
type AbilityCheckResult struct {
	// Roll is the actual d20 roll result used (highest/lowest if advantage/disadvantage)
	Roll int

	// Total is the final value (Roll + Modifier + ChainBonuses)
	Total int

	// DC is the Difficulty Class that was tested against
	DC int

	// Success indicates whether the check succeeded (Total >= DC)
	Success bool

	// IsNat1 indicates if the d20 roll was a natural 1
	IsNat1 bool

	// IsNat20 indicates if the d20 roll was a natural 20
	IsNat20 bool

	// AdvantageSources contains the sources that granted advantage on this check
	AdvantageSources []dnd5eEvents.CheckModifierSource

	// DisadvantageSources contains the sources that imposed disadvantage on this check
	DisadvantageSources []dnd5eEvents.CheckModifierSource

	// BonusSources contains the sources that added bonuses to this check
	BonusSources []dnd5eEvents.CheckBonusSource
}

// MakeAbilityCheck executes a full ability check: the AbilityCheckChain fires
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
// EventBus and CheckerID are required — supplied, never defaulted. A full
// ability check consults the chain; a nil bus here is refused rather than
// quietly skipping every condition (rpg-toolkit#1357). A caller that has no
// bus wants MakeUnaidedAbilityCheck, which states that choice by name.
//
// If input.Roller is nil, a default CryptoRoller is used.
// Returns an error if the dice roller fails or chain execution fails.
func MakeAbilityCheck(ctx context.Context, input *AbilityCheckInput) (*AbilityCheckResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	if input.EventBus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"EventBus is required: a full ability check consults the AbilityCheckChain — "+
				"a caller with no bus rolls unaided, by name, with MakeUnaidedAbilityCheck")
	}
	if input.CheckerID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"CheckerID is required: chain subscribers key off the checker's id")
	}

	tally := newCheckTally(input.HasAdvantage, input.HasDisadvantage)

	chainEvent := &dnd5eEvents.AbilityCheckChainEvent{
		CheckerID: input.CheckerID,
		Skill:     input.Skill,
		DC:        input.DC,
	}

	// Create chain and fire through subscribers
	checkChain := events.NewStagedChain[*dnd5eEvents.AbilityCheckChainEvent](combat.ModifierStages)
	chainTopic := dnd5eEvents.AbilityCheckChain.On(input.EventBus)

	modifiedChain, err := chainTopic.PublishWithChain(ctx, chainEvent, checkChain)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to publish ability check chain event")
	}

	// Execute chain to apply all modifiers
	result, err := modifiedChain.Execute(ctx, chainEvent)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to execute ability check chain")
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

	return rollCheck(ctx, input.Roller, input.DC, input.Modifier, tally)
}

// MakeUnaidedAbilityCheck executes an ability check that consults no
// conditions: d20 with advantage/disadvantage cancellation, plus the
// caller-supplied modifier, against the DC, with natural 1/20 detection —
// and nothing else. It has no bus parameter, so no chain fires and none is
// promised: the absence of condition modifiers is this function's stated
// contract, never a silent degradation (rpg-toolkit#1357).
//
// A caller holding a bus wants MakeAbilityCheck, the full check. Bus-free
// callers — the session seam is bus-free by structural pin — roll unaided
// here; their checks meet conditions at the resolution rung, the layer that
// owns interaction machinery (rpg-project#351,
// ideas/living-world/concealed-door/design.md, "the resolver and the
// no-bus law").
//
// If input.Roller is nil, a default CryptoRoller is used.
// Returns an error if the dice roller fails.
func MakeUnaidedAbilityCheck(ctx context.Context, input *UnaidedAbilityCheckInput) (*AbilityCheckResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}

	tally := newCheckTally(input.HasAdvantage, input.HasDisadvantage)

	return rollCheck(ctx, input.Roller, input.DC, input.Modifier, tally)
}

// checkTally is what a check has gathered before the die is rolled: the
// effective advantage/disadvantage flags, the chain bonus, and the sources
// behind each. The full check merges chain output into it; the unaided check
// carries only the caller's own flags.
type checkTally struct {
	hasAdvantage        bool
	hasDisadvantage     bool
	bonus               int
	advantageSources    []dnd5eEvents.CheckModifierSource
	disadvantageSources []dnd5eEvents.CheckModifierSource
	bonusSources        []dnd5eEvents.CheckBonusSource
}

// newCheckTally seeds a tally from caller-supplied advantage/disadvantage,
// tracking each as an "Input" source for auditability.
func newCheckTally(hasAdvantage, hasDisadvantage bool) checkTally {
	tally := checkTally{
		hasAdvantage:    hasAdvantage,
		hasDisadvantage: hasDisadvantage,
	}
	if hasAdvantage {
		tally.advantageSources = append(tally.advantageSources, dnd5eEvents.CheckModifierSource{
			Name:       "Input",
			SourceType: "input",
		})
	}
	if hasDisadvantage {
		tally.disadvantageSources = append(tally.disadvantageSources, dnd5eEvents.CheckModifierSource{
			Name:       "Input",
			SourceType: "input",
		})
	}
	return tally
}

// rollCheck is the one implementation of the roll arithmetic both public
// functions share: advantage/disadvantage cancellation, the d20, the total
// against the DC, natural 1/20 detection. The unaided check is the full
// check minus the chain, structurally, not a copy.
func rollCheck(
	ctx context.Context, roller dice.Roller, dc, modifier int, tally checkTally,
) (*AbilityCheckResult, error) {
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

	return &AbilityCheckResult{
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
