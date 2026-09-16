// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// level_up.go is the write half of [Manager.NextLevel] and the narrowest
// write verb in the package: ONE aggregate, no world, no story.
//
// It has no writeScope and calls no commit, and that is a fact about the verb
// rather than a shortcut. A scope exists to keep an encounter, a session and a
// set of sheets consistent across one call and to hold the freeze gate; a
// level touches one sheet between runs, so there is nothing to keep consistent
// with it and no window that could be open. What it still owes the host is the
// report — [SaveReport] out on success, a *[SaveError] on failure — because S6
// is about telling a caller what landed, not about how many things did.
//
// EVERY RULE BELOW IS THE RULEBOOK'S. What a level grants, what it may ask,
// how many hit points a die and a Constitution produce, and whether the
// experience has been earned are all [character.Character.Advance]'s. This
// file loads, translates in both directions, saves, and reports.

// LevelUp takes the level this character has earned, appends it to the sheet's
// own level record, and saves the sheet.
//
// The character's class is read off the sheet rather than taken from the
// caller: multiclassing is not open (R2.4), so a class parameter here could
// hold exactly one correct value and could be got wrong with nothing to refuse
// it. The day that seam opens, this input gains the class, the way the
// rulebook's own AdvanceInput already states it.
//
// **This seam cannot tell whether the character is seated in a run.** The
// rulebook refuses a level to a sheet that is [character.Character.InCombat],
// which is what the sheet itself can answer — it holds a live action economy
// from its first turn in a fight until a session clears it. A character seated
// in an encounter that has not reached its first turn is invisible to that
// check, and it is invisible HERE too: this Manager has no index from a
// character id to the sessions holding it, only the reverse (a session names
// one encounter, and an encounter names its members). THAT INDEX IS THE SEAM.
// It is not built here, and until it exists this verb's in-combat refusal is
// exactly the sheet's, no narrower and no wider.
//
// Returns ErrNilInput, ErrNoMemberID (empty character), ErrBadLevelRequest (an
// unknown hit point method, a choice the level did not ask for, a selection
// that is not a canonical spell ref, or anything the rulebook calls an invalid
// argument), ErrLevelNotOffered (a requirement kind this seam cannot
// translate), ErrCannotAdvance (not enough experience, in a fight, nothing to
// advance from), ErrNoCharacter / ErrBadCharacter / ErrBadRepository (loading
// the stored sheet), or ErrSaveFailed with a populated report.
func (m *Manager) LevelUp(ctx context.Context, in *LevelUpInput) (*LevelUpOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("level up: %w", ErrNilInput)
	}
	if in.Character == "" {
		return nil, fmt.Errorf("level up: %w", ErrNoMemberID)
	}

	// Before any I/O, the same "reject cheap" pattern Unpack's catalog lookup
	// and Trade's price check already keep: a method nobody offers is a shape
	// refusal, and reading a sheet to discover it would be work spent on a
	// call that was never going to land.
	method, err := advanceHitPointMethod(in.HitPointMethod)
	if err != nil {
		return nil, fmt.Errorf("level up: %w", err)
	}

	data, err := m.fetchCharacterData(ctx, "character", in.Character)
	if err != nil {
		return nil, fmt.Errorf("level up: %w", err)
	}

	sheet, err := character.Load(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("level up: character %q: %w: %v", in.Character, ErrBadCharacter, err)
	}

	submitted, err := levelChoiceData(sheet.NextLevelRequirements(), in.Choices)
	if err != nil {
		return nil, fmt.Errorf("level up: character %q: %w", in.Character, err)
	}

	advanced, err := sheet.Advance(ctx, &character.AdvanceInput{
		ClassID:        data.ClassID,
		HitPointMethod: method,
		Choices:        submitted,
		Roller:         &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("level up: character %q: %w", in.Character, translateAdvance(err))
	}

	record := sheet.ToData()
	aggregate := "character:" + record.ID
	if err := m.characters.SaveCharacter(ctx, record); err != nil {
		return nil, fmt.Errorf("level up: %w", &SaveError{
			Report: SaveReport{Failed: []string{aggregate}},
			Err:    fmt.Errorf("saving character: %w", err),
		})
	}

	return &LevelUpOutput{
		Saved:  SaveReport{Written: []string{aggregate}},
		Gained: levelGained(data.ClassID, advanced.Gained),
	}, nil
}

// advanceHitPointMethod maps this package's method onto the rulebook's.
//
// The rulebook has a third, HitPointMethodMax, which is level 1 only and which
// it refuses for any later level. This seam does not offer it at all: an enum
// that carries a value every call rejects invites a host to wire it, and the
// refusal is cheaper and clearer one layer up, before a sheet has been read.
func advanceHitPointMethod(method LevelUpHitPointMethod) (character.HitPointMethod, error) {
	switch method {
	case HitPointsRolled:
		return character.HitPointMethodRolled, nil
	case HitPointsAverage:
		return character.HitPointMethodAverage, nil
	default:
		return "", fmt.Errorf(
			"%w: hit point method %q is not one this verb offers (%q or %q)",
			ErrBadLevelRequest, method, HitPointsRolled, HitPointsAverage)
	}
}

// levelChoiceData translates the host's submissions into the rulebook's
// choice vocabulary, using the KIND EACH REQUIREMENT CARRIES.
//
// The client never sends a category, which is the point: it names a choice and
// its selections, and what kind of question that id was is read back off the
// level's own row. A client that could name the category could name the wrong
// one, and a level whose question changed kind would need every client to
// change with it.
//
// The row is projected through [levelChoicesOf], the same function
// [Manager.NextLevel] returns, so a submission is measured against exactly
// what the read offered — including its refusal of a kind this seam cannot
// translate, which therefore fires on the write too even for a caller that
// never read.
func levelChoiceData(
	required *choices.Requirements, submissions []LevelChoiceSubmission,
) ([]choices.ChoiceData, error) {
	asked, err := levelChoicesOf(required)
	if err != nil {
		return nil, err
	}

	if len(submissions) == 0 {
		// Not an error here. A level that asks nothing takes nothing, and a
		// level that asks something and got nothing is the rulebook's refusal
		// to make — it is the one that knows a count was unmet.
		return nil, nil
	}

	byID := make(map[string]LevelChoice, len(asked))
	for _, choice := range asked {
		byID[choice.ID] = choice
	}

	translated := make([]choices.ChoiceData, 0, len(submissions))
	for _, submission := range submissions {
		choice, ok := byID[submission.ChoiceID]
		if !ok {
			return nil, fmt.Errorf(
				"%w: this level did not ask for choice %q", ErrBadLevelRequest, submission.ChoiceID)
		}

		category, err := choiceCategoryOf(choice.Kind)
		if err != nil {
			return nil, err
		}

		selected, err := selectedSpells(submission)
		if err != nil {
			return nil, err
		}

		translated = append(translated, choices.ChoiceData{
			Category:       category,
			Source:         shared.SourceClass,
			ChoiceID:       choices.ChoiceID(submission.ChoiceID),
			SpellSelection: selected,
		})
	}

	return translated, nil
}

// choiceCategoryOf maps a projected kind onto the rulebook's category.
//
// The default arm is unreachable through [levelChoicesOf], which mints only
// the two kinds below — and it is here for the day it mints a third: a new
// kind whose translation nobody wrote is refused rather than silently sent
// down as a spell selection.
func choiceCategoryOf(kind LevelChoiceKind) (shared.ChoiceCategory, error) {
	switch kind {
	case LevelChoiceCantrip:
		return shared.ChoiceCantrips, nil
	case LevelChoiceSpell:
		return shared.ChoiceSpells, nil
	default:
		return "", fmt.Errorf(
			"%w: choices of kind %q cannot be submitted through this seam yet", ErrLevelNotOffered, kind)
	}
}

// selectedSpells reads a submission's canonical refs back as the bare spell ids
// the rulebook's choice vocabulary uses.
//
// The rulebook refuses an unknown spell too, and this refuses it FIRST on
// purpose: by the time Advance sees a selection it has already been folded
// into a category and a choice id, and the message that comes back names the
// validation that failed rather than the string the host actually sent.
func selectedSpells(submission LevelChoiceSubmission) ([]spells.Spell, error) {
	if len(submission.Selections) == 0 {
		return nil, nil
	}

	selected := make([]spells.Spell, 0, len(submission.Selections))
	for _, selection := range submission.Selections {
		parsed, err := core.ParseString(selection)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: choice %q selected %q, which is not a ref: %v",
				ErrBadLevelRequest, submission.ChoiceID, selection, err)
		}
		if parsed.Module != refs.Module || parsed.Type != refs.TypeSpells {
			return nil, fmt.Errorf(
				"%w: choice %q selected %q, which does not name a spell",
				ErrBadLevelRequest, submission.ChoiceID, selection)
		}
		if refs.Spells.ByID(string(parsed.ID)) == nil {
			return nil, fmt.Errorf(
				"%w: choice %q selected %q, which names no spell this build carries",
				ErrBadLevelRequest, submission.ChoiceID, selection)
		}
		selected = append(selected, spells.Spell(parsed.ID))
	}

	return selected, nil
}

// translateAdvance maps the rulebook's coded refusals onto this package's own
// sentinels.
//
// The same argument translateResolution makes for the composition's sentinels,
// one module over: a host matching on rpgerr codes would be coupled to a
// package this seam exists to keep replaceable (S2). The rulebook's own words
// ride along as TEXT — "%v", never "%w" — so the refusal a player reads still
// names the spell or the experience total, while the only thing a host can
// branch on is this package's vocabulary.
//
// An error with no code this switch knows passes through UNCHANGED, for the
// reason translateResolution's default arm does: a failing host Roller reaches
// the hit point roll and comes back out through here, and flattening the
// host's own error to protect it from us would break its matching on it.
func translateAdvance(err error) error {
	switch rpgerr.GetCode(err) {
	case rpgerr.CodeInvalidArgument:
		// The request was wrong: a choice nobody asked for, a count unmet, a
		// spell already known, a hit point method for level 1 only.
		return fmt.Errorf("%w: %v", ErrBadLevelRequest, err)
	case rpgerr.CodePrerequisiteNotMet, rpgerr.CodeTimingRestriction,
		rpgerr.CodeNotAllowed, rpgerr.CodeInvalidState:
		// The request was fine and the answer is no: the experience is not
		// earned yet, the character is in a fight, the level names another
		// class, the record cannot be advanced from. Each of these clears on
		// its own, which is why they are one sentinel and not four.
		return fmt.Errorf("%w: %v", ErrCannotAdvance, err)
	default:
		return err
	}
}

// levelGained projects the rulebook's account of a level into this package's.
//
// The class is passed in rather than read off the account: GainedAtLevel says
// what a level added and not which class it went into, and a host that wrote
// without reading first has nothing else to learn it from.
func levelGained(classID classes.Class, gained character.GainedAtLevel) LevelGained {
	out := LevelGained{
		CharacterLevel:   gained.CharacterLevel,
		ClassLevel:       gained.ClassLevel,
		HitPointGain:     gained.HitPointGain,
		ProficiencyBonus: gained.ProficiencyBonus,
		Class:            classRef(classID),
		ClassName:        classes.Name(classID),
	}

	if len(gained.Features) > 0 {
		out.Features = make([]string, 0, len(gained.Features))
		for _, feature := range gained.Features {
			out.Features = append(out.Features, feature.String())
		}
	}

	if len(gained.Resources) > 0 {
		out.Resources = make([]ResourceMaximumChange, 0, len(gained.Resources))
		for _, change := range gained.Resources {
			out.Resources = append(out.Resources, ResourceMaximumChange{
				Key:  string(change.Key),
				Name: resourceDisplayName(change.Key),
				From: change.From,
				To:   change.To,
			})
		}
	}

	return out
}

// resourceDisplayName names a pool, falling back to its own key.
//
// A pool the rulebook's display table has never heard of still ships, named by
// its key. Dropping it would be the worse answer: a level that moved a
// maximum and reported nothing reads exactly like a level that moved nothing.
func resourceDisplayName(key coreResources.ResourceKey) string {
	if name, ok := resources.DisplayName(key); ok {
		return name
	}
	return string(key)
}
