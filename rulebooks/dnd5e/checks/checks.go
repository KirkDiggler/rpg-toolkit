// Package checks implements D&D 5e ability check mechanics.
// Mirrors rulebooks/dnd5e/saves: same roller + modifier + chain-sourced
// advantage/disadvantage/bonuses shape, applied to skill checks instead of
// saving throws.
//
// Every check consults the AbilityCheckChain — the bus is required, never
// defaulted (rpg-toolkit#1357). There is no bus-free entry point: for a real
// character nobody can prove no condition applies, so a check that skips the
// chain is a claim this package refuses to express.
package checks

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// AbilityCheckInput contains all parameters needed to make an ability check.
type AbilityCheckInput struct {
	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	// Pass a mock roller here for testing.
	Roller dice.Roller

	// EventBus is the event bus the AbilityCheckChain fires on, so that
	// conditions and features (guidance, inspiration, a blinded checker's
	// disadvantage) can modify the check. Required — an ability check
	// consults the chain, and no caller can prove no condition applies,
	// so nil is refused rather than quietly skipping every condition
	// (rpg-toolkit#1357).
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

	// D20Source is the canonical rule/content source that caused the d20 roll
	// — the skill or bare ability the check was rolled through. Its ref and
	// name say the RULE; the entity is CheckerID, which this package writes
	// onto the die itself (rpg-project#462 R7).
	D20Source dnd5eEvents.RollSource

	// ModifierSource is the canonical source for Modifier.
	ModifierSource dnd5eEvents.RollSource
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

	// BonusSources contains the sources that added bonuses to this check
	BonusSources []dnd5eEvents.CheckBonusSource

	// Calculation is the complete sourced arithmetic checked for this result:
	// the d20 pool, the flat modifier, and one component per chain-granted
	// bonus.
	//
	// BUILT HERE, IN THE RULES PACKAGE, the way saves already build theirs
	// (rpg-project#462 R3). It used to be assembled downstream in resolution
	// from the bare Roll, which could only fake a one-face trace for a pair
	// the rules package had actually rolled — and the fake said so in its own
	// comment. The d20 component's trace now carries every face, which one was
	// kept, and the Keep record naming the sources that granted or imposed;
	// that is why the result carries no AdvantageSources/DisadvantageSources
	// lists beside it (R1).
	Calculation *dnd5eEvents.RollCalculation
}

// MakeAbilityCheck executes an ability check: the AbilityCheckChain fires on
// the supplied bus so conditions and features can grant advantage, impose
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
// EventBus and CheckerID are required — supplied, never defaulted, refused
// loudly when absent. An ability check consults the chain, period: the day a
// condition subscribes, a bus-less call site would be a silent rules bug, so
// that call site cannot be written (rpg-toolkit#1357). The production caller
// is the resolution rung's check machine, which loads the character with
// conditions and fires the chain through resolution's lawful bus
// (rpg-project#351, ideas/living-world/concealed-door/design.md).
//
// If input.Roller is nil, a default CryptoRoller is used.
// Returns an error if the dice roller fails or chain execution fails.
func MakeAbilityCheck(ctx context.Context, input *AbilityCheckInput) (*AbilityCheckResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	if input.EventBus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"EventBus is required: an ability check consults the AbilityCheckChain, "+
				"and without the bus no condition can reach the roll")
	}
	if input.CheckerID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"CheckerID is required: chain subscribers key off the checker's id")
	}
	if err := validateCalculationSource("d20", input.D20Source); err != nil {
		return nil, err
	}
	if err := validateCalculationSource("modifier", input.ModifierSource); err != nil {
		return nil, err
	}

	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	var bonusSources []dnd5eEvents.CheckBonusSource

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

	// Collect modifiers from the chain. The advantage and disadvantage sources
	// go straight to the d20 roller as the rules that granted and imposed, so
	// the keep record can name them instead of a boolean losing them.
	granted := checkRollSources(result.AdvantageSources)
	imposed := checkRollSources(result.DisadvantageSources)
	bonusSources = append(bonusSources, result.BonusSources...)

	// ONE D20 ROLLER for the whole rulebook. This package used to carry its
	// own switch, and it was the one that threw the second face away: it
	// returned a single settled Roll, so an untrained character's two d20s
	// reached the log as one number and the word "Untrained" never got there
	// at all (rpg-project#462).
	d20Source := dnd5eEvents.CloneRollSource(input.D20Source)
	d20Source.SourceID = input.CheckerID
	d20, err := rolls.RollD20(ctx, &rolls.RollD20Input{
		Roller: roller, Source: d20Source, Granted: granted, Imposed: imposed,
	})
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to roll ability check d20")
	}
	roll := d20.Face

	modifier := input.Modifier
	components := make([]dnd5eEvents.RollComponent, 0, 2+len(bonusSources))
	components = append(components,
		dnd5eEvents.RollComponent{Source: d20Source, Dice: d20.Trace},
		dnd5eEvents.RollComponent{
			Source: dnd5eEvents.CloneRollSource(input.ModifierSource), Modifier: &modifier,
		},
	)
	for i, source := range bonusSources {
		rollSource := source.RollSource()
		if err := validateCalculationSource("bonus", rollSource); err != nil {
			return nil, rpgerr.Wrapf(err, "bonus source %d", i)
		}
		bonus := source.Bonus
		components = append(components, dnd5eEvents.RollComponent{Source: rollSource, Modifier: &bonus})
	}

	calculation := dnd5eEvents.NewRollCalculation(components)
	if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
		return nil, rpgerr.Wrap(err, "ability check calculation is invalid")
	}

	return &AbilityCheckResult{
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

// checkRollSources maps the chain's modifier sources onto the calculation's
// sourced-fact type. One function, so the keep record and the log cannot spell
// the same rule two ways.
func checkRollSources(sources []dnd5eEvents.CheckModifierSource) []dnd5eEvents.RollSource {
	if len(sources) == 0 {
		return nil
	}

	mapped := make([]dnd5eEvents.RollSource, len(sources))
	for i, source := range sources {
		mapped[i] = source.RollSource()
	}
	return mapped
}

// validateCalculationSource refuses a source that cannot carry a calculation
// component, before any die is rolled.
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
