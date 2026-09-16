// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// LevelRecordLoadTestSuite covers design §2.2 and §2.3: the record is the
// truth, Data.Level is a projection of it, and a sheet where the two disagree
// does not load.
type LevelRecordLoadTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *LevelRecordLoadTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestLevelRecordLoadSuite(t *testing.T) {
	suite.Run(t, new(LevelRecordLoadTestSuite))
}

// sheet is a minimal persisted fighter, with whatever record the test hands it.
func (s *LevelRecordLoadTestSuite) sheet(level int, levels []LevelEntry) *Data {
	return &Data{
		ID:       "record-fighter",
		PlayerID: "player-1",
		Name:     "Arthur",
		Level:    level,
		Levels:   levels,
		ClassID:  classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 15, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 12, abilities.WIS: 10, abilities.CHA: 8,
		},
		HitPoints:    12,
		MaxHitPoints: 12,
	}
}

// --- R2.6: a character that predates the record ---

func (s *LevelRecordLoadTestSuite) TestAPreRecordLevelOneCharacterGainsASynthesizedEntry() {
	data := s.sheet(1, nil)

	loaded, err := Load(s.ctx, data)

	s.Require().NoError(err)
	s.Equal(1, loaded.GetLevel())

	record := loaded.Levels()
	s.Require().Len(record, 1)
	s.Equal(1, record[0].Level)
	s.Equal(classes.Fighter, record[0].ClassID)
	s.Equal(HitPointMethodMax, record[0].HitPointMethod,
		"level 1 took the full hit die, which is the one thing we can still say about it")
	s.Equal(12, record[0].HitPointGain, "d10 plus a +2 Constitution modifier")
	s.Empty(record[0].Choices,
		"the draft's choices are gone; inventing them would be inventing history")
}

func (s *LevelRecordLoadTestSuite) TestTheSynthesizedEntryIsWrittenBackOnTheNextSave() {
	loaded, err := Load(s.ctx, s.sheet(1, nil))
	s.Require().NoError(err)

	data := loaded.ToData()

	s.Require().Len(data.Levels, 1,
		"the sheet leaves the load carrying a record, so it never needs synthesizing twice")
	s.Equal(1, data.Level)
}

// --- R2.7: a recordless character at any other level ---

func (s *LevelRecordLoadTestSuite) TestAPreRecordCharacterAboveLevelOneIsRefused() {
	loaded, err := Load(s.ctx, s.sheet(2, nil))

	s.Require().Error(err)
	s.Nil(loaded)
	s.ErrorContains(err, "no level record")
}

func (s *LevelRecordLoadTestSuite) TestASheetClaimingLevelZeroIsRefused() {
	loaded, err := Load(s.ctx, s.sheet(0, nil))

	s.Require().Error(err)
	s.Nil(loaded, "a character below level 1 has no history to guess at either")
}

// --- R2.3: the projection and the record must agree ---

func (s *LevelRecordLoadTestSuite) TestASheetWhoseLevelDisagreesWithItsRecordIsRefused() {
	data := s.sheet(3, syntheticLevels(classes.Fighter, 2))

	loaded, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Nil(loaded)
	s.ErrorContains(err, "claims level 3")
	s.ErrorContains(err, "2 entries")
}

// --- R2.5: entry n is level n+1 ---

func (s *LevelRecordLoadTestSuite) TestAnOutOfOrderRecordIsRefused() {
	data := s.sheet(2, []LevelEntry{
		{Level: 2, ClassID: classes.Fighter, HitPointMethod: HitPointMethodMax},
		{Level: 1, ClassID: classes.Fighter, HitPointMethod: HitPointMethodAverage},
	})

	loaded, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Nil(loaded)
	s.ErrorContains(err, "expected 1")
}

// --- R2.4: every entry names the character's own class ---

func (s *LevelRecordLoadTestSuite) TestARecordNamingAnotherClassIsRefused() {
	data := s.sheet(2, []LevelEntry{
		{Level: 1, ClassID: classes.Fighter, HitPointMethod: HitPointMethodMax},
		{Level: 2, ClassID: classes.Wizard, HitPointMethod: HitPointMethodAverage},
	})

	loaded, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Nil(loaded)
	s.ErrorContains(err, "multiclassing")
}

// --- R2.2: the sheet does not keep the loaded projections as truth ---

func (s *LevelRecordLoadTestSuite) TestALoadedSheetDerivesItsLevelFromTheRecordNotTheField() {
	data := s.sheet(5, syntheticLevels(classes.Fighter, 5))
	data.ProficiencyBonus = 2 // stale: what a pre-derivation sheet stored

	loaded, err := Load(s.ctx, data)

	s.Require().NoError(err)
	s.Equal(5, loaded.GetLevel())
	s.Equal(3, loaded.ProficiencyBonus(),
		"the stored +2 is ignored; level 5 is +3")
	s.Equal(3, loaded.ToData().ProficiencyBonus,
		"and the next save corrects the projection")
}

func (s *LevelRecordLoadTestSuite) TestTheLoadedRecordIsNotTheCallersSlice() {
	levels := syntheticLevels(classes.Fighter, 2)
	loaded, err := Load(s.ctx, s.sheet(2, levels))
	s.Require().NoError(err)

	levels[1].ClassID = classes.Wizard

	s.Equal(classes.Fighter, loaded.Levels()[1].ClassID)
}
