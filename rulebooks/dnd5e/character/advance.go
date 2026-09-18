// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// AdvanceInput contains everything a level needs that the toolkit cannot
// derive for itself.
type AdvanceInput struct {
	// ClassID is the class the new level is taken in. Until multiclassing
	// exists it must equal the character's own class (R2.4); it is stated
	// rather than assumed so the caller that opens that seam is the one that
	// changes, not this signature.
	ClassID classes.Class

	// HitPointMethod selects how the hit point gain is produced. The toolkit
	// computes the VALUE; the caller never supplies it (R4.3), because a
	// caller-supplied number would put a game rule in the orchestrator.
	// [HitPointMethodMax] is level 1 only and is refused here.
	HitPointMethod HitPointMethod

	// Choices are the selections this level requires. May be empty — and must
	// be, for a level that requires none: a choice nobody asked for would be
	// written into an append-only record that can never be corrected.
	Choices []choices.ChoiceData

	// Roller supplies the randomness for [HitPointMethodRolled]. Nil defaults
	// to dice.NewRoller(), the same contract [MakeSavingThrowInput] and
	// [SpendHitDiceInput] carry. It is a source of faces, not a hit point
	// total, so it does not put the rule in the caller.
	Roller dice.Roller
}

// AdvanceOutput reports what the level did.
type AdvanceOutput struct {
	// Entry is the record entry that was appended.
	Entry LevelEntry

	// Gained describes what this level added, for display. DERIVED from the
	// entry plus the current rules, never stored — see [LevelEntry].
	Gained GainedAtLevel
}

// GainedAtLevel is what one level added to a character, as the character can
// describe it immediately afterwards.
//
// It is a projection for display and nothing reads it back: every field here
// is recomputable from the level record and the current rules, which is the
// whole point of storing inputs rather than effects (design §7.1).
type GainedAtLevel struct {
	// CharacterLevel is the level the character now holds — the total number
	// of levels taken.
	CharacterLevel int

	// ClassLevel is the number of levels now taken in the class this level was
	// taken in. Grants and class resources are indexed by this, never by
	// CharacterLevel (R4.6).
	ClassLevel int

	// Features are the refs of the features this level granted.
	Features []core.Ref

	// Conditions are the refs of the conditions this level granted.
	Conditions []core.Ref

	// HitPointGain is what this level added to maximum hit points.
	HitPointGain int

	// ProficiencyBonus is the bonus the character now has, derived from
	// CharacterLevel (R4.7). Reported because it is the number most likely to
	// have moved without anything visible granting it.
	ProficiencyBonus int

	// Resources are the pools whose maximum this level moved, in key order.
	//
	// A slot increase is not a question and is applied without asking (R4.6),
	// which means nothing on the level-up screen would mention it unless the
	// engine says so. This is what lets a response read "1st-level spell slots
	// 2 to 3" instead of leaving the player to notice (R4.7).
	Resources []ResourceChange
}

// ResourceChange is one pool whose maximum a level moved.
//
// From is 0 for a pool the character did not have before, which is the same
// thing said about a maximum that rose from nothing.
type ResourceChange struct {
	// Key names the pool.
	Key coreResources.ResourceKey

	// From is the maximum before this level.
	From int

	// To is the maximum after it.
	To int
}

// Advance takes one more level in a class, appending it to the character's
// record and applying what that level grants.
//
// This is the primitive character advancement was missing: until it existed a
// [classes.Grant] was consumed only by draft compilation, so nothing could
// apply one to a character that already exists. The same act is what a
// subclass at 3, an ability score improvement at 4, a feat, or a boon will
// each use — level 2 is the cheapest thing that proves it, not the goal.
//
// What it does, in order (design §4.1): validate everything and reject before
// mutating anything; count the character level and the class level separately;
// take the grants gained AT that class level, never the cumulative set, which
// would grant every level-1 feature a second time (R3.2); build the features
// and conditions those grants name; attach them; resize the class pools; add
// the hit points; append the entry.
//
// **It is atomic.** Everything that can fail happens before anything is
// mutated, except the bus attachments — and those are undone, newest first, if
// a later one fails. A failed Advance leaves the character exactly as it was,
// record included (R4.2). See the note on attachment ordering inside.
//
// It refuses a character that is in combat (R4.1): advancement is a
// between-run act, and attaching a condition to a character mid-turn is a
// different problem this design does not solve.
//
// Returns a populated output or an error, never (nil, nil) (R4.4).
func (c *Character) Advance(ctx context.Context, input *AdvanceInput) (*AdvanceOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "advance input is required")
	}
	if err := c.checkCanAdvance(input); err != nil {
		return nil, err
	}

	characterLevel := len(c.levels) + 1
	classLevel := c.ClassLevel(input.ClassID) + 1

	grants := classes.GetGrantsGainedAtLevel(input.ClassID, classLevel)
	if err := checkGrantsApplicable(grants, input.ClassID, classLevel); err != nil {
		return nil, err
	}
	if err := c.checkLevelChoices(input, classLevel); err != nil {
		return nil, err
	}

	// What the level's choices become on the sheet, built through the compiler
	// creation uses (R4.4c). Built with the rest of the validation because a
	// choice naming something this build cannot turn into a ref is a content
	// defect and must stop the level rather than produce a sheet quietly
	// missing a spell — the same reason finalization compiles before it builds.
	chosenCantrips, err := compileKnownSpells(input.Choices, shared.ChoiceCantrips, "cantrip")
	if err != nil {
		return nil, err
	}
	chosenSpells, err := compileKnownSpells(input.Choices, shared.ChoiceSpells, "spell")
	if err != nil {
		return nil, err
	}
	if err := c.checkNothingAlreadyKnown(chosenCantrips, chosenSpells); err != nil {
		return nil, err
	}

	if c.classID == classes.Cleric {
		// Add only newly gained domain spells; existing grants remain on the sheet.
		grants := choices.ClericSpellGrants(c.subclassID, classLevel)
		grants = withoutKnown(grants, c.knownSpells)
		chosenSpells, err = appendSpellGrants(chosenSpells, grants)
		if err != nil {
			return nil, err
		}
	}

	newFeatures, newConditions, err := c.buildGranted(grants, input.ClassID)
	if err != nil {
		return nil, err
	}

	hitPointGain, err := c.rollHitPointGain(ctx, input)
	if err != nil {
		return nil, err
	}

	// The last thing that can fail, and the only mutation with an undo. After
	// this line nothing below returns an error, so the sheet either takes
	// every change this level makes or none of them.
	attached, err := c.attachGranted(ctx, newFeatures, newConditions)
	if err != nil {
		return nil, err
	}

	entry := LevelEntry{
		Level:          characterLevel,
		ClassID:        input.ClassID,
		HitPointGain:   hitPointGain,
		HitPointMethod: input.HitPointMethod,
		Choices:        cloneChoices(input.Choices),
	}

	resourceChanges := c.commitLevel(commitLevelInput{
		entry:          entry,
		newFeatures:    newFeatures,
		newConditions:  newConditions,
		attached:       attached,
		knownCantrips:  chosenCantrips,
		knownSpells:    chosenSpells,
		classLevel:     classLevel,
		characterLevel: characterLevel,
	})

	// The record the caller is handed is a copy: the entry on the sheet is
	// append-only, and a shared Choices slice would let a reader edit it.
	reported := entry
	reported.Choices = cloneChoices(entry.Choices)

	return &AdvanceOutput{
		Entry: reported,
		Gained: GainedAtLevel{
			CharacterLevel:   characterLevel,
			ClassLevel:       classLevel,
			Features:         grantedFeatureRefs(newFeatures),
			Conditions:       grantedConditionRefs(newConditions),
			HitPointGain:     hitPointGain,
			ProficiencyBonus: c.ProficiencyBonus(),
			Resources:        resourceChanges,
		},
	}, nil
}

// checkCanAdvance holds every refusal that depends only on the input and the
// character's current state, so that Advance reads as validate-then-act.
func (c *Character) checkCanAdvance(input *AdvanceInput) error {
	if len(c.levels) == 0 {
		return rpgerr.NewfWithOpts(rpgerr.CodeInvalidState, []rpgerr.Option{
			rpgerr.WithMeta("character_id", c.id),
		}, "character %q has no level record, so there is no level to advance FROM", c.id)
	}

	if input.ClassID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "class is required")
	}

	// R2.4: every entry names the character's own class. This is the seam
	// multiclassing will open, and refusing here is what keeps a record from
	// recording a level nothing else in the engine believes in.
	if input.ClassID != c.classID {
		return rpgerr.NewfWithOpts(rpgerr.CodeNotAllowed, []rpgerr.Option{
			rpgerr.WithMeta("character_id", c.id),
			rpgerr.WithMeta("requested_class", string(input.ClassID)),
			rpgerr.WithMeta("character_class", string(c.classID)),
		}, "cannot take a level in %q: character %q is a %q and multiclassing does not exist yet",
			input.ClassID, c.id, c.classID)
	}

	switch input.HitPointMethod {
	case HitPointMethodRolled, HitPointMethodAverage:
	case HitPointMethodMax:
		return rpgerr.NewfWithOpts(rpgerr.CodeInvalidArgument, []rpgerr.Option{
			rpgerr.WithMeta("character_id", c.id),
		}, "hit point method %q is level 1 only; a level gained later is rolled or averaged",
			input.HitPointMethod)
	default:
		return rpgerr.NewfWithOpts(rpgerr.CodeInvalidArgument, []rpgerr.Option{
			rpgerr.WithMeta("character_id", c.id),
		}, "unknown hit point method %q", input.HitPointMethod)
	}

	// R4.11: a level must be EARNED. Entitlement is derived from the experience
	// total by the 2014 threshold table, and the gap between it and the record
	// is the whole of the "level up available" signal — there is no flag to go
	// stale. The rule lives here because the toolkit owns rules: the same check
	// in the orchestrator would be a game rule in the API, and an API that can
	// decide when a level is earned is one that can grant one.
	if taking := len(c.levels) + 1; taking > c.EntitledLevel() {
		return rpgerr.NewfWithOpts(rpgerr.CodePrerequisiteNotMet, []rpgerr.Option{
			rpgerr.WithMeta("character_id", c.id),
			rpgerr.WithMeta("experience", c.experience),
			rpgerr.WithMeta("required_experience", ExperienceThresholdForLevel(taking)),
			rpgerr.WithMeta("entitled_level", c.EntitledLevel()),
		}, "character %q has %d experience and level %d needs %d",
			c.id, c.experience, taking, ExperienceThresholdForLevel(taking))
	}

	// R4.1. InCombat is what the sheet itself can answer: it holds a live
	// action economy from its first turn in a fight until the session clears
	// it when the fight dissolves. A character seated in an encounter that has
	// not yet reached its first turn is not visible from here — the sheet does
	// not know what an encounter is — so this refusal is narrower than the
	// rule it enforces. It is the only signal on this side of the seam.
	if c.InCombat() {
		return rpgerr.NewfWithOpts(rpgerr.CodeTimingRestriction, []rpgerr.Option{
			rpgerr.WithMeta("character_id", c.id),
		}, "character %q is in combat; advancement is a between-run act", c.id)
	}

	return nil
}

// checkGrantsApplicable refuses a grant this primitive would only half apply.
//
// A grant carries proficiencies, equipment, spells and languages as well as
// features and conditions, and advancement applies only the last two (design
// §4.1 step 6). No grant above level 1 carries any of the rest today, so this
// forbids nothing that exists — it is here so that the day one does, the level
// fails loudly instead of quietly dropping half of what it granted.
func checkGrantsApplicable(grants []classes.Grant, classID classes.Class, classLevel int) error {
	for _, grant := range grants {
		var carried string
		switch {
		case len(grant.ArmorProficiencies) > 0 || len(grant.WeaponProficiencies) > 0 ||
			len(grant.ToolProficiencies) > 0 || len(grant.SkillProficiencies) > 0:
			carried = "proficiencies"
		case len(grant.Equipment) > 0:
			carried = "equipment"
		case len(grant.Spells) > 0:
			carried = "spells"
		case len(grant.Languages) > 0:
			carried = "languages"
		default:
			continue
		}

		return rpgerr.NewfWithOpts(rpgerr.CodeNotAllowed, []rpgerr.Option{
			rpgerr.WithMeta("class", string(classID)),
			rpgerr.WithMeta("class_level", classLevel),
			rpgerr.WithMeta("carries", carried),
		}, "%s level %d grants %s, which advancement cannot apply; "+
			"applying the rest would leave the character quietly incomplete",
			classID, classLevel, carried)
	}

	return nil
}

// checkLevelChoices is the whole of what a level asks of its input: that the
// choices this level requires are the choices it was given, that they are
// legal answers, and that advancement can actually apply them.
//
// It used to check PRESENCE only — a set of supplied ids against a set of
// required ids — so three off-list spells under the right id passed, and so did
// one spell where the level asked for two. Creation validates through
// [choices.Validator]; advancement now validates through the same one, against
// the requirements that level gained (design R4.4b: "Presence is not
// validation").
//
// A level that grants nothing is a valid level — several classes have them —
// so an empty requirement set is not by itself an error.
func (c *Character) checkLevelChoices(input *AdvanceInput, classLevel int) error {
	required := choices.GetClassRequirementsGainedAtLevel(input.ClassID, classLevel)
	if input.ClassID == classes.Cleric {
		choices.ExcludeGrantedSpellChoices(required, c.subclassID, classLevel)
	}
	ids := required.ChoiceIDs()

	if len(ids) == 0 {
		if len(input.Choices) > 0 {
			return rpgerr.NewfWithOpts(rpgerr.CodeInvalidArgument, []rpgerr.Option{
				rpgerr.WithMeta("class", string(input.ClassID)),
				rpgerr.WithMeta("class_level", classLevel),
			}, "%s level %d requires no choices, but %d were supplied; the record is append-only "+
				"and a choice nothing asked for could never be taken back",
				input.ClassID, classLevel, len(input.Choices))
		}
		return nil
	}

	if err := checkRequirementsApplicable(required, input.ClassID, classLevel); err != nil {
		return err
	}
	if err := checkRequirementsAnswerable(required, input.ClassID, classLevel); err != nil {
		return err
	}
	if err := checkNothingUnasked(input, ids, classLevel); err != nil {
		return err
	}

	result := choices.NewValidator().Validate(required, choices.SubmissionsFrom(input.Choices))
	if !result.Valid && len(result.Errors) > 0 {
		first := result.Errors[0]
		return rpgerr.NewfWithOpts(rpgerr.CodeInvalidArgument, []rpgerr.Option{
			rpgerr.WithMeta("class", string(input.ClassID)),
			rpgerr.WithMeta("class_level", classLevel),
			rpgerr.WithMeta("choice_id", string(first.ChoiceID)),
			rpgerr.WithMeta("category", string(first.Category)),
		}, "%s level %d: %s", input.ClassID, classLevel, first.Message)
	}

	return nil
}

// NextLevelRequirements returns what the level this character would take next
// asks OF IT — the class's row for that level, with everything it already knows
// removed from the spell and cantrip options.
//
// [choices.GetClassRequirementsGainedAtLevel] answers for a CLASS at a level,
// which is the right question at creation and an incomplete one afterwards: it
// cannot know that this particular bard already holds four of the five spells
// its level-2 row offers. A screen driven by the class answer offers a spell
// the character knows, and the player only finds out it was not a choice after
// picking it. The class function is unchanged and is still what creation reads;
// this is the character's own view of the same row.
//
// The COUNT is not reduced with the options, because how many spells a level
// teaches is the class's rule and not a property of who is taking it. A level
// whose remaining options cannot satisfy its count is unanswerable, and
// [Character.Advance] refuses it rather than teaching half of one.
//
// There is no class parameter: R2.4 forces every level into the character's own
// class, and a parameter that can hold exactly one correct value is one a
// caller can get wrong with no error to refuse it. When multiclassing opens
// that seam this gains the class the level is taken in, the way [AdvanceInput]
// already states it.
func (c *Character) NextLevelRequirements() *choices.Requirements {
	reqs := choices.GetClassRequirementsGainedAtLevel(c.classID, c.ClassLevel(c.classID)+1)
	if c.classID == classes.Cleric {
		choices.ExcludeGrantedSpellChoices(reqs, c.subclassID, c.ClassLevel(c.classID)+1)
	}

	// Safe to write to: the class function builds its rows fresh per call
	// precisely so a caller folding something in cannot edit the table.
	if reqs.Cantrips != nil {
		reqs.Cantrips.Options = withoutKnown(reqs.Cantrips.Options, c.knownCantrips)
	}
	if reqs.Spellbook != nil {
		reqs.Spellbook.Options = withoutKnown(reqs.Spellbook.Options, c.knownSpells)
	}

	return reqs
}

// withoutKnown returns the options that are not already on a known list.
//
// A ref's ID *is* the spell id — everything after the second separator — so the
// chosen vocabulary and the stored one compare directly, without composing a
// ref out of a string or parsing one back into an id.
func withoutKnown(options []spells.Spell, known []*core.Ref) []spells.Spell {
	if len(known) == 0 || len(options) == 0 {
		return options
	}

	held := make(map[spells.Spell]struct{}, len(known))
	for _, ref := range known {
		if ref != nil {
			held[spells.Spell(ref.ID)] = struct{}{}
		}
	}

	out := make([]spells.Spell, 0, len(options))
	for _, option := range options {
		if _, had := held[option]; !had {
			out = append(out, option)
		}
	}
	return out
}

// checkNothingAlreadyKnown refuses a level that would teach this character a
// spell or cantrip it already has.
//
// Found on the walk: a bard created with four of the five spells its level-2
// row offers was offered all five, because the class function answers for a
// class at a level and cannot know what one character holds. Nothing refused
// the duplicate, so choosing Bane appended a second Bane to the known list —
// a sheet holding one spell twice against a table that says five.
//
// [Character.NextLevelRequirements] keeps the duplicate off the screen. This
// keeps it off the sheet, because a screen is a courtesy and not a rule: the
// client sends what it likes and the engine decides what is legal.
func (c *Character) checkNothingAlreadyKnown(chosenCantrips, chosenSpells []*core.Ref) error {
	if err := refuseAlreadyKnown(c.id, c.knownCantrips, chosenCantrips, "cantrip"); err != nil {
		return err
	}
	return refuseAlreadyKnown(c.id, c.knownSpells, chosenSpells, "spell")
}

// refuseAlreadyKnown names the first chosen ref that is already held.
//
// Each accepted choice joins the held set as it passes, so one level naming the
// same spell through two requirements is refused by the same rule that refuses
// a repeat of something learned years ago.
func refuseAlreadyKnown(characterID string, held, chosen []*core.Ref, role string) error {
	if len(chosen) == 0 {
		return nil
	}

	known := make(map[core.ID]struct{}, len(held)+len(chosen))
	for _, ref := range held {
		if ref != nil {
			known[ref.ID] = struct{}{}
		}
	}

	for _, ref := range chosen {
		if ref == nil {
			continue
		}
		if _, had := known[ref.ID]; had {
			return rpgerr.NewfWithOpts(rpgerr.CodeInvalidArgument, []rpgerr.Option{
				rpgerr.WithMeta("character_id", characterID),
				rpgerr.WithMeta("role", role),
				rpgerr.WithMeta("spell", string(ref.ID)),
			}, "character %q already knows the %s %q", characterID, role, ref.ID)
		}
		known[ref.ID] = struct{}{}
	}

	return nil
}

// checkRequirementsApplicable refuses a level whose question advancement could
// take an answer to and then do nothing with.
//
// This wave applies spells and cantrips, because those are what the progression
// table asks for. Every other kind — a subclass, a fighting style, expertise,
// skills — is refused until a level that asks for it arrives with the compiler
// that applies it (R4.4c). It is the shape [checkGrantsApplicable] already has
// for the other half of a level, and for the same reason: failing closed and
// loudly beats a sheet that is quietly missing half a level.
func checkRequirementsApplicable(
	required *choices.Requirements, classID classes.Class, classLevel int,
) error {
	var asks string
	switch {
	case required.Subclass != nil:
		// The one requirement the engine can pose and cannot receive:
		// shared.ChoiceCategory has no subclass value (rpg-toolkit#1767).
		asks = "a subclass"
	case required.Skills != nil || len(required.AdditionalSkills) > 0:
		asks = "skills"
	case required.FightingStyle != nil:
		asks = "a fighting style"
	case required.Expertise != nil:
		asks = "expertise"
	case len(required.Equipment) > 0 || len(required.EquipmentCategories) > 0:
		asks = "equipment"
	case len(required.Languages) > 0:
		asks = "languages"
	case required.Tools != nil:
		asks = "tool proficiencies"
	default:
		return nil
	}

	return rpgerr.NewfWithOpts(rpgerr.CodeNotAllowed, []rpgerr.Option{
		rpgerr.WithMeta("class", string(classID)),
		rpgerr.WithMeta("class_level", classLevel),
		rpgerr.WithMeta("asks", asks),
	}, "%s level %d requires choosing %s, which advancement cannot apply; "+
		"taking the level would record the choice and change nothing on the sheet",
		classID, classLevel, asks)
}

// checkRequirementsAnswerable refuses a question with no answers.
//
// A class table can say a spell is learned at a level this build has no spells
// for — a wizard's 2nd-level spells, a ranger's first — and an option list of
// nothing is not a choice anyone can make. The requirement is still posed,
// because the class really does learn a spell there; refusing it names the gap
// instead of handing the player a level that taught them nothing.
func checkRequirementsAnswerable(
	required *choices.Requirements, classID classes.Class, classLevel int,
) error {
	if req := required.Cantrips; req != nil && req.Count > 0 && len(req.Options) == 0 {
		return unansweredContent(classID, classLevel, string(req.ID), "cantrips")
	}
	if req := required.Spellbook; req != nil && req.Count > 0 && len(req.Options) == 0 {
		return unansweredContent(classID, classLevel, string(req.ID),
			fmt.Sprintf("%d%s-level spells", req.SpellLevel, ordinalSuffix(req.SpellLevel)))
	}
	return nil
}

// unansweredContent is the refusal for a requirement this build has no content
// to answer with.
func unansweredContent(classID classes.Class, classLevel int, choiceID, missing string) error {
	return rpgerr.NewfWithOpts(rpgerr.CodeNotAllowed, []rpgerr.Option{
		rpgerr.WithMeta("class", string(classID)),
		rpgerr.WithMeta("class_level", classLevel),
		rpgerr.WithMeta("choice_id", choiceID),
	}, "%s level %d requires choosing from the %s %s knows, and this build has none",
		classID, classLevel, missing, classID)
}

// ordinalSuffix names a spell level the way a message should read.
func ordinalSuffix(n int) string {
	switch n {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// checkNothingUnasked refuses a choice this level did not ask for.
//
// The client sends the choices it was asked for and nothing else (R4.15). The
// record is append-only, so a choice nothing asked for is one no correction can
// ever take back — the same reason a level that requires none refuses any.
func checkNothingUnasked(input *AdvanceInput, required []choices.ChoiceID, classLevel int) error {
	asked := make(map[choices.ChoiceID]struct{}, len(required))
	for _, id := range required {
		asked[id] = struct{}{}
	}

	for _, choice := range input.Choices {
		if _, ok := asked[choice.ChoiceID]; !ok {
			return rpgerr.NewfWithOpts(rpgerr.CodeInvalidArgument, []rpgerr.Option{
				rpgerr.WithMeta("class", string(input.ClassID)),
				rpgerr.WithMeta("class_level", classLevel),
				rpgerr.WithMeta("choice_id", string(choice.ChoiceID)),
			}, "%s level %d did not ask for choice %q", input.ClassID, classLevel, choice.ChoiceID)
		}
	}

	return nil
}

// buildGranted constructs — and does not attach — everything the grants name.
//
// Construction is separated from attachment because construction is the half
// that fails on bad content and the half with nothing to undo: an unattached
// feature is a value nobody has seen.
func (c *Character) buildGranted(
	grants []classes.Grant, classID classes.Class,
) ([]features.Feature, []loadedEffect, error) {
	newFeatures := make([]features.Feature, 0)
	newConditions := make([]loadedEffect, 0)
	classSourceRef := "dnd5e:classes:" + string(classID)

	for _, grant := range grants {
		for _, featureRef := range grant.Features {
			output, err := features.CreateFromRef(&features.CreateFromRefInput{
				Ref:         featureRef.Ref,
				Config:      featureRef.Config,
				CharacterID: c.id,
			})
			if err != nil {
				return nil, nil, rpgerr.Wrapf(err, "failed to create feature from ref %s", featureRef.Ref)
			}
			newFeatures = append(newFeatures, output.Feature)
		}

		for _, condRef := range grant.Conditions {
			output, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
				Ref:       condRef.Ref,
				Config:    condRef.Config,
				MemberID:  c.id,
				SourceRef: classSourceRef,
			})
			if err != nil {
				return nil, nil, rpgerr.Wrapf(err, "failed to create condition from ref %s", condRef.Ref)
			}
			if err := requireNameable(output.Condition, c.id); err != nil {
				return nil, nil, err
			}
			newConditions = append(newConditions, loadedEffect{
				ref:      *output.Condition.Ref(),
				behavior: output.Condition,
			})
		}
	}

	return newFeatures, newConditions, nil
}

// attachGranted puts this level's features and conditions on the bus the sheet
// is holding, and reports what went on so a later teardown can take it off.
//
// A sheet with no bus attaches nothing: effects are inert until something puts
// them on a bus (rpg-toolkit#842), and the conditions are handed to
// [Character.Advance]'s commit to wait in pendingEffects for the next
// [Attach], exactly where a loaded sheet's conditions wait.
//
// A failure here takes back everything this call put on, newest first, so the
// caller sees a character that never advanced rather than one carrying half a
// level's subscriptions.
func (c *Character) attachGranted(
	ctx context.Context, newFeatures []features.Feature, newConditions []loadedEffect,
) ([]attachedFeature, error) {
	if c.bus == nil {
		return nil, nil
	}

	attached := make([]attachedFeature, 0, len(newFeatures))
	applied := make([]attachedEffect, 0, len(newConditions))

	rollback := func() {
		for i := len(applied) - 1; i >= 0; i-- {
			_ = applied[i].effect.behavior.Remove(ctx, applied[i].bus)
		}
		for i := len(attached) - 1; i >= 0; i-- {
			_ = attached[i].lifecycle.Remove(ctx, attached[i].bus)
		}
	}

	for _, feature := range newFeatures {
		lifecycle, ok := feature.(featureLifecycle)
		if !ok {
			continue
		}

		featureBus := dnd5eEvents.BusForEffect(c.bus, *feature.Ref())
		if err := lifecycle.Apply(ctx, featureBus); err != nil {
			_ = lifecycle.Remove(ctx, featureBus)
			rollback()
			return nil, rpgerr.Wrapf(err, "failed to apply feature %s", feature.Ref().String())
		}
		attached = append(attached, attachedFeature{lifecycle: lifecycle, bus: featureBus})
	}

	for _, effect := range newConditions {
		effectBus := dnd5eEvents.BusForEffect(c.bus, effect.ref)
		if err := effect.behavior.Apply(ctx, effectBus); err != nil {
			_ = effect.behavior.Remove(ctx, effectBus)
			rollback()
			return nil, rpgerr.Wrapf(err, "failed to apply condition %s", effect.ref.String())
		}
		applied = append(applied, attachedEffect{effect: effect, bus: effectBus})
	}

	return attached, nil
}

// commitLevelInput is everything a committed level writes to the sheet, named
// rather than positional because eight parameters of which three are ints is a
// signature two of them can be swapped in silently.
type commitLevelInput struct {
	entry         LevelEntry
	newFeatures   []features.Feature
	newConditions []loadedEffect
	attached      []attachedFeature

	// knownCantrips and knownSpells are what this level's choices compiled to.
	knownCantrips []*core.Ref
	knownSpells   []*core.Ref

	// classLevel sizes what the class grants; characterLevel sizes hit dice.
	classLevel     int
	characterLevel int
}

// commitLevel performs every mutation this level makes. Nothing here can fail,
// which is what makes [Character.Advance] atomic: by the time it is called the
// only step with an undo has already succeeded.
//
// It returns the pools whose maximum moved, because that is knowable only here
// — the sizing happens against the pools as they were a statement earlier.
func (c *Character) commitLevel(input commitLevelInput) []ResourceChange {
	c.levels = append(c.levels, input.entry)

	c.features = append(c.features, input.newFeatures...)
	if len(input.attached) > 0 {
		keeper := c.SheetKeeper()
		keeper.attachedFeatures = append(keeper.attachedFeatures, input.attached...)
	}

	for _, effect := range input.newConditions {
		c.conditions = append(c.conditions, effect.behavior)
		if c.bus == nil {
			// Waiting for a bus, in the one place Attach looks.
			c.pendingEffects = append(c.pendingEffects, effect)
		}
	}

	// The level's choices reach the sheet, not just the record (R4.4c). A spell
	// chosen at a level-up used to be written into an append-only entry and
	// nowhere else: not the known list, not the cast menu, not the sheet.
	c.knownCantrips = append(c.knownCantrips, input.knownCantrips...)
	c.knownSpells = append(c.knownSpells, input.knownSpells...)

	changes := c.resizeClassResources(input.entry.ClassID, input.classLevel, input.characterLevel)

	c.maxHitPoints += input.entry.HitPointGain

	// A level raises the pool, and raises what is in it — but it never lifts a
	// character off zero. A character at zero hit points is dying, and quietly
	// standing them up while their death saves stay on the sheet would be
	// healing dressed as arithmetic.
	if c.hitPoints > 0 {
		c.hitPoints += input.entry.HitPointGain
	}

	// The pools and the hit points both moved, and a sheet that did not report
	// itself dirty would have the whole level discarded by the next write-back
	// (rpg-toolkit#1087).
	c.poolChanged()

	return changes
}

// resizeClassResources grows this character's class pools to what its new
// level grants, WITHOUT refilling what it already spent, and reports every
// maximum that moved.
//
// A level that raises a maximum hands over the difference unspent and leaves
// everything already spent spent: gaining a level gives you one more hit die,
// not a full night's rest. A pool the character does not have yet arrives
// full, which is the same thing said about a maximum that rose from zero.
func (c *Character) resizeClassResources(
	classID classes.Class, classLevel, characterLevel int,
) []ResourceChange {
	if c.resources == nil {
		c.resources = make(map[coreResources.ResourceKey]*combat.RecoverableResource)
	}

	changes := make([]ResourceChange, 0)

	for key, resized := range buildClassResources(c, classID, classLevel, characterLevel) {
		existing, had := c.resources[key]
		if !had {
			c.resources[key] = resized
			changes = append(changes, ResourceChange{Key: key, From: 0, To: resized.Maximum()})
			continue
		}

		gain := resized.Maximum() - existing.Maximum()
		if gain <= 0 {
			// Not a level this pool grows on. Leave the character's own pool
			// alone rather than replacing it with a fresh full one.
			continue
		}

		current := existing.Current() + gain
		if spend := resized.Maximum() - current; spend > 0 {
			_ = resized.Use(spend)
		}
		c.resources[key] = resized
		changes = append(changes, ResourceChange{
			Key: key, From: existing.Maximum(), To: resized.Maximum(),
		})
	}

	// Map iteration is unordered and this is a projection someone reads, so it
	// is sorted rather than left to vary between two runs of the same level.
	sort.Slice(changes, func(i, j int) bool { return changes[i].Key < changes[j].Key })

	return changes
}

// rollHitPointGain produces the hit points this level adds, inside the toolkit
// (R4.3), from the class hit die and the Constitution modifier.
//
// A level never costs hit points: a Constitution modifier worse than the die
// roll floors the gain at zero rather than shrinking the character.
func (c *Character) rollHitPointGain(ctx context.Context, input *AdvanceInput) (int, error) {
	classData := classes.GetData(input.ClassID)
	if classData == nil {
		return 0, rpgerr.Newf(rpgerr.CodeNotFound, "unknown class: %s", input.ClassID)
	}

	var rolled int
	switch input.HitPointMethod {
	case HitPointMethodAverage:
		// PHB p.15: the fixed value a class takes instead of rolling.
		rolled = classData.HitDice/2 + 1
	case HitPointMethodRolled:
		roller := input.Roller
		if roller == nil {
			roller = dice.NewRoller()
		}
		roll, err := roller.Roll(ctx, classData.HitDice)
		if err != nil {
			return 0, rpgerr.Wrapf(err, "failed to roll hit die d%d", classData.HitDice)
		}
		rolled = roll
	case HitPointMethodMax:
		// Refused by checkCanAdvance; named so the switch is exhaustive.
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"hit point method %q is level 1 only", input.HitPointMethod)
	default:
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"unknown hit point method %q", input.HitPointMethod)
	}

	gain := rolled + c.GetAbilityModifier(abilities.CON)
	if gain < 0 {
		gain = 0
	}
	return gain, nil
}

// cloneChoices copies the choices an entry records, so the caller's slice and
// the character's history are not the same memory.
func cloneChoices(supplied []choices.ChoiceData) []choices.ChoiceData {
	if len(supplied) == 0 {
		return nil
	}
	out := make([]choices.ChoiceData, len(supplied))
	copy(out, supplied)
	return out
}

// grantedFeatureRefs names each feature for the derived display output.
func grantedFeatureRefs(list []features.Feature) []core.Ref {
	refs := make([]core.Ref, 0, len(list))
	for _, feature := range list {
		refs = append(refs, *feature.Ref())
	}
	return refs
}

// grantedConditionRefs names each condition for the derived display output.
func grantedConditionRefs(list []loadedEffect) []core.Ref {
	refs := make([]core.Ref, 0, len(list))
	for _, effect := range list {
		refs = append(refs, effect.ref)
	}
	return refs
}
