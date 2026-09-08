package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// KnownSpellsSuite covers the sheet's known-spell fields, which in slice one
// are GROUNDWORK: the shape a known/prepared list will have
// (rpg-project#391 §5.2), carried and round-tripped, with nothing yet putting
// anything in it.
//
// A level-1 bard knows none, and that is the ruling rather than a gap — slice
// one is Bardic Inspiration alone, so the cantrip and spell questions are not
// asked at creation and come back with the cast door. What is tested here is
// therefore the machinery around an empty list, plus the two paths that WILL
// fill it: the compiler that turns a chosen id into a ref, and the loader that
// reads one back.
type KnownSpellsSuite struct {
	suite.Suite
	bus events.EventBus
}

func TestKnownSpellsSuite(t *testing.T) {
	suite.Run(t, new(KnownSpellsSuite))
}

func (s *KnownSpellsSuite) SetupTest() { s.bus = events.NewEventBus() }

// bardDraft builds a level-1 bard draft — skills, instruments and equipment,
// and no spell choices at all.
func (s *KnownSpellsSuite) bardDraft() *Draft {
	draft, err := NewDraft(&DraftConfig{ID: "draft-1", PlayerID: "player-1"})
	s.Require().NoError(err)

	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Scanlan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Bard,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
			Tools:  []shared.SelectionID{"lute", "flute", "drum"},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BardWeaponsPrimary, OptionID: choices.BardWeaponRapier},
				{ChoiceID: choices.BardPack, OptionID: choices.BardPackDiplomat},
				{ChoiceID: choices.BardInstrument, OptionID: choices.BardInstrumentLute},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 13,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 16,
		},
		Method: "standard-array",
	}))
	return draft
}

// spellRefsAsStrings renders a known-spell list for comparison.
func spellRefsAsStrings(refList []*core.Ref) []string {
	out := make([]string, 0, len(refList))
	for _, ref := range refList {
		out = append(out, ref.String())
	}
	return out
}

// TestALevelOneBardFinalizesWithoutBeingAskedForSpells is the walk blocker,
// fixed at the source. The draft completes on skills, instruments and
// equipment, and the sheet says the bard knows no spells because the bard was
// never asked for any.
func (s *KnownSpellsSuite) TestALevelOneBardFinalizesWithoutBeingAskedForSpells() {
	draft := s.bardDraft()

	s.Require().NoError(draft.ValidateChoices(), "nothing further is required of a slice-one bard")

	char, err := draft.ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)

	s.Empty(char.KnownCantrips())
	s.Empty(char.KnownSpells())
	s.Empty(char.ToData().SpellSlots, "and no slot pool either")
}

// TestTheBardIsAskedNothingAboutSpells pins the ruling itself: slice one asks
// for Bardic Inspiration and nothing that only casting could use.
func (s *KnownSpellsSuite) TestTheBardIsAskedNothingAboutSpells() {
	requirements := choices.GetClassRequirements(classes.Bard)

	s.Require().NotNil(requirements)
	s.Nil(requirements.Cantrips, "no cantrip question until there is something to cast")
	s.Nil(requirements.Spellbook, "and no spell question either")
	s.NotNil(requirements.Skills, "the questions slice one does ask")
	s.NotNil(requirements.Tools)
}

// TestAFighterKnowsNothing — the fields are absent rather than empty on a
// character who was never asked, so "knows nothing" and "does not cast" read
// the same way they always did.
func (s *KnownSpellsSuite) TestAFighterKnowsNothing() {
	draft, err := NewDraft(&DraftConfig{ID: "draft-2", PlayerID: "player-2"})
	s.Require().NoError(err)
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Grog"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills:        []skills.Skill{skills.Athletics, skills.Intimidation},
			FightingStyle: "defense",
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 15,
			abilities.INT: 8, abilities.WIS: 12, abilities.CHA: 10,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(context.Background(), "fighter-1", s.bus)
	s.Require().NoError(err, "a fighter is unaffected by any of this")

	s.Nil(char.KnownCantrips())
	s.Nil(char.KnownSpells())
	data := char.ToData()
	s.Nil(data.KnownCantrips)
	s.Nil(data.KnownSpells)
}

// TestTheCompilerTurnsAChosenIdIntoARef exercises the path rung 2 will use,
// without a requirement offering anything: the recorded choice is compiled
// straight, which is what the finalize step does with it.
func (s *KnownSpellsSuite) TestTheCompilerTurnsAChosenIdIntoARef() {
	draft := s.bardDraft()
	draft.recordChoice(choices.ChoiceData{
		Category:       shared.ChoiceCantrips,
		Source:         shared.SourceClass,
		SpellSelection: []spells.Spell{spells.ViciousMockery, spells.MinorIllusion},
	})

	known, err := draft.compileKnownSpells(shared.ChoiceCantrips, "cantrip")

	s.Require().NoError(err)
	s.Equal([]string{
		refs.Spells.ViciousMockery().String(),
		refs.Spells.MinorIllusion().String(),
	}, spellRefsAsStrings(known))
}

// TestTheCompiledRefIsNotTheCatalogsOwn — the catalog hands back shared
// singletons, and a sheet that aliased one would let a caller reading its
// known spells rewrite the catalog for everybody.
func (s *KnownSpellsSuite) TestTheCompiledRefIsNotTheCatalogsOwn() {
	draft := s.bardDraft()
	draft.recordChoice(choices.ChoiceData{
		Category:       shared.ChoiceCantrips,
		Source:         shared.SourceClass,
		SpellSelection: []spells.Spell{spells.ViciousMockery},
	})

	known, err := draft.compileKnownSpells(shared.ChoiceCantrips, "cantrip")
	s.Require().NoError(err)
	s.Require().Len(known, 1)

	s.NotSame(refs.Spells.ViciousMockery(), known[0])
	known[0].ID = "fireball"
	s.Equal("vicious-mockery", refs.Spells.ViciousMockery().ID)
}

// TestASpellThisBuildHasNoRefForIsRefused pins the catalog gate. Composing
// "dnd5e:spells:<id>" out of a chosen string would always succeed, which is
// the problem: a typo would become a ref pointing at nothing, persisted, and
// read back later by whatever mints Cast declarations.
func (s *KnownSpellsSuite) TestASpellThisBuildHasNoRefForIsRefused() {
	draft := s.bardDraft()
	draft.recordChoice(choices.ChoiceData{
		Category:       shared.ChoiceCantrips,
		Source:         shared.SourceClass,
		SpellSelection: []spells.Spell{"song-of-nothing"},
	})

	_, err := draft.compileKnownSpells(shared.ChoiceCantrips, "cantrip")

	s.Require().ErrorContains(err, "song-of-nothing")
}

// knownSheet is a stored sheet carrying known spells directly — the shape a
// caster's sheet will have, built here without a draft because nothing in
// slice one produces one.
func (s *KnownSpellsSuite) knownSheet() *Data {
	char, err := s.bardDraft().ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)

	data := char.ToData()
	data.KnownCantrips = []string{
		refs.Spells.ViciousMockery().String(), refs.Spells.MinorIllusion().String(),
	}
	data.KnownSpells = []string{
		refs.Spells.CharmPerson().String(), refs.Spells.HealingWord().String(),
	}
	return data
}

// TestTheySurviveARoundTrip — written by ToData and read back by Load, which
// is the only way anything downstream will ever see them.
func (s *KnownSpellsSuite) TestTheySurviveARoundTrip() {
	loaded, err := Load(context.Background(), s.knownSheet())
	s.Require().NoError(err)

	s.Equal([]string{
		refs.Spells.ViciousMockery().String(), refs.Spells.MinorIllusion().String(),
	}, spellRefsAsStrings(loaded.KnownCantrips()))
	s.Equal([]string{
		refs.Spells.CharmPerson().String(), refs.Spells.HealingWord().String(),
	}, spellRefsAsStrings(loaded.KnownSpells()))

	back := loaded.ToData()
	s.Equal(s.knownSheet().KnownCantrips, back.KnownCantrips)
	s.Equal(s.knownSheet().KnownSpells, back.KnownSpells)
}

// TestTheyAreRefsRatherThanNames — the sheet holds an identity for content it
// does not carry a copy of.
func (s *KnownSpellsSuite) TestTheyAreRefsRatherThanNames() {
	loaded, err := Load(context.Background(), s.knownSheet())
	s.Require().NoError(err)

	for _, ref := range append(loaded.KnownCantrips(), loaded.KnownSpells()...) {
		s.Equal(refs.Module, ref.Module)
		s.Equal(refs.TypeSpells, ref.Type)
		s.Require().NoError(ref.IsValid())
	}
}

// TestTheListIsCopiedOut — core.Ref is a mutable struct, and a caller must not
// be able to rewrite what a character knows from the outside.
func (s *KnownSpellsSuite) TestTheListIsCopiedOut() {
	loaded, err := Load(context.Background(), s.knownSheet())
	s.Require().NoError(err)

	handed := loaded.KnownCantrips()
	handed[0].ID = "fireball"

	s.Equal(refs.Spells.ViciousMockery().String(), loaded.KnownCantrips()[0].String())
}

// TestASheetNamingSomethingUnreadableIsRefused — fail closed. A character who
// quietly forgot a spell is a bug nobody could see.
func (s *KnownSpellsSuite) TestASheetNamingSomethingUnreadableIsRefused() {
	data := s.knownSheet()
	data.KnownCantrips = []string{"not a ref at all"}
	_, err := Load(context.Background(), data)
	s.Require().Error(err)

	data = s.knownSheet()
	data.KnownSpells = []string{refs.Conditions.Inspired().String()}
	_, err = Load(context.Background(), data)
	s.Require().ErrorContains(err, "not a spell ref")
}
