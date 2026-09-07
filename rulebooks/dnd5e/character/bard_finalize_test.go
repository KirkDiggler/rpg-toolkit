package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// BardFinalizeSuite is rpg-project#397's "a bard finalizes" done-when: the
// feature is on the sheet, the pool is its Charisma modifier, and no spell
// slots are on it (R6).
type BardFinalizeSuite struct {
	suite.Suite
	bus events.EventBus
}

func (s *BardFinalizeSuite) SetupTest() { s.bus = events.NewEventBus() }

// finalize builds a level-1 bard with the given Charisma.
func (s *BardFinalizeSuite) finalize(charisma int) *Character {
	draft := s.bardDraft(charisma, []shared.SelectionID{"lute", "flute", "drum"})
	char, err := draft.ToCharacter(context.Background(), "bard-1", s.bus)
	s.Require().NoError(err)
	return char
}

// bardDraft is the draft behind finalize, with the instrument selection left
// to the caller so a scene can supply the wrong number of them.
func (s *BardFinalizeSuite) bardDraft(charisma int, instruments []shared.SelectionID) *Draft {
	draft, err := NewDraft(&DraftConfig{ID: "bard-draft", PlayerID: "player-1"})
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
			Tools:  instruments,
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
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: charisma,
		},
		Method: "standard-array",
	}))

	return draft
}

// TestPoolIsTheCharismaModifier — max(1, CHA mod), and full on a fresh sheet.
func (s *BardFinalizeSuite) TestPoolIsTheCharismaModifier() {
	char := s.finalize(16)

	pool := char.GetResource(resources.Inspiration)
	s.Require().NotNil(pool, "a bard carries an inspiration pool")
	s.Equal(3, pool.Maximum(), "CHA 16 is a +3 modifier")
	s.Equal(3, pool.Current())
}

// TestPoolNeverFallsBelowOne is the minimum RAW keeps, and the reason it is
// here rather than assumed: a bard with a Charisma of 10 would otherwise carry
// a pool of zero, which is a feature on the sheet that can never be used.
func (s *BardFinalizeSuite) TestPoolNeverFallsBelowOne() {
	char := s.finalize(8)

	pool := char.GetResource(resources.Inspiration)
	s.Require().NotNil(pool)
	s.Equal(1, pool.Maximum(), "a -1 modifier still leaves one use")
}

// TestTheFeatureIsOnTheSheet — the level-1 grant reached compileFeatures, and
// the panel would aim it at an ally rather than at nobody.
func (s *BardFinalizeSuite) TestTheFeatureIsOnTheSheet() {
	char := s.finalize(16)

	found := false
	for _, feature := range char.features {
		if feature.Ref() != nil && feature.Ref().ID == refs.Features.BardicInspiration().ID {
			found = true
			s.Equal(coreCombat.ActionBonus, feature.ActionType())
		}
	}
	s.True(found, "a level-1 bard carries Bardic Inspiration")

	s.Equal(TargetKindSingleEntity, targetKindForRef(refs.Features.BardicInspiration()),
		"the panel aims it at an ally")

	current, maximum := char.featureResourceInfo(newBardicInspirationForTest())
	s.Equal(3, maximum, "the row reads the pool the grant will spend")
	s.Equal(3, current)
}

// TestNoSpellSlots is R6. Two slots nothing in this stack could spend were a
// zero value that lied, so they are deleted rather than left for later.
func (s *BardFinalizeSuite) TestNoSpellSlots() {
	char := s.finalize(16)
	s.Empty(char.ToData().SpellSlots, "a bard has no reachable slots in this slice")
}

// TestLongRestRestoresTheUses — the pool is a long-rest resource and nothing
// but the table row says so.
func (s *BardFinalizeSuite) TestLongRestRestoresTheUses() {
	char := s.finalize(16)
	pool := char.GetResource(resources.Inspiration)
	s.Require().NoError(char.UseResource(resources.Inspiration, 2))
	s.Require().Equal(1, pool.Current())

	s.Require().NoError(char.LongRest(context.Background()))

	s.Equal(3, char.GetResource(resources.Inspiration).Current(), "a long rest fills it")
}

// TestPoolIsALongRestResource pins the reset type itself, so a change from
// long to short rest cannot pass by leaving LongRest still working.
func (s *BardFinalizeSuite) TestPoolIsALongRestResource() {
	char := s.finalize(16)
	s.Equal(coreResources.ResetLongRest, char.GetResource(resources.Inspiration).ResetType)
}

// TestTheChosenInstrumentsBecomeProficiencies is the whole point of choosing
// them: three instruments picked at creation are on the finished sheet.
func (s *BardFinalizeSuite) TestTheChosenInstrumentsBecomeProficiencies() {
	char := s.finalize(16)

	s.Subset(char.ToData().ToolProficiencies, []proficiencies.Tool{
		proficiencies.ToolLute, proficiencies.ToolFlute, proficiencies.ToolDrum,
	}, "a bard who chose three instruments is proficient with three instruments")
}

// TestADraftWithTwoInstrumentsIsRefused — the requirement is enforced at the
// draft, not merely advertised. A bard who picked two must be told, rather
// than finalized with a hole in the sheet.
func (s *BardFinalizeSuite) TestADraftWithTwoInstrumentsIsRefused() {
	draft := s.bardDraft(16, []shared.SelectionID{"lute", "flute"})

	err := draft.ValidateChoices()

	s.Require().Error(err)
	s.Contains(err.Error(), "3 musical instruments")
}

// newBardicInspirationForTest is the feature as the factory builds it.
func newBardicInspirationForTest() features.Feature { return features.NewBardicInspiration() }

func TestBardFinalizeSuite(t *testing.T) {
	suite.Run(t, new(BardFinalizeSuite))
}
