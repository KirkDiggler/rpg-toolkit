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

// KnownSpellsSuite pins what a level-1 bard's spell CHOICES leave behind: two
// cantrips and four first-level spells, as content refs on the finished sheet.
//
// There is no Cast verb and no slot pool in this slice, so these are a record
// of what was chosen rather than a capability. That is exactly why they need a
// test: nothing else reads them yet, so nothing else would notice them going
// missing.
type KnownSpellsSuite struct {
	suite.Suite
	bus events.EventBus
}

func TestKnownSpellsSuite(t *testing.T) {
	suite.Run(t, new(KnownSpellsSuite))
}

func (s *KnownSpellsSuite) SetupTest() { s.bus = events.NewEventBus() }

// bardDraft builds a level-1 bard draft with the given spell choices.
func (s *KnownSpellsSuite) bardDraft(cantrips, spellList []spells.Spell) *Draft {
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
			Skills:   []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
			Tools:    []shared.SelectionID{"lute", "flute", "drum"},
			Cantrips: cantrips,
			Spells:   spellList,
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

// theBardsChoices is the selection every scene here starts from.
func theBardsChoices() ([]spells.Spell, []spells.Spell) {
	return []spells.Spell{spells.ViciousMockery, spells.MinorIllusion},
		[]spells.Spell{spells.CharmPerson, spells.CureWounds, spells.HealingWord, spells.Thunderwave}
}

// spellRefsAsStrings renders a known-spell list for comparison.
func spellRefsAsStrings(refList []*core.Ref) []string {
	out := make([]string, 0, len(refList))
	for _, ref := range refList {
		out = append(out, ref.String())
	}
	return out
}

// TestABardFinalizesCarryingWhatTheyChose is Kirk's walk blocker, from the
// other end: the draft completes, and the spells are on the sheet.
func (s *KnownSpellsSuite) TestABardFinalizesCarryingWhatTheyChose() {
	cantrips, spellList := theBardsChoices()

	char, err := s.bardDraft(cantrips, spellList).ToCharacter(context.Background(), "bard-1", s.bus)

	s.Require().NoError(err, "a bard who made every choice finalizes")
	s.Equal([]string{
		refs.Spells.ViciousMockery().String(),
		refs.Spells.MinorIllusion().String(),
	}, spellRefsAsStrings(char.KnownCantrips()))
	s.Equal([]string{
		refs.Spells.CharmPerson().String(),
		refs.Spells.CureWounds().String(),
		refs.Spells.HealingWord().String(),
		refs.Spells.Thunderwave().String(),
	}, spellRefsAsStrings(char.KnownSpells()))
}

// TestTheyAreRefsRatherThanNames — the sheet holds an identity for content it
// does not carry a copy of.
func (s *KnownSpellsSuite) TestTheyAreRefsRatherThanNames() {
	cantrips, spellList := theBardsChoices()
	char, err := s.bardDraft(cantrips, spellList).ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)

	for _, ref := range append(char.KnownCantrips(), char.KnownSpells()...) {
		s.Equal(refs.Module, ref.Module)
		s.Equal(refs.TypeSpells, ref.Type)
		s.Require().NoError(ref.IsValid())
	}
}

// TestNoSlotsCameWithThem is #397's R6 holding after the ruling change: the
// CHOICES land, the casting does not.
func (s *KnownSpellsSuite) TestNoSlotsCameWithThem() {
	cantrips, spellList := theBardsChoices()
	char, err := s.bardDraft(cantrips, spellList).ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)

	s.Empty(char.ToData().SpellSlots, "no slot pool in this slice")
}

// TestTheySurviveARoundTrip — written by ToData and read back by Load, which
// is the only way anything downstream ever sees them.
func (s *KnownSpellsSuite) TestTheySurviveARoundTrip() {
	cantrips, spellList := theBardsChoices()
	char, err := s.bardDraft(cantrips, spellList).ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)

	data := char.ToData()
	s.Len(data.KnownCantrips, 2)
	s.Len(data.KnownSpells, 4)
	s.Equal(refs.Spells.ViciousMockery().String(), data.KnownCantrips[0])

	reloaded, err := Load(context.Background(), data)
	s.Require().NoError(err)
	s.Equal(spellRefsAsStrings(char.KnownCantrips()), spellRefsAsStrings(reloaded.KnownCantrips()))
	s.Equal(spellRefsAsStrings(char.KnownSpells()), spellRefsAsStrings(reloaded.KnownSpells()))
}

// TestTheListIsCopiedOut — core.Ref is a mutable struct, and a caller must not
// be able to rewrite what a character knows from the outside.
func (s *KnownSpellsSuite) TestTheListIsCopiedOut() {
	cantrips, spellList := theBardsChoices()
	char, err := s.bardDraft(cantrips, spellList).ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)

	handed := char.KnownCantrips()
	handed[0].ID = "fireball"

	s.Equal(refs.Spells.ViciousMockery().String(), char.KnownCantrips()[0].String())
}

// TestTheWrongNumberOfCantripsIsRefused — the count is a rule, and the draft
// is where it is enforced.
func (s *KnownSpellsSuite) TestTheWrongNumberOfCantripsIsRefused() {
	_, spellList := theBardsChoices()

	one := s.bardDraft([]spells.Spell{spells.ViciousMockery}, spellList)
	s.Require().ErrorContains(one.ValidateChoices(), "2 cantrips")

	three := s.bardDraft([]spells.Spell{
		spells.ViciousMockery, spells.MinorIllusion, spells.Light,
	}, spellList)
	s.Require().ErrorContains(three.ValidateChoices(), "2 cantrips")
}

// TestTheWrongNumberOfSpellsIsRefused, from both sides.
func (s *KnownSpellsSuite) TestTheWrongNumberOfSpellsIsRefused() {
	cantrips, spellList := theBardsChoices()

	three := s.bardDraft(cantrips, spellList[:3])
	s.Require().ErrorContains(three.ValidateChoices(), "4 1st-level spells")

	five := s.bardDraft(cantrips, append(append([]spells.Spell{}, spellList...), spells.Sleep))
	s.Require().ErrorContains(five.ValidateChoices(), "4 1st-level spells")
}

// TestChoosingNoneIsRefused — an absent choice is not a satisfied one, which
// is the failure Kirk actually hit.
func (s *KnownSpellsSuite) TestChoosingNoneIsRefused() {
	_, spellList := theBardsChoices()
	s.Require().ErrorContains(s.bardDraft(nil, spellList).ValidateChoices(), "cantrips")

	cantrips, _ := theBardsChoices()
	s.Require().ErrorContains(s.bardDraft(cantrips, nil).ValidateChoices(), "spells")
}

// TestSomethingOffTheBardListIsRefused — the option list is the gate the
// compiler relies on, so it has to actually hold.
func (s *KnownSpellsSuite) TestSomethingOffTheBardListIsRefused() {
	_, spellList := theBardsChoices()

	draft := s.bardDraft([]spells.Spell{spells.ViciousMockery, spells.FireBolt}, spellList)

	s.Require().ErrorContains(draft.ValidateChoices(), "fire-bolt")
}

// TestASpellThisBuildHasNoRefForIsRefused pins the catalog gate. The option
// list is checked by the validator; this is the check that catches an id the
// requirement admitted but the ref catalog has never heard of, which is what
// would otherwise put a ref pointing at nothing onto a sheet.
func (s *KnownSpellsSuite) TestASpellThisBuildHasNoRefForIsRefused() {
	cantrips, spellList := theBardsChoices()
	draft := s.bardDraft(cantrips, spellList)

	// Reach past the validator by rewriting the recorded choice, because the
	// requirement's option list would never offer this in the first place.
	draft.recordChoice(choices.ChoiceData{
		Category:       shared.ChoiceCantrips,
		Source:         shared.SourceClass,
		ChoiceID:       choices.BardCantrips1,
		SpellSelection: []spells.Spell{"song-of-nothing"},
	})

	_, err := draft.compileKnownSpells(shared.ChoiceCantrips, "cantrip")

	s.Require().ErrorContains(err, "song-of-nothing")
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

// TestASheetNamingSomethingUnreadableIsRefused — fail closed. A character who
// quietly forgot a spell is a bug nobody could see.
func (s *KnownSpellsSuite) TestASheetNamingSomethingUnreadableIsRefused() {
	cantrips, spellList := theBardsChoices()
	char, err := s.bardDraft(cantrips, spellList).ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)

	data := char.ToData()
	data.KnownCantrips = []string{"not a ref at all"}
	_, err = Load(context.Background(), data)
	s.Require().Error(err)

	data = char.ToData()
	data.KnownSpells = []string{refs.Conditions.Inspired().String()}
	_, err = Load(context.Background(), data)
	s.Require().ErrorContains(err, "not a spell ref")
}
