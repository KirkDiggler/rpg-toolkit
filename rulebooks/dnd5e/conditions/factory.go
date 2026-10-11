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

// CreateFromRef creates a condition from a ref string and configuration. The
// whole parsed ref is looked up in the registry, so a known id under another
// module or type is unknown, and the type's own create reads the config.
//
// Errors: nil input, an empty ref or member, an unparsable ref, an unregistered
// ref (wrapping [ErrUnknownCondition]), a registered type nothing builds from a
// ref (wrapping [ErrNotCreatable]), and the type's own refusal.
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

	ref, err := core.ParseString(input.Ref)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse ref: %s", input.Ref)
	}

	entry, err := lookup(ref.String())
	if err != nil {
		return nil, err
	}
	if entry.create == nil {
		return nil, rpgerr.WrapWithCode(ErrNotCreatable, rpgerr.CodeInvalidArgument,
			"condition is not created from a ref: "+ref.String())
	}

	condition, err := entry.create(input)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to create condition: %s", ref.String())
	}

	return &CreateFromRefOutput{Condition: condition}, nil
}

// createdAs adapts a constructor that returns its own pointer type and an
// error to a create closure's interface result. It exists so a failed
// construction returns a nil interface, never an interface holding a nil
// pointer.
func createdAs[T dnd5eEvents.ConditionBehavior](condition T, err error) (dnd5eEvents.ConditionBehavior, error) {
	if err != nil {
		return nil, err
	}
	return condition, nil
}

// unarmoredDefenseConfig is the config structure for unarmored defense
type unarmoredDefenseConfig struct {
	Variant string `json:"variant"` // "barbarian" or "monk"
}

// createUnarmoredDefense creates an unarmored defense condition from config.
// Both grants name their variant: a missing or unknown one is refused, never
// defaulted to either.
func createUnarmoredDefense(config json.RawMessage, characterID, sourceRef string) (*UnarmoredDefenseCondition, error) {
	var cfg unarmoredDefenseConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse unarmored defense config")
		}
	}

	var variant UnarmoredDefenseType
	switch cfg.Variant {
	case "barbarian":
		variant = UnarmoredDefenseBarbarian
	case "monk":
		variant = UnarmoredDefenseMonk
	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"unarmored defense config requires a 'variant' of \"barbarian\" or \"monk\", got %q", cfg.Variant)
	}

	return NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: characterID,
		Type:     variant,
		Source:   sourceRef,
	}), nil
}

// createRaging creates a raging condition. Rage takes no config: its damage
// bonus is read from the attacker's barbarian levels in each attack's frame,
// so no level or bonus is accepted, stored or defaulted.
func createRaging(characterID, sourceRef string) *RagingCondition {
	// Default to rage feature ref if not specified
	source := sourceRef
	if source == "" {
		source = refs.Features.Rage().String()
	}

	return &RagingCondition{
		CharacterID: characterID,
		Source:      source,
	}
}

// createBrutalCritical creates a brutal critical condition. It takes no
// config: its dice are read from the attacker's barbarian levels in each
// attack's frame.
func createBrutalCritical(memberID string) *BrutalCriticalCondition {
	return NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: memberID})
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

// createMartialArts creates a martial arts condition. It takes no config:
// the Martial Arts die is read from the sheet's level record at each swing.
func createMartialArts(memberID string) *MartialArtsCondition {
	return NewMartialArtsCondition(MartialArtsInput{MemberID: memberID})
}

// createSneakAttack creates a sneak attack condition. It takes no config: its
// dice are read from the attacker's rogue levels in each attack's frame, and a
// missing level is never defaulted.
func createSneakAttack(memberID string) *SneakAttackCondition {
	return NewSneakAttackCondition(SneakAttackInput{
		MemberID: memberID,
		// Roller is nil - will use default roller when needed
	})
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

// guidedConfig is the config structure for the guided condition. SourceID is
// the cleric who cast Guidance.
type guidedConfig struct {
	SourceID string `json:"source_id"`
}

// createGuided creates a guided condition from config. The member is the
// creature Guidance touched.
//
// Unlike the plain-string sourceRef conditions above, [NewGuidedCondition]
// takes a parsed *core.Ref and validates it against the canonical Guidance
// spell ref — [BlessedCondition]'s strictness, not [ViciousMockeryCondition]'s
// — so the string this factory is handed is parsed here rather than deferred
// to the constructor.
func createGuided(config json.RawMessage, memberID, sourceRef string) (*GuidedCondition, error) {
	var cfg guidedConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse guided config")
		}
	}

	ref, err := core.ParseString(sourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse guided source ref: %s", sourceRef)
	}

	return NewGuidedCondition(NewGuidedConditionInput{
		MemberID: memberID, SourceID: cfg.SourceID, SourceRef: ref,
	})
}

// resistanceConfig is the config structure for the resistance condition.
// SourceID is the cleric who cast Resistance.
type resistanceConfig struct {
	SourceID string `json:"source_id"`
}

// createResistance creates a resistance condition from config. The member is
// the creature Resistance touched.
//
// Unlike the plain-string sourceRef conditions above, [NewResistanceCondition]
// takes a parsed *core.Ref and validates it against the canonical Resistance
// spell ref — [BlessedCondition]'s strictness, not [ViciousMockeryCondition]'s
// — so the string this factory is handed is parsed here rather than deferred
// to the constructor. Mirrors [createGuided] exactly.
func createResistance(config json.RawMessage, memberID, sourceRef string) (*ResistanceCondition, error) {
	var cfg resistanceConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse resistance config")
		}
	}

	ref, err := core.ParseString(sourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse resistance source ref: %s", sourceRef)
	}

	return NewResistanceCondition(NewResistanceConditionInput{
		MemberID: memberID, SourceID: cfg.SourceID, SourceRef: ref,
	})
}

// sanctuaryConfig is the config structure for the sanctuary condition.
// SourceID is the cleric who cast Sanctuary; SaveDC is that cleric's spell
// save DC when the ward landed, written under the cast effect's SaveDCKey.
type sanctuaryConfig struct {
	SourceID string `json:"source_id"`
	SaveDC   int    `json:"save_dc"`
}

// createSanctuary creates a sanctuary condition from config. The member is
// the creature Sanctuary wards. Mirrors [createGuided] exactly.
func createSanctuary(config json.RawMessage, memberID, sourceRef string) (*SanctuaryCondition, error) {
	var cfg sanctuaryConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse sanctuary config")
		}
	}

	ref, err := core.ParseString(sourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse sanctuary source ref: %s", sourceRef)
	}

	return NewSanctuaryCondition(NewSanctuaryConditionInput{
		MemberID: memberID, SourceID: cfg.SourceID, SourceRef: ref, SaveDC: cfg.SaveDC,
	})
}

// sanctuaryImmuneConfig is the config structure for the sanctuary immune
// condition. SourceID is the caster whose Sanctuary this immunity blocks.
type sanctuaryImmuneConfig struct {
	SourceID string `json:"source_id"`
}

// createSanctuaryImmune creates a sanctuary immune condition from config.
// The member is the attacker who succeeded the ward save. Mirrors
// [createGuided] exactly.
func createSanctuaryImmune(config json.RawMessage, memberID, sourceRef string) (*SanctuaryImmuneCondition, error) {
	var cfg sanctuaryImmuneConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse sanctuary immune config")
		}
	}

	ref, err := core.ParseString(sourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse sanctuary immune source ref: %s", sourceRef)
	}

	return NewSanctuaryImmuneCondition(NewSanctuaryImmuneConditionInput{
		MemberID: memberID, SourceID: cfg.SourceID, SourceRef: ref,
	})
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

// createGuidingBolt builds the on-hit target light from its originating caster.
func createGuidingBolt(config json.RawMessage, memberID, sourceRef string) (*GuidingBoltCondition, error) {
	var cfg sanctuaryImmuneConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse guiding bolt config")
		}
	}

	ref, err := core.ParseString(sourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse guiding bolt source ref: %s", sourceRef)
	}

	return NewGuidingBoltCondition(NewGuidingBoltConditionInput{
		MemberID: memberID, SourceID: cfg.SourceID, SourceRef: ref,
	})
}

type shieldOfFaithConfig struct {
	SourceID string `json:"source_id"`
}

func createShieldOfFaith(config json.RawMessage, memberID, sourceRef string) (*ShieldOfFaithCondition, error) {
	var cfg shieldOfFaithConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse shieldOfFaith config")
		}
	}

	ref, err := core.ParseString(sourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse shieldOfFaith source ref: %s", sourceRef)
	}

	return NewShieldOfFaithCondition(NewShieldOfFaithConditionInput{
		MemberID: memberID, SourceID: cfg.SourceID, SourceRef: ref,
	})
}

func createFaerieFire(config json.RawMessage, memberID, sourceRef string) (*FaerieFireCondition, error) {
	var cfg faerieFireConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, rpgerr.Wrap(err, "failed to parse faerieFire config")
		}
	}

	ref, err := core.ParseString(sourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to parse faerieFire source ref: %s", sourceRef)
	}

	return NewFaerieFireCondition(NewFaerieFireConditionInput{
		MemberID: memberID, SourceID: cfg.SourceID, SourceRef: ref,
	})
}

type faerieFireConfig struct {
	SourceID string `json:"source_id"`
}

// blessingConfig is the config the Bless and Bane conditions are built from.
// SourceID is the caster.
type blessingConfig struct {
	SourceID string `json:"source_id"`
}

// createBlessed builds Blessed on the member from the cast's config and the
// ref of the spell that granted it.
func createBlessed(in *CreateFromRefInput) (*BlessedCondition, error) {
	var config blessingConfig
	if err := json.Unmarshal(in.Config, &config); err != nil {
		return nil, rpgerr.Wrapf(err, "build condition blessed for %q", in.MemberID)
	}
	source, err := core.ParseString(in.SourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "build condition blessed for %q: source", in.MemberID)
	}
	condition, err := NewBlessedCondition(NewBlessedConditionInput{
		MemberID: in.MemberID, SourceID: config.SourceID, SourceRef: source,
	})
	if err != nil {
		return nil, rpgerr.Wrapf(err, "build condition blessed for %q", in.MemberID)
	}
	return condition, nil
}

// createBaned builds Baned on the member from the cast's config and the ref of
// the spell that granted it.
func createBaned(in *CreateFromRefInput) (*BanedCondition, error) {
	var config blessingConfig
	if err := json.Unmarshal(in.Config, &config); err != nil {
		return nil, rpgerr.Wrapf(err, "build condition baned for %q", in.MemberID)
	}
	source, err := core.ParseString(in.SourceRef)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "build condition baned for %q: source", in.MemberID)
	}
	condition, err := NewBanedCondition(NewBanedConditionInput{
		MemberID: in.MemberID, SourceID: config.SourceID, SourceRef: source,
	})
	if err != nil {
		return nil, rpgerr.Wrapf(err, "build condition baned for %q", in.MemberID)
	}
	return condition, nil
}
