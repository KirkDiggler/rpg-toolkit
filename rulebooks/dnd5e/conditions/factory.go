// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// CreateFromRefInput provides input for creating a condition from a ref string
type CreateFromRefInput struct {
	// Ref is the condition reference in "module:type:value" format
	// e.g., "dnd5e:conditions:unarmored_defense"
	Ref string
	// Config is condition-specific configuration as JSON
	Config json.RawMessage
	// MemberID is the ID of the character this condition applies to
	MemberID string
	// SourceRef is the ref of what granted this condition in "module:type:value" format
	// e.g., "dnd5e:classes:barbarian" for class-granted conditions
	// e.g., "dnd5e:features:rage" for feature-activated conditions
	SourceRef string
}

// CreateFromRefOutput provides the result of creating a condition from a ref
type CreateFromRefOutput struct {
	// Condition is the created condition
	Condition dnd5eEvents.ConditionBehavior
}

// CreateFromRef creates a condition from a ref string and configuration.
// The ref is parsed to determine which condition type to create, and
// the config is parsed by each condition's specific factory logic.
func CreateFromRef(input *CreateFromRefInput) (*CreateFromRefOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input is nil")
	}

	if input.Ref == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "ref is required")
	}

	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "member_id is required")
	}

	// Parse the ref to get the condition type
	ref, err := core.ParseString(input.Ref)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse ref: %s", input.Ref)
	}

	// Validate module and type
	if ref.Module != refs.Module {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unsupported module: %s", ref.Module)
	}
	if ref.Type != refs.TypeConditions {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"unsupported type: %s (expected '%s')", ref.Type, refs.TypeConditions)
	}

	// Create the condition based on the ID
	var condition dnd5eEvents.ConditionBehavior

	switch ref.ID {
	case refs.Conditions.UnarmoredDefense().ID:
		condition, err = createUnarmoredDefense(input.Config, input.MemberID, input.SourceRef)
	case refs.Conditions.Raging().ID:
		condition, err = createRaging(input.Config, input.MemberID, input.SourceRef)
	case refs.Conditions.BrutalCritical().ID:
		condition, err = createBrutalCritical(input.Config, input.MemberID)
	case refs.Conditions.FightingStyleArchery().ID:
		condition = NewFightingStyleArcheryCondition(input.MemberID)
	case refs.Conditions.FightingStyleDefense().ID:
		condition = NewFightingStyleDefenseCondition(input.MemberID)
	case refs.Conditions.FightingStyleDueling().ID:
		condition = NewFightingStyleDuelingCondition(input.MemberID)
	case refs.Conditions.FightingStyleGreatWeaponFighting().ID:
		condition = NewFightingStyleGreatWeaponFightingCondition(input.MemberID, nil)
	case refs.Conditions.FightingStyleProtection().ID:
		condition = NewFightingStyleProtectionCondition(input.MemberID)
	case refs.Conditions.FightingStyleTwoWeaponFighting().ID:
		condition = NewFightingStyleTwoWeaponFightingCondition(input.MemberID)
	case refs.Conditions.ImprovedCritical().ID:
		condition, err = createImprovedCritical(input.Config, input.MemberID)
	case refs.Conditions.MartialArts().ID:
		condition, err = createMartialArts(input.Config, input.MemberID)
	case refs.Conditions.UnarmoredMovement().ID:
		condition, err = createUnarmoredMovement(input.Config, input.MemberID)
	case refs.Conditions.SneakAttack().ID:
		condition, err = createSneakAttack(input.Config, input.MemberID)
	case refs.Conditions.Disengaging().ID:
		condition = NewDisengagingCondition(input.MemberID)
	case refs.Conditions.Dodging().ID:
		condition = NewDodgingCondition(input.MemberID)
	case refs.Conditions.Prone().ID:
		condition = NewProneCondition(input.MemberID)
	case refs.Conditions.Hidden().ID:
		condition = NewHiddenCondition(input.MemberID)
	case refs.Conditions.Helped().ID:
		condition, err = createHelped(input.Config, input.MemberID)
	case refs.Conditions.Inspired().ID:
		condition, err = createInspired(input.Config, input.MemberID)
	case refs.Conditions.TrueStrike().ID:
		condition, err = createTrueStrike(input.Config, input.MemberID, input.SourceRef)
	case refs.Conditions.BladeWard().ID:
		condition, err = createBladeWard(input.Config, input.MemberID, input.SourceRef)
	case refs.Conditions.ViciousMockery().ID:
		condition, err = createViciousMockery(input.Config, input.MemberID, input.SourceRef)
	case refs.Conditions.Commanded().ID:
		condition, err = createCommanded(input.Config, input.MemberID, input.SourceRef)
	case refs.Conditions.Concentrating().ID:
		condition, err = createConcentrating(input.Config, input.MemberID, input.SourceRef)
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown condition: %s", ref.ID)
	}

	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to create condition: %s", ref.ID)
	}

	return &CreateFromRefOutput{Condition: condition}, nil
}

// unarmoredDefenseConfig is the config structure for unarmored defense
type unarmoredDefenseConfig struct {
	Variant string `json:"variant"` // "barbarian" or "monk"
}

// createUnarmoredDefense creates an unarmored defense condition from config
func createUnarmoredDefense(config json.RawMessage, characterID, sourceRef string) (*UnarmoredDefenseCondition, error) {
	var cfg unarmoredDefenseConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse unarmored defense config")
		}
	}

	// Default to barbarian variant if not specified
	variant := UnarmoredDefenseBarbarian
	if cfg.Variant == "monk" {
		variant = UnarmoredDefenseMonk
	}

	return NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: characterID,
		Type:     variant,
		Source:   sourceRef,
	}), nil
}

// ragingConfig is the config structure for raging condition
type ragingConfig struct {
	DamageBonus int `json:"damage_bonus"`
	Level       int `json:"level"`
}

// createRaging creates a raging condition from config
func createRaging(config json.RawMessage, characterID, sourceRef string) (*RagingCondition, error) {
	var cfg ragingConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse raging config")
		}
	}

	// Default damage bonus to 2 if not specified
	damageBonus := cfg.DamageBonus
	if damageBonus == 0 {
		damageBonus = 2
	}

	// Default to rage feature ref if not specified
	source := sourceRef
	if source == "" {
		source = refs.Features.Rage().String()
	}

	return &RagingCondition{
		CharacterID: characterID,
		DamageBonus: damageBonus,
		Level:       cfg.Level,
		Source:      source,
	}, nil
}

// brutalCriticalConfig is the config structure for brutal critical
type brutalCriticalConfig struct {
	Level int `json:"level"` // Barbarian level (9+ for 1 die, 13+ for 2, 17+ for 3)
}

// createBrutalCritical creates a brutal critical condition from config
func createBrutalCritical(config json.RawMessage, memberID string) (*BrutalCriticalCondition, error) {
	var cfg brutalCriticalConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse brutal critical config")
		}
	}

	// Level determines extra dice via calculateExtraDice in the constructor
	// Default to level 9 if not specified (minimum level for brutal critical)
	level := cfg.Level
	if level == 0 {
		level = 9
	}

	return NewBrutalCriticalCondition(BrutalCriticalInput{
		MemberID: memberID,
		Level:    level,
	}), nil
}

// improvedCriticalConfig is the config structure for improved critical
type improvedCriticalConfig struct {
	Threshold int `json:"threshold"` // Critical threshold (default 19)
}

// createImprovedCritical creates an improved critical condition from config
func createImprovedCritical(config json.RawMessage, memberID string) (*ImprovedCriticalCondition, error) {
	var cfg improvedCriticalConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse improved critical config")
		}
	}

	// Default to 19 if not specified
	threshold := cfg.Threshold
	if threshold == 0 {
		threshold = 19
	}

	return NewImprovedCriticalCondition(ImprovedCriticalInput{
		MemberID:  memberID,
		Threshold: threshold,
	}), nil
}

// martialArtsConfig is the config structure for martial arts
type martialArtsConfig struct {
	MonkLevel int `json:"monk_level"`
}

// createMartialArts creates a martial arts condition from config
func createMartialArts(config json.RawMessage, memberID string) (*MartialArtsCondition, error) {
	var cfg martialArtsConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse martial arts config")
		}
	}

	// Monk level is required
	if cfg.MonkLevel == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "martial arts config requires 'monk_level' field")
	}

	return NewMartialArtsCondition(MartialArtsInput{
		MemberID:  memberID,
		MonkLevel: cfg.MonkLevel,
	}), nil
}

// unarmoredMovementConfig is the config structure for unarmored movement
type unarmoredMovementConfig struct {
	MonkLevel int `json:"monk_level"`
}

// createUnarmoredMovement creates an unarmored movement condition from config
func createUnarmoredMovement(config json.RawMessage, memberID string) (*UnarmoredMovementCondition, error) {
	var cfg unarmoredMovementConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse unarmored movement config")
		}
	}

	// Monk level is required
	if cfg.MonkLevel == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "unarmored movement config requires 'monk_level' field")
	}

	return NewUnarmoredMovementCondition(UnarmoredMovementInput{
		MemberID:  memberID,
		MonkLevel: cfg.MonkLevel,
	}), nil
}

// sneakAttackConfig is the config structure for sneak attack
type sneakAttackConfig struct {
	RogueLevel int `json:"rogue_level"`
}

// createSneakAttack creates a sneak attack condition from config
func createSneakAttack(config json.RawMessage, memberID string) (*SneakAttackCondition, error) {
	var cfg sneakAttackConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse sneak attack config")
		}
	}

	// Default to level 1 if not specified
	level := cfg.RogueLevel
	if level == 0 {
		level = 1
	}

	return NewSneakAttackCondition(SneakAttackInput{
		MemberID: memberID,
		Level:    level,
		// Roller is nil - will use default roller when needed
	}), nil
}

// helpedConfig is the config structure for the helped condition
type helpedConfig struct {
	HelperID string `json:"helper_id"`
}

// createHelped creates a helped condition from config. HelperID identifies
// whose next turn is the safety-net removal trigger (PHB p.192: "before the
// start of your [the helper's] next turn").
func createHelped(config json.RawMessage, memberID string) (*HelpedCondition, error) {
	var cfg helpedConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse helped config")
		}
	}

	if cfg.HelperID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "helped config requires 'helper_id' field")
	}

	return NewHelpedCondition(memberID, cfg.HelperID), nil
}

// inspiredConfig is the config structure for the inspired condition. SourceID
// is the bard who granted the die; Die is its notation, defaulted rather than
// required because level 1 is the only level this rulebook grants one at.
type inspiredConfig struct {
	SourceID string `json:"source_id"`
	Die      string `json:"die"`
}

// createInspired creates an inspired condition from config.
func createInspired(config json.RawMessage, memberID string) (*InspiredCondition, error) {
	var cfg inspiredConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse inspired config")
		}
	}

	if cfg.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "inspired config requires 'source_id' field")
	}

	return NewInspiredCondition(memberID, cfg.SourceID, cfg.Die), nil
}

// trueStrikeConfig is the config structure for the true strike condition.
// TargetID is the creature the caster gains advantage against, filled by
// whoever resolved the cast from the target it named.
type trueStrikeConfig struct {
	TargetID string `json:"target_id"`
}

// createTrueStrike creates a true strike condition from config. The member is
// the CASTER — the advantage is theirs — so a missing target id is refused
// rather than defaulted: a True Strike good against nobody in particular would
// grant advantage on every attack.
func createTrueStrike(config json.RawMessage, memberID, sourceRef string) (*TrueStrikeCondition, error) {
	var cfg trueStrikeConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse true strike config")
		}
	}

	if cfg.TargetID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "true strike config requires 'target_id' field")
	}

	return NewTrueStrikeCondition(memberID, cfg.TargetID, sourceRef), nil
}

// bladeWardConfig is the config structure for the blade ward condition.
// TurnEnds is how many of the warded creature's own turn ends the ward lasts,
// declared by the cast content rather than assumed here.
type bladeWardConfig struct {
	TurnEnds int `json:"turn_ends"`
}

// createBladeWard creates a blade ward condition from config. The member is the
// warded creature, who for a self-targeted cast is also the caster.
//
// A missing or non-positive turn count is REFUSED rather than defaulted. A ward
// that expired before it was applied is an affordance with nothing behind it,
// and defaulting here would put a duration ruling in the factory where content
// is supposed to own it.
func createBladeWard(config json.RawMessage, memberID, sourceRef string) (*BladeWardCondition, error) {
	var cfg bladeWardConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse blade ward config")
		}
	}

	if cfg.TurnEnds <= 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"blade ward config requires a positive 'turn_ends' field")
	}

	return NewBladeWardCondition(memberID, sourceRef, cfg.TurnEnds), nil
}

// commandedConfig is the config structure for the commanded condition. The
// caster and the word both arrive by binding — the caster under the cast
// effect's CounterpartKey, the word under its OptionKey — so a missing one is
// a binding that did not happen rather than an omission content made.
type commandedConfig struct {
	CasterID string `json:"caster_id"`
	Word     string `json:"word"`
	TurnEnds int    `json:"turn_ends"`
}

// createCommanded creates a commanded condition from config. The member is the
// creature that failed its save.
//
// Every field is REFUSED rather than defaulted, which is the whole of what this
// function adds over the constructor: two of the three are written here by the
// engine rather than by content, and a default would turn a binding that
// silently failed into a compulsion that looks fine and walks at nobody.
func createCommanded(config json.RawMessage, memberID, sourceRef string) (*CommandedCondition, error) {
	var cfg commandedConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse commanded config")
		}
	}

	return NewCommandedCondition(memberID, sourceRef, cfg.CasterID, cfg.Word, cfg.TurnEnds)
}

// viciousMockeryConfig is the config structure for the vicious mockery
// condition. SourceID is the bard who imposed it.
type viciousMockeryConfig struct {
	SourceID string `json:"source_id"`
}

// createViciousMockery creates a vicious mockery condition from config. The
// member is the mocked creature.
func createViciousMockery(
	config json.RawMessage, memberID, sourceRef string,
) (*ViciousMockeryCondition, error) {
	var cfg viciousMockeryConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse vicious mockery config")
		}
	}

	if cfg.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "vicious mockery config requires 'source_id' field")
	}

	return NewViciousMockeryCondition(memberID, cfg.SourceID, sourceRef), nil
}

// concentratingConfig is the config structure for the concentrating condition.
// SpellRef and SpellName name what is being held together; TurnEnds is the
// spell's own duration; Children are the addresses it already left behind,
// present for a hold rebuilt rather than freshly cast.
type concentratingConfig struct {
	SpellRef         string                         `json:"spell_ref"`
	SpellName        string                         `json:"spell_name"`
	TurnEnds         int                            `json:"turn_ends"`
	SkipFirstTurnEnd bool                           `json:"skip_first_turn_end"`
	Children         []dnd5eEvents.ConditionAddress `json:"children"`
}

// createConcentrating creates a concentrating condition from config. The member
// is the CASTER — the hold is theirs.
//
// A missing spell falls back to the ref of whatever granted the condition,
// which for a cast IS the spell. Both empty is refused rather than defaulted:
// concentration on nothing is a badge with no spell behind it, and the check it
// would provoke would have nothing at stake. A duration of zero is refused for
// the same reason a profile's is — a hold whose clock already ran out.
func createConcentrating(config json.RawMessage, memberID, sourceRef string) (*ConcentratingCondition, error) {
	var cfg concentratingConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse concentrating config")
		}
	}

	spellRef := cfg.SpellRef
	if spellRef == "" {
		spellRef = sourceRef
	}
	if spellRef == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "concentrating config requires 'spell_ref' field")
	}
	if cfg.TurnEnds <= 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "concentrating config requires a positive 'turn_ends'")
	}

	sourceID := ""
	if spellRef == refs.Spells.Bane().String() {
		sourceID = memberID
	}
	condition := NewConcentratingConditionWithInput(NewConcentratingConditionInput{
		MemberID:         memberID,
		SourceID:         sourceID,
		SpellRef:         spellRef,
		SpellName:        cfg.SpellName,
		TurnEnds:         cfg.TurnEnds,
		SkipFirstTurnEnd: cfg.SkipFirstTurnEnd,
	})
	condition.Children = cfg.Children
	return condition, nil
}
