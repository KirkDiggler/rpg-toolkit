// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"errors"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// levelup_fixtures_test.go holds the between-runs cast: sheets that have
// earned a level and a Manager wired with nothing but the two capabilities the
// level-up verbs actually use.
//
// The sheets are written directly rather than finalized through a draft, the
// way every other suite here builds its cast. What matters about them is the
// level record, the experience total and what they already know — the three
// things the level reads — and a draft flow would decide two of those for
// them.
//
// EVERY POOL IS SEEDED AT ITS LEVEL-1 MAXIMUM, which is the part worth saying
// out loud: a fixture with an empty Resources map reports every change as
// 0 → N, and 0 → N is also what a pool the character never had reports. Seeded,
// the hit dice move 1 → 2 and the bard's slots 2 → 3, so an assertion about a
// TRANSITION can only pass if the level really moved the maximum.

// levelUpXP is the 2014 table's threshold for level 2 (PHB p.15). Named
// because every fixture below sits exactly on it and every "not yet earned"
// row sits below it.
const levelUpXP = 300

// advancingFighter is a level-1 fighter who has earned level 2.
//
// CONSTITUTION 14 on purpose: the modifier is +2, so an average d10 level
// gains 6 + 2 = 8 and a rolled one gains the die plus 2. A 10 or a 12 would
// make the modifier 0 or +1, and a gain that matched the die alone could not
// tell "the Constitution was folded in" from "it was ignored".
func advancingFighter(id string) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: "Ferrin", Level: 1,
		Levels:     syntheticLevels(classes.Fighter, 1),
		ClassID:    classes.Fighter,
		RaceID:     "human",
		Experience: levelUpXP,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 8,
		},
		HitPoints: 12, MaxHitPoints: 12, ArmorClass: 16, ProficiencyBonus: 2,
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice: {Current: 1, Maximum: 1, ResetType: coreResources.ResetLongRest},
		},
	}
}

// advancingBard is a level-1 bard who has earned level 2, holding the two
// cantrips and four spells a finalized level-1 bard has.
//
// The four spells are FOUR OF THE FIVE the class's level-1 list offers, which
// is what makes the level-2 row interesting: the class asks for one more spell
// from a list of five, and this character's own view of that row has exactly
// one option left on it.
func advancingBard(id string) *character.Data {
	bard := &character.Data{
		ID: id, PlayerID: "player-" + id, Name: "Belwyn", Level: 1,
		Levels:     syntheticLevels(classes.Bard, 1),
		ClassID:    classes.Bard,
		RaceID:     "human",
		Experience: levelUpXP,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 16,
		},
		HitPoints: 9, MaxHitPoints: 9, ArmorClass: 12, ProficiencyBonus: 2,
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice:         {Current: 1, Maximum: 1, ResetType: coreResources.ResetLongRest},
			resources.Inspiration:     {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
			resources.SpellSlotLevel1: {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
		},
	}
	bard.KnownCantrips = spellRefs(spells.ViciousMockery, spells.TrueStrike)
	bard.KnownSpells = spellRefs(spells.Bane, spells.Thunderwave, spells.DissonantWhispers, spells.Command)
	return bard
}

// advancingWizard is a level-1 wizard who has earned level 2 — the level a
// wizard picks its arcane tradition at, which is the kind of question this
// seam cannot ask yet.
func advancingWizard(id string) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: "Wren", Level: 1,
		Levels:     syntheticLevels(classes.Wizard, 1),
		ClassID:    classes.Wizard,
		RaceID:     "human",
		Experience: levelUpXP,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 12,
			abilities.INT: 16, abilities.WIS: 12, abilities.CHA: 10,
		},
		HitPoints: 7, MaxHitPoints: 7, ArmorClass: 12, ProficiencyBonus: 2,
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice: {Current: 1, Maximum: 1, ResetType: coreResources.ResetLongRest},
		},
	}
}

// advancingDruid is a level-3 druid who has earned level 4 — the first level
// that asks a druid for a cantrip, and the one whose option list this build
// cannot name in full.
//
// 2700 is the level-4 threshold. The sheet knows no cantrips, so nothing is
// removed from the row and the whole authored list is offered.
func advancingDruid(id string) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: "Dara", Level: 3,
		Levels:     syntheticLevels(classes.Druid, 3),
		ClassID:    classes.Druid,
		RaceID:     "human",
		Experience: 2700,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 12, abilities.WIS: 16, abilities.CHA: 8,
		},
		HitPoints: 21, MaxHitPoints: 21, ArmorClass: 13, ProficiencyBonus: 2,
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice: {Current: 3, Maximum: 3, ResetType: coreResources.ResetLongRest},
		},
	}
}

// spellRefs writes bare spell ids the way a finalized sheet stores them, as
// canonical refs. It panics on an id the catalog cannot name, so a fixture
// that names a spell wrong fails as a broken fixture rather than as a broken
// assertion.
func spellRefs(ids ...spells.Spell) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		ref := refs.Spells.ByID(string(id))
		if ref == nil {
			panic("the fixture named a spell the ref catalog does not have: " + id)
		}
		out = append(out, ref.String())
	}
	return out
}

// spellRef is one canonical ref, for the assertions and submissions that name
// a single spell.
func spellRef(id spells.Spell) string {
	return spellRefs(id)[0]
}

// advancementManager wires a Manager for the two between-runs verbs.
//
// The session and encounter repositories are empty and stay empty: neither
// verb opens a session, and a suite that had to build a world to take a level
// would be testing something this seam deliberately does not do.
func advancementManager(
	t fataler, roller session.Roller, sheets ...*character.Data,
) (*session.Manager, *fakeCharacters) {
	characters := newFakeCharacters(sheets...)
	return advancementManagerWith(t, roller, characters), characters
}

// advancementManagerWith is the same wiring over a character repository the
// caller chose, for the rows that need a store which answers reads and refuses
// writes.
func advancementManagerWith(
	t fataler, roller session.Roller, characters session.CharacterRepository,
) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Sessions:        newFakeSessions(),
		Encounters:      newFakeEncounters(),
		Characters:      characters,
		Events:          session.DiscardEvents{},
		Dice:            roller,
		PresentationIDs: testPresentationIDs{},
		TurnDriver:      session.Pass{},
	})
	if err != nil {
		t.Fatalf("wiring the advancement manager: %v", err)
	}
	return mgr
}

// refusingSaves reads like any store and cannot write, which is what a
// repository that has lost its backend looks like from inside a verb.
//
// Written as a wrapper rather than a flag on the shared fake, the way
// failingCharacters and failingEncounters already are here: a fake that grows
// a failure switch is a fake every other suite has to reason about.
type refusingSaves struct {
	*fakeCharacters
}

// errSaveRefused is what the broken store says, and the verb must keep it
// matchable: a host branches on its own repository's errors.
var errSaveRefused = errors.New("the character store refused the write")

func (r *refusingSaves) SaveCharacter(context.Context, *character.Data) error {
	return errSaveRefused
}
