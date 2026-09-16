// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// next_level.go is the read half of taking a level, and the reason both halves
// live in this package at all is a ruling: *"the API is dumb … we added the
// session package to act as the SDK to the API. So we should not need an
// orchestrator in API anymore and our level up should be contained in our
// session package."* (Kirk, 2026-09-16; level-up design §6, R6.1 and R6.2.)
// The first build put "load, Advance, save" in the host's orchestrator, which
// made the host hold the one thing a seam exists to absorb — the ordering.
//
// THIS VERB IS NOT SEATED. Every other verb on this Manager opens a session
// first; a level is taken between runs, so this one takes a character id and
// nothing else. That is why it has no Session field and no writeScope: there
// is no world to freeze and no story to tell.
//
// IT ALSO DECIDES NOTHING, which is the charter test for a seam. It projects
// what the sheet says and what the class tables say, side by side, and the
// comparison that means "a level is waiting" — EntitledLevel above Level — is
// the host's to make. A boolean here would be a rule, and this is the package
// that owns no rules.

// NextLevel reports the level this character would take next: what its sheet
// says about its right to take one, what the level grants, and what it asks.
//
// It loads the stored sheet and puts nothing on a bus — character.Load is
// bus-free, and reconstituting a sheet onto a bus is resolution's job, never
// this package's (entities.go's "THERE IS NO BUS HERE").
//
// A requirement kind this seam cannot offer — a subclass at wizard or druid 2,
// today the only one a level above 1 produces — is REFUSED here with
// [ErrLevelNotOffered] rather than dropped, and so is a spell option the ref
// catalog cannot name. Refusing at the read is what keeps a client from being
// shown a confirmation [Manager.LevelUp] would go on to refuse.
//
// Returns ErrNilInput, ErrNoMemberID (empty character), ErrNoCharacter /
// ErrBadCharacter / ErrBadRepository (loading the stored sheet),
// ErrUnknownContent (a class with no table) or ErrLevelNotOffered.
func (m *Manager) NextLevel(ctx context.Context, in *NextLevelInput) (*NextLevelOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("next level: %w", ErrNilInput)
	}
	if in.Character == "" {
		return nil, fmt.Errorf("next level: %w", ErrNoMemberID)
	}

	data, err := m.fetchCharacterData(ctx, "character", in.Character)
	if err != nil {
		return nil, fmt.Errorf("next level: %w", err)
	}

	sheet, err := character.Load(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("next level: character %q: %w: %v", in.Character, ErrBadCharacter, err)
	}

	classID := data.ClassID
	classData := classes.GetData(classID)
	if classData == nil {
		return nil, fmt.Errorf(
			"next level: character %q: class %q has no table: %w", in.Character, classID, ErrUnknownContent)
	}

	// The two arithmetics are separate on purpose. They agree while
	// multiclassing is shut (R2.4) and the day it opens they stop agreeing,
	// so a caller reading one for the other would be right until it was
	// silently wrong.
	classLevel := sheet.ClassLevel(classID) + 1
	characterLevel := sheet.GetLevel() + 1

	asked, err := levelChoicesOf(sheet.NextLevelRequirements())
	if err != nil {
		return nil, fmt.Errorf(
			"next level: character %q taking %s level %d: %w", in.Character, classID, classLevel, err)
	}

	return &NextLevelOutput{
		Character:      in.Character,
		Class:          string(classID),
		Level:          sheet.GetLevel(),
		Experience:     sheet.Experience(),
		EntitledLevel:  sheet.EntitledLevel(),
		NextThreshold:  sheet.NextLevelThreshold(),
		CharacterLevel: characterLevel,
		ClassLevel:     classLevel,
		HitDie:         classData.HitDice,
		Features:       grantedFeatureRefs(classID, classLevel),
		Choices:        asked,
	}, nil
}

// levelChoicesOf projects a level's requirement row into this package's own
// vocabulary, refusing anything it cannot say.
//
// ONE projection, read by BOTH verbs: [Manager.NextLevel] returns it and
// [Manager.LevelUp] reads the kind of each id back off it. That is not thrift
// — it is the guarantee that what a client is offered is exactly what the
// write accepts. Two walks of the same row would be free to drift, and the
// drift would show up as a confirmed level-up that failed.
//
// The completeness check is [choices.Requirements.ChoiceIDs], which the
// rulebook keeps as the one list precisely so a caller comparing rows "does
// not silently miss a kind added later". Anything in it this function did not
// translate is refused, so a twelfth requirement kind arriving in the rulebook
// fails loudly here rather than vanishing from a player's menu.
func levelChoicesOf(reqs *choices.Requirements) ([]LevelChoice, error) {
	if reqs == nil {
		return nil, nil
	}

	asked := make([]LevelChoice, 0, 2)
	translated := make(map[choices.ChoiceID]struct{}, 2)

	if reqs.Cantrips != nil {
		options, err := spellRefOptions(reqs.Cantrips.Options)
		if err != nil {
			return nil, err
		}
		asked = append(asked, LevelChoice{
			ID:      string(reqs.Cantrips.ID),
			Kind:    LevelChoiceCantrip,
			Count:   reqs.Cantrips.Count,
			Options: options,
		})
		translated[reqs.Cantrips.ID] = struct{}{}
	}

	if reqs.Spellbook != nil {
		options, err := spellRefOptions(reqs.Spellbook.Options)
		if err != nil {
			return nil, err
		}
		asked = append(asked, LevelChoice{
			ID:         string(reqs.Spellbook.ID),
			Kind:       LevelChoiceSpell,
			Count:      reqs.Spellbook.Count,
			SpellLevel: reqs.Spellbook.SpellLevel,
			Options:    options,
		})
		translated[reqs.Spellbook.ID] = struct{}{}
	}

	for _, id := range reqs.ChoiceIDs() {
		if _, ok := translated[id]; ok {
			continue
		}
		return nil, fmt.Errorf(
			"%w: this level requires choosing a %s, which this seam cannot offer yet",
			ErrLevelNotOffered, requirementKindOf(reqs, id))
	}

	return asked, nil
}

// requirementKindOf names the kind of requirement an id belongs to, so a
// refusal says "subclass" rather than an identifier nobody outside the
// rulebook can read.
//
// The fallback is the id itself, and it is the honest answer for a kind added
// to the rulebook after this switch was written: the refusal still fires —
// ChoiceIDs is what fires it — and still says which question it could not ask,
// which is enough to find the row. A default that guessed a name would be the
// only part of the refusal that could be wrong.
func requirementKindOf(reqs *choices.Requirements, id choices.ChoiceID) string {
	switch {
	case reqs.Skills != nil && reqs.Skills.ID == id:
		return "skill"
	case reqs.Tools != nil && reqs.Tools.ID == id:
		return "tool proficiency"
	case reqs.FightingStyle != nil && reqs.FightingStyle.ID == id:
		return "fighting style"
	case reqs.Expertise != nil && reqs.Expertise.ID == id:
		return "expertise"
	case reqs.Subclass != nil && reqs.Subclass.ID == id:
		return "subclass"
	}

	for _, req := range reqs.AdditionalSkills {
		if req != nil && req.ID == id {
			return "skill"
		}
	}
	for _, req := range reqs.Equipment {
		if req != nil && req.ID == id {
			return "piece of equipment"
		}
	}
	for _, req := range reqs.EquipmentCategories {
		if req != nil && req.ID == id {
			return "piece of equipment"
		}
	}
	for _, req := range reqs.Languages {
		if req != nil && req.ID == id {
			return "language"
		}
	}

	return fmt.Sprintf("%q", id)
}

// spellRefOptions turns the rulebook's bare spell ids into the canonical refs
// this seam trades in, refusing any the catalog cannot name.
//
// A REF IS CONSTRUCTED HERE, and that is why an unnameable option is refused
// rather than carried: refs.Spells.ByID is the catalog, and composing
// "dnd5e:spells:" + id by hand would mint a ref that cannot be wrong and
// points at nothing — the exact failure the catalog's own doc names. Dropping
// the option instead would leave a menu whose missing row is indistinguishable
// from a row that never existed, and would silently make a choice whose count
// equals its option list unanswerable.
//
// Contrast [grantedFeatureRefs], which does not validate: a granted feature
// arrives as an authored ref STRING and is carried, not built.
func spellRefOptions(options []spells.Spell) ([]string, error) {
	if len(options) == 0 {
		return nil, nil
	}

	out := make([]string, 0, len(options))
	for _, option := range options {
		ref := refs.Spells.ByID(string(option))
		if ref == nil {
			return nil, fmt.Errorf(
				"%w: spell %q is on this level's list and has no ref in the catalog", ErrLevelNotOffered, option)
		}
		out = append(out, ref.String())
	}

	return out, nil
}

// grantedFeatureRefs names what a class level grants, as the canonical ref
// strings its authored rows already carry.
func grantedFeatureRefs(classID classes.Class, classLevel int) []string {
	grants := classes.GetGrantsGainedAtLevel(classID, classLevel)
	if len(grants) == 0 {
		return nil
	}

	granted := make([]string, 0, len(grants))
	for _, grant := range grants {
		for _, feature := range grant.Features {
			granted = append(granted, feature.Ref)
		}
	}
	if len(granted) == 0 {
		return nil
	}

	return granted
}
