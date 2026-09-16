// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

type AdvanceTestSuite struct {
	suite.Suite

	ctx context.Context
	bus events.EventBus
}

func (s *AdvanceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestAdvanceSuite(t *testing.T) {
	suite.Run(t, new(AdvanceTestSuite))
}

// fighter finalizes the shared level-1 fighter fixture onto this suite's bus.
func (s *AdvanceTestSuite) fighter() *Character {
	draft := newFighterDraft(s.T())
	char, err := draft.ToCharacter(s.ctx, "advancing-fighter", s.bus)
	s.Require().NoError(err)
	return char
}

// featureRef reports whether the sheet carries a feature with this ref.
func (s *AdvanceTestSuite) hasFeature(char *Character, id string) bool {
	for _, feature := range char.GetFeatures() {
		if feature.Ref().ID == id {
			return true
		}
	}
	return false
}

// scriptedD10 returns the same face every time it is asked for one.
type scriptedD10 struct {
	face  int
	sides []int
}

func (r *scriptedD10) Roll(_ context.Context, size int) (int, error) {
	r.sides = append(r.sides, size)
	return r.face, nil
}

func (r *scriptedD10) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		roll, err := r.Roll(ctx, size)
		if err != nil {
			return nil, err
		}
		out = append(out, roll)
	}
	return out, nil
}

// --- Design §9.4: a level-1 fighter advances and holds Action Surge ---

func (s *AdvanceTestSuite) TestAFighterTakesLevelTwoAndGainsActionSurge() {
	char := s.fighter()
	s.Require().Equal(1, char.GetLevel())
	s.Require().False(s.hasFeature(char, refs.Features.ActionSurge().ID),
		"a level-1 fighter must not already have Action Surge")

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})

	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Equal(2, char.GetLevel())
	s.True(s.hasFeature(char, refs.Features.ActionSurge().ID),
		"level 2 grants Action Surge")

	s.Equal(2, out.Entry.Level)
	s.Equal(classes.Fighter, out.Entry.ClassID)
	s.Equal(HitPointMethodAverage, out.Entry.HitPointMethod)
	s.Equal(2, out.Gained.CharacterLevel)
	s.Equal(2, out.Gained.ClassLevel)
	s.Equal([]string{refs.Features.ActionSurge().ID}, refIDs(out.Gained.Features))
	s.Empty(out.Gained.Conditions)
}

func (s *AdvanceTestSuite) TestTheRecordHoldsBothLevelsInOrder() {
	char := s.fighter()
	_, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	record := char.Levels()
	s.Require().Len(record, 2, "two levels taken")
	s.Equal(1, record[0].Level)
	s.Equal(classes.Fighter, record[0].ClassID)
	s.Equal(HitPointMethodMax, record[0].HitPointMethod,
		"level 1 takes the full hit die")
	s.Equal(2, record[1].Level)
	s.Equal(classes.Fighter, record[1].ClassID)
	s.Equal(HitPointMethodAverage, record[1].HitPointMethod)
}

func (s *AdvanceTestSuite) TestTheRecordedLevelOneCarriesTheDraftsOwnChoices() {
	char := s.fighter()

	record := char.Levels()
	s.Require().NotEmpty(record)

	categories := make([]shared.ChoiceCategory, 0, len(record[0].Choices))
	for _, choice := range record[0].Choices {
		categories = append(categories, choice.Category)
	}
	s.Contains(categories, shared.ChoiceSkills,
		"level 1 recorded the skills the draft chose")
	s.Contains(categories, shared.ChoiceFightingStyle,
		"level 1 recorded the fighting style the draft chose")
}

func (s *AdvanceTestSuite) TestTheLevelsCopyCannotEditHistory() {
	char := s.fighter()

	record := char.Levels()
	s.Require().NotEmpty(record)
	record[0].Level = 99
	record[0].ClassID = classes.Wizard

	s.Equal(1, char.Levels()[0].Level)
	s.Equal(classes.Fighter, char.Levels()[0].ClassID)
}

func (s *AdvanceTestSuite) TestActionSurgeArrivesUsableAndIsSpendable() {
	char := s.fighter()
	_, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	surge := char.GetFeature(refs.Features.ActionSurge().ID)
	s.Require().NotNil(surge, "the feature is on the sheet by its own id")

	status, err := surge.Status(&features.StatusInput{})
	s.Require().NoError(err)
	s.Require().NotNil(status.Status.Resource)
	s.Equal(1, status.Status.Resource.Current, "one use, unspent")
	s.Equal(1, status.Status.Resource.Maximum)

	// Spend it: a turn is seeded, the feature grants the extra action, and the
	// use is gone.
	_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	economy := char.toToolkitActionEconomy()
	s.Require().NoError(surge.Activate(s.ctx, char, features.FeatureInput{
		Bus:           s.bus,
		ActionEconomy: economy,
	}))
	s.Equal(2, economy.ActionsRemaining, "Action Surge grants a second action")

	spent, err := surge.Status(&features.StatusInput{})
	s.Require().NoError(err)
	s.Equal(0, spent.Status.Resource.Current, "the use was spent")
}

// --- Design §9.1: the record persists and round-trips ---

func (s *AdvanceTestSuite) TestAnAdvancedFighterSurvivesAJSONRoundTrip() {
	char := s.fighter()
	_, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodRolled,
		Roller:         &scriptedD10{face: 7},
	})
	s.Require().NoError(err)

	raw, err := json.Marshal(char.ToData())
	s.Require().NoError(err)

	var data Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Equal(2, data.Level, "Data.Level is written from the record")
	s.Equal(2, data.ProficiencyBonus, "Data.ProficiencyBonus is written from the record")

	loaded, err := LoadFromData(s.ctx, &data, events.NewEventBus())
	s.Require().NoError(err)

	s.Equal(2, loaded.GetLevel())
	s.Equal(char.Levels(), loaded.Levels(), "the record comes back entry for entry")
	s.True(s.hasFeature(loaded, refs.Features.ActionSurge().ID),
		"Action Surge survives the round trip")
	s.Equal(char.GetMaxHitPoints(), loaded.GetMaxHitPoints())

	before := char.GetResourceData()
	after := loaded.GetResourceData()
	for key, want := range before {
		got, ok := after[key]
		s.Require().True(ok, "resource %s came back", key)
		s.Equal(want, got, "resource %s came back unchanged", key)
	}
}

func (s *AdvanceTestSuite) TestASpentActionSurgeReloadsSpent() {
	char := s.fighter()
	_, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	surge := char.GetFeature(refs.Features.ActionSurge().ID)
	s.Require().NotNil(surge)
	s.Require().NoError(surge.Activate(s.ctx, char, features.FeatureInput{
		Bus:           s.bus,
		ActionEconomy: char.toToolkitActionEconomy(),
	}))

	raw, err := json.Marshal(char.ToData())
	s.Require().NoError(err)
	var data Data
	s.Require().NoError(json.Unmarshal(raw, &data))

	loaded, err := LoadFromData(s.ctx, &data, events.NewEventBus())
	s.Require().NoError(err)

	reloaded := loaded.GetFeature(refs.Features.ActionSurge().ID)
	s.Require().NotNil(reloaded)
	status, err := reloaded.Status(&features.StatusInput{})
	s.Require().NoError(err)
	s.Equal(0, status.Status.Resource.Current, "the spend persisted")
	s.Equal(1, status.Status.Resource.Maximum)
}

// --- Design §9.4/§4.1 step 7: the numbers a level moves ---

func (s *AdvanceTestSuite) TestTheAverageMethodAddsHalfTheDiePlusOneAndCON() {
	char := s.fighter()
	before := char.GetMaxHitPoints()
	conMod := char.GetAbilityModifier(abilities.CON)

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	// Fighter hit die is d10: the fixed average is 6.
	s.Equal(6+conMod, out.Entry.HitPointGain)
	s.Equal(before+6+conMod, char.GetMaxHitPoints())
	s.Equal(out.Entry.HitPointGain, out.Gained.HitPointGain)
}

func (s *AdvanceTestSuite) TestTheRolledMethodRollsTheClassHitDie() {
	char := s.fighter()
	conMod := char.GetAbilityModifier(abilities.CON)
	roller := &scriptedD10{face: 9}

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodRolled,
		Roller:         roller,
	})
	s.Require().NoError(err)

	s.Equal([]int{10}, roller.sides, "a fighter rolls a d10, not some other die")
	s.Equal(9+conMod, out.Entry.HitPointGain)
}

func (s *AdvanceTestSuite) TestALevelGrantsAnUnspentHitDieWithoutRefillingTheRest() {
	char := s.fighter()
	s.Require().NoError(char.GetResource(resources.HitDice).Use(1))
	s.Require().Equal(0, char.GetResource(resources.HitDice).Current())

	_, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	hitDice := char.GetResource(resources.HitDice)
	s.Equal(2, hitDice.Maximum(), "the maximum grew with the level")
	s.Equal(1, hitDice.Current(),
		"the new die arrives unspent and the spent one stays spent")
}

func (s *AdvanceTestSuite) TestAdvancingMarksTheSheetForWriteBack() {
	char := s.fighter()
	char.dirty = false

	_, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	s.True(char.IsDirty(),
		"a level that did not mark the sheet would be discarded by the next write-back")
}

// --- Design §9.5: refusals, and what a refusal leaves behind ---

func (s *AdvanceTestSuite) TestAdvanceRefusesInsideAnEncounter() {
	char := s.fighter()
	_, err := char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	s.Require().True(char.InCombat())

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorContains(err, "in combat")
	s.Equal(1, char.GetLevel(), "the refusal left the level alone")
	s.False(s.hasFeature(char, refs.Features.ActionSurge().ID))
}

func (s *AdvanceTestSuite) TestARefusedAdvanceChangesNothing() {
	char := s.fighter()
	// ToData stamps UpdatedAt with the wall clock every time it is called, so
	// the stamp is zeroed on both sides and everything else is compared whole.
	snapshot := func() string {
		data := char.ToData()
		data.UpdatedAt = time.Time{}
		raw, err := json.Marshal(data)
		s.Require().NoError(err)
		return string(raw)
	}
	beforeJSON := snapshot()

	cases := []struct {
		name  string
		input *AdvanceInput
	}{
		{"another class", &AdvanceInput{ClassID: classes.Wizard, HitPointMethod: HitPointMethodAverage}},
		{"no class", &AdvanceInput{HitPointMethod: HitPointMethodAverage}},
		{"max hit points", &AdvanceInput{ClassID: classes.Fighter, HitPointMethod: HitPointMethodMax}},
		{"unknown method", &AdvanceInput{ClassID: classes.Fighter, HitPointMethod: "guess"}},
		{"unasked-for choices", &AdvanceInput{
			ClassID:        classes.Fighter,
			HitPointMethod: HitPointMethodAverage,
			Choices:        []choices.ChoiceData{{Category: shared.ChoiceSkills, ChoiceID: "made-up"}},
		}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			out, err := char.Advance(s.ctx, tc.input)

			s.Require().Error(err)
			s.Nil(out)
			s.Equal(1, char.GetLevel())
			s.False(s.hasFeature(char, refs.Features.ActionSurge().ID))

			s.JSONEq(beforeJSON, snapshot(),
				"a refused advance leaves the sheet byte-identical")
		})
	}
}

func (s *AdvanceTestSuite) TestAdvanceRefusesANilInput() {
	char := s.fighter()

	out, err := char.Advance(s.ctx, nil)

	s.Require().Error(err)
	s.Nil(out, "never (nil, nil)")
}

func (s *AdvanceTestSuite) TestAdvanceRefusesALevelThatWouldNeedAChoiceItCannotTake() {
	char := s.fighter()
	// Fighter picks a Martial Archetype at 3, and a subclass is not something
	// a ChoiceData can express, so level 3 must be refused rather than taken
	// with the choice silently skipped.
	_, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorContains(err, "requires choice")
	s.Equal(2, char.GetLevel(), "the character is still level 2")
}

// --- Design §9.3a: class level and character level are different numbers ---

// mixedRecordCharacter is the one thing R2.4 will not let Advance build: a
// character whose class level and character level differ. It is built by hand,
// from inside the package, precisely because every number below is equal to
// every other number today — a test written against a real character would
// pass with the two swapped.
func (s *AdvanceTestSuite) mixedRecordCharacter() *Character {
	levels := make([]LevelEntry, 0, 5)
	for i := 1; i <= 5; i++ {
		class := classes.Wizard
		if i <= 2 {
			class = classes.Fighter
		}
		levels = append(levels, LevelEntry{Level: i, ClassID: class, HitPointMethod: HitPointMethodAverage})
	}

	return &Character{
		id:            "two-fighter-levels-and-three-wizard",
		classID:       classes.Fighter,
		levels:        levels,
		abilityScores: shared.AbilityScores{abilities.CON: 10},
		resources:     make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}
}

func (s *AdvanceTestSuite) TestCharacterLevelCountsEveryEntryAndClassLevelCountsItsOwn() {
	char := s.mixedRecordCharacter()

	s.Equal(5, char.GetLevel(), "five levels taken in total")
	s.Equal(2, char.ClassLevel(classes.Fighter), "two of them in fighter")
	s.Equal(3, char.ClassLevel(classes.Wizard), "three of them in wizard")
	s.Equal(0, char.ClassLevel(classes.Bard), "none in a class never taken")
}

func (s *AdvanceTestSuite) TestProficiencyBonusFollowsCharacterLevelNotClassLevel() {
	char := s.mixedRecordCharacter()

	// Character level 5 -> +3. Fighter class level 2 -> +2. The two disagree,
	// which is the whole point: a proficiency bonus derived from the class
	// level would report 2 here.
	s.Equal(3, char.ProficiencyBonus())
	s.NotEqual(proficiencyBonusForLevel(char.ClassLevel(classes.Fighter)), char.ProficiencyBonus())
}

func (s *AdvanceTestSuite) TestGrantsFollowClassLevelNotCharacterLevel() {
	char := s.mixedRecordCharacter()

	byClassLevel := classes.GetGrantsGainedAtLevel(classes.Fighter, char.ClassLevel(classes.Fighter))
	byCharacterLevel := classes.GetGrantsGainedAtLevel(classes.Fighter, char.GetLevel())

	s.Require().Len(byClassLevel, 1, "the second FIGHTER level is the one that grants")
	s.Equal(refs.Features.ActionSurge().String(), byClassLevel[0].Features[0].Ref)
	s.Empty(byCharacterLevel,
		"the fifth CHARACTER level grants this fighter nothing, so the two cannot be swapped")
}

func (s *AdvanceTestSuite) TestClassPoolsFollowClassLevelAndHitDiceFollowCharacterLevel() {
	char := s.mixedRecordCharacter()

	// A monk's Ki is the clearest case: it is sized by monk level, while hit
	// dice count every level whatever class took it.
	built := buildClassResources(char, classes.Monk, 2, 5)

	ki, ok := built[resources.Ki]
	s.Require().True(ok, "two monk levels grant Ki")
	s.Equal(2, ki.Maximum(), "Ki is sized by the CLASS level")

	hitDice, ok := built[resources.HitDice]
	s.Require().True(ok)
	s.Equal(5, hitDice.Maximum(), "hit dice are counted by the CHARACTER level")
}

// --- Design §9.6: the proficiency bonus is derived ---

func (s *AdvanceTestSuite) TestProficiencyBonusIsDerivedFromTheLevel() {
	cases := []struct {
		level int
		want  int
	}{
		{1, 2}, {2, 2}, {3, 2}, {4, 2},
		{5, 3}, {8, 3},
		{9, 4}, {12, 4},
		{13, 5}, {16, 5},
		{17, 6}, {20, 6},
	}

	for _, tc := range cases {
		char := &Character{classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, tc.level)}
		s.Equal(tc.want, char.ProficiencyBonus(), "level %d", tc.level)
	}
}

func (s *AdvanceTestSuite) TestALevelFiveFighterReportsPlusThreeThroughEveryDoorThatUsesIt() {
	char := s.fighter()
	char.levels = syntheticLevels(classes.Fighter, 5)

	s.Equal(3, char.ProficiencyBonus())
	s.Equal(3, char.ToData().ProficiencyBonus, "the projection moved with it")

	// Athletics is one of this fixture's proficient skills, so its modifier
	// carries the bonus.
	strMod := char.GetAbilityModifier(abilities.STR)
	s.Equal(strMod+3, char.GetSkillModifier(skills.Athletics))
}

// --- Design §9.7: nothing changed for a level-1 character ---

func (s *AdvanceTestSuite) TestALevelOneFighterIsUnchangedByTheRecord() {
	char := s.fighter()

	s.Equal(1, char.GetLevel())
	s.Equal(2, char.ProficiencyBonus())
	s.False(s.hasFeature(char, refs.Features.ActionSurge().ID),
		"the new level-2 grant is not reachable at level 1")

	data := char.ToData()
	s.Equal(1, data.Level)
	s.Equal(2, data.ProficiencyBonus)
	s.Equal(1, char.GetResource(resources.HitDice).Maximum())
}

// refIDs flattens refs to their ids for readable assertions.
func refIDs(list []core.Ref) []string {
	out := make([]string, 0, len(list))
	for _, ref := range list {
		out = append(out, ref.ID)
	}
	return out
}

// --- Design R4.2: what a failed attach leaves on the bus ---

// attachingFeature is a feature with a bus lifecycle whose Apply can be made
// to fail, so that the rollback in attachGranted has something to roll back.
type attachingFeature struct {
	ref       *core.Ref
	failApply bool
	applied   int
	removed   int
}

func (f *attachingFeature) GetID() string            { return f.ref.ID }
func (f *attachingFeature) GetType() core.EntityType { return features.EntityTypeFeature }
func (f *attachingFeature) Ref() *core.Ref           { return f.ref }
func (f *attachingFeature) Name() string             { return f.ref.ID }

func (f *attachingFeature) ActionType() coreCombat.ActionType { return coreCombat.ActionFree }

func (f *attachingFeature) Status(*features.StatusInput) (*features.StatusOutput, error) {
	return &features.StatusOutput{Status: &features.Status{Ref: *f.ref, Name: f.ref.ID}}, nil
}

func (f *attachingFeature) ToJSON() (json.RawMessage, error) {
	return json.RawMessage(`{"ref":"` + f.ref.String() + `"}`), nil
}

func (f *attachingFeature) CanActivate(context.Context, core.Entity, features.FeatureInput) error {
	return nil
}

func (f *attachingFeature) Activate(context.Context, core.Entity, features.FeatureInput) error {
	return nil
}

func (f *attachingFeature) Apply(context.Context, events.EventBus) error {
	f.applied++
	if f.failApply {
		return rpgerr.New(rpgerr.CodeInternal, "this feature refuses to attach")
	}
	return nil
}

func (f *attachingFeature) Remove(context.Context, events.EventBus) error {
	f.removed++
	return nil
}

func (s *AdvanceTestSuite) TestAFailedAttachTakesBackTheFeaturesThatWentOn() {
	char := s.fighter()
	first := &attachingFeature{ref: &core.Ref{Module: "dnd5e", Type: "features", ID: "first"}}
	second := &attachingFeature{
		ref:       &core.Ref{Module: "dnd5e", Type: "features", ID: "second"},
		failApply: true,
	}

	attached, err := char.attachGranted(s.ctx, []features.Feature{first, second}, nil)

	s.Require().Error(err)
	s.Nil(attached)
	s.Equal(1, first.applied)
	s.Equal(1, first.removed, "the feature that did go on came back off")
	s.Equal(1, second.removed, "and so did the one whose Apply failed part-way")
	s.Len(char.SheetKeeper().attachedFeatures, 0,
		"nothing this call attached is recorded on the keeper")
}

func (s *AdvanceTestSuite) TestASheetWithNoBusAttachesNothing() {
	char := s.fighter()
	char.bus = nil
	feature := &attachingFeature{ref: &core.Ref{Module: "dnd5e", Type: "features", ID: "inert"}}

	attached, err := char.attachGranted(s.ctx, []features.Feature{feature}, nil)

	s.Require().NoError(err)
	s.Nil(attached)
	s.Equal(0, feature.applied,
		"effects are inert until something puts them on a bus")
}

func (s *AdvanceTestSuite) TestALoadedSheetWithNoBusCanStillAdvance() {
	char := s.fighter()
	data := char.ToData()

	inert, err := Load(s.ctx, data)
	s.Require().NoError(err)

	out, err := inert.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})

	s.Require().NoError(err)
	s.Equal(2, out.Gained.CharacterLevel)
	s.True(s.hasFeature(inert, refs.Features.ActionSurge().ID))

	// And the feature that arrived without a bus goes on the one it is later
	// attached to.
	bus := events.NewEventBus()
	s.Require().NoError(Attach(s.ctx, inert, bus))

	surge := inert.GetFeature(refs.Features.ActionSurge().ID)
	s.Require().NotNil(surge)
	status, err := surge.Status(&features.StatusInput{})
	s.Require().NoError(err)
	s.Equal(1, status.Status.Resource.Current)
}

// --- Design R4.5: a level that grants nothing is still a level ---

func (s *AdvanceTestSuite) TestAMonkSecondLevelGrantsNoFeatureAndIsStillValid() {
	draft := newMonkDraft(s.T())
	char, err := draft.ToCharacter(s.ctx, "advancing-monk", s.bus)
	s.Require().NoError(err)
	before := len(char.GetFeatures())
	_, hadKi := char.GetResourceData()[resources.Ki]
	s.Require().False(hadKi, "a level-1 monk has no Ki")

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Monk,
		HitPointMethod: HitPointMethodAverage,
	})

	s.Require().NoError(err)
	s.Empty(out.Gained.Features, "the monk has no level-2 grant row yet")
	s.Len(char.GetFeatures(), before, "and gained no feature")
	s.Equal(2, char.GetLevel(), "which does not make it an invalid level")

	ki := char.GetResource(resources.Ki)
	s.Require().NotNil(ki, "Ki arrives at monk level 2 from the class resource math")
	s.Equal(2, ki.Maximum())
	s.Equal(2, ki.Current())
}

func (s *AdvanceTestSuite) TestALevelRaisesThePoolWithoutStandingUpADyingCharacter() {
	char := s.fighter()
	char.hitPoints = 0
	before := char.GetMaxHitPoints()

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})

	s.Require().NoError(err)
	s.Equal(before+out.Entry.HitPointGain, char.GetMaxHitPoints())
	s.Equal(0, char.GetHitPoints(),
		"a dying character is still dying; a level is not healing")
}

func (s *AdvanceTestSuite) TestTheReturnedEntryCannotEditHistory() {
	char := s.fighter()
	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
		Choices:        nil,
	})
	s.Require().NoError(err)

	out.Entry.Level = 99

	s.Equal(2, char.Levels()[1].Level)
}

// --- Design §9.3a at the CALL SITE, not just in the helpers ---

// oneFighterLevelAmongFive is a character whose class level and character
// level differ by four: one level taken in fighter, then four in wizard.
//
// R2.4 will not let Advance build this, and every Advance-driven test above
// uses a single-class fighter where the two numbers are equal — so those tests
// pass just as happily with the two arguments swapped inside Advance. This
// fixture is the one shape that tells them apart, and it is here so the rule
// Kirk's multiclass remark motivated is not the one rule with no test that
// would catch it breaking.
//
// Its hit dice pool is seeded at five so that resizing it is a visible act:
// a pool sized from the CLASS level would be asked to shrink, and shrinking is
// the one thing resizeClassResources refuses to do.
func (s *AdvanceTestSuite) oneFighterLevelAmongFive() *Character {
	levels := make([]LevelEntry, 0, 5)
	for i := 1; i <= 5; i++ {
		class := classes.Wizard
		if i == 1 {
			class = classes.Fighter
		}
		levels = append(levels, LevelEntry{Level: i, ClassID: class, HitPointMethod: HitPointMethodAverage})
	}

	char := &Character{
		id:            "one-fighter-level-among-five",
		classID:       classes.Fighter,
		levels:        levels,
		hitDice:       10,
		hitPoints:     30,
		maxHitPoints:  30,
		abilityScores: shared.AbilityScores{abilities.CON: 10},
		resources:     make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}
	char.resources[resources.HitDice] = resources.NewHitDiceResource(resources.HitDiceResourceConfig{
		CharacterID: char.id,
		Level:       5,
	})

	s.Require().Equal(5, char.GetLevel())
	s.Require().Equal(1, char.ClassLevel(classes.Fighter))
	return char
}

func (s *AdvanceTestSuite) TestAdvanceIndexesGrantsByClassLevelAndTheEntryByCharacterLevel() {
	char := s.oneFighterLevelAmongFive()

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Fighter,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	// The grant. This is the second FIGHTER level, so Action Surge arrives —
	// even though it is the sixth CHARACTER level, which grants a fighter
	// nothing at all. Looking the grants up by the character level finds an
	// empty list and this assertion fails.
	s.Equal([]string{refs.Features.ActionSurge().ID}, refIDs(out.Gained.Features),
		"the second FIGHTER level is the one that grants Action Surge")
	s.True(s.hasFeature(char, refs.Features.ActionSurge().ID))

	// The entry. A level record entry is numbered by character level (R2.5):
	// entry n is level n+1, whatever class took it.
	s.Equal(6, out.Entry.Level, "the sixth level taken")
	s.Equal(6, char.Levels()[5].Level)
	s.Equal(6, char.GetLevel())

	// The two numbers, reported separately and not interchangeably.
	s.Equal(6, out.Gained.CharacterLevel)
	s.Equal(2, out.Gained.ClassLevel)

	// Hit dice count every level whatever class took it, so the pool grows
	// from five to six. Sized by the class level it would be asked to shrink
	// to two, which resizeClassResources refuses — leaving it at five.
	s.Equal(6, char.GetResource(resources.HitDice).Maximum(),
		"hit dice are counted by the CHARACTER level")
}

func (s *AdvanceTestSuite) TestAdvanceSizesAClassPoolByTheClassLevel() {
	// The mirror of the assertion above, on a pool that is sized by the class
	// rather than by the character: a monk with two monk levels among six
	// reaches monk level 3, and gets three Ki points rather than seven.
	levels := make([]LevelEntry, 0, 6)
	for i := 1; i <= 6; i++ {
		class := classes.Wizard
		if i <= 2 {
			class = classes.Monk
		}
		levels = append(levels, LevelEntry{Level: i, ClassID: class, HitPointMethod: HitPointMethodAverage})
	}
	char := &Character{
		id:            "two-monk-levels-among-six",
		classID:       classes.Monk,
		levels:        levels,
		hitDice:       8,
		hitPoints:     30,
		maxHitPoints:  30,
		abilityScores: shared.AbilityScores{abilities.CON: 10},
		resources:     make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Monk,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	s.Equal(7, out.Gained.CharacterLevel)
	s.Equal(3, out.Gained.ClassLevel)
	s.Equal(3, char.GetResource(resources.Ki).Maximum(),
		"Ki is sized by the MONK level, not by the seven levels this character holds")
	s.Equal(7, char.GetResource(resources.HitDice).Maximum(),
		"and hit dice by every level it holds")
}
