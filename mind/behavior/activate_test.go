// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

// ActivateSuite is the `activate` word (rpg-project#1131, ADR-0051): a row that
// names an ability, on the table only while the caller lists the word as
// payable now. The module carries the word and judges nothing about it.
type ActivateSuite struct {
	suite.Suite
}

func TestActivateSuite(t *testing.T) { suite.Run(t, new(ActivateSuite)) }

func (s *ActivateSuite) pick(table behavior.Table, facts behavior.Facts, temper behavior.Temper) *behavior.PickOutput {
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: table, Facts: facts, Temper: temper, Die: &facedDie{face: 1}})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	return out
}

func secondWind() behavior.Table {
	return behavior.Table{behavior.KeyTime: {
		{Weight: 1, Activate: "second wind"},
		{Weight: 1, Hold: true},
	}}
}

// TestOnTheTableOnlyWhenTheCallerSaysItCanBePaidFor: the row is a candidate
// only while its exact word is in Facts.CanActivate.
func (s *ActivateSuite) TestOnTheTableOnlyWhenTheCallerSaysItCanBePaidFor() {
	listed := s.pick(secondWind(), behavior.Facts{CanActivate: []string{"second wind"}}, behavior.Temper{})
	s.Len(listed.Candidates, 2)
	s.True(listed.Rows[0].Affordable)
	s.Equal(0, listed.Entry)

	for name, facts := range map[string]behavior.Facts{
		"nothing listed":     {},
		"another ability":    {CanActivate: []string{"action surge"}},
		"a different case":   {CanActivate: []string{"Second Wind"}},
		"a padded word":      {CanActivate: []string{" second wind"}},
		"an empty (not nil)": {CanActivate: []string{}},
	} {
		out := s.pick(secondWind(), facts, behavior.Temper{})
		s.False(out.Rows[0].Affordable, name)
		s.Len(out.Candidates, 1, name)
		s.Equal(1, out.Entry, name)
	}
}

// TestItNeedsNothingElse: no movement, swing or action to dash is asked for.
func (s *ActivateSuite) TestItNeedsNothingElse() {
	out := s.pick(secondWind(), behavior.Facts{CanActivate: []string{"second wind"}}, behavior.Temper{})
	s.True(out.Rows[0].Affordable, "CanMove, CanAttack and CanDash are all false")
}

// TestItsOwnWhenStillGatesIt reads EnemyReach, which holds off Facts.EnemyInReach.
func (s *ActivateSuite) TestItsOwnWhenStillGatesIt() {
	table := behavior.Table{behavior.KeyTime: {
		{Weight: 1, When: []behavior.When{{Enemy: behavior.EnemyReach}}, Activate: "second wind"},
		{Weight: 1, Hold: true},
	}}
	listed := []string{"second wind"}
	away := s.pick(table, behavior.Facts{CanActivate: listed}, behavior.Temper{})
	s.Len(away.Candidates, 1)
	s.Equal(1, away.Entry)

	near := s.pick(table, behavior.Facts{CanActivate: listed, EnemyInReach: true}, behavior.Temper{})
	s.Len(near.Candidates, 2)
	s.Equal(0, near.Entry)
}

// TestAGroupCarriesNoActivate: a word, so refused on a group like the others.
func (s *ActivateSuite) TestAGroupCarriesNoActivate() {
	group := behavior.Answer{Weight: 1, Activate: "second wind",
		Rows: []behavior.Answer{{Weight: 1, Hold: true}}}
	err := group.Validate()
	s.Require().ErrorIs(err, behavior.ErrBadGroup)
	s.Contains(err.Error(), "`activate`")
}

// TestACompositionRowHoldsOneWord goes through NewComposer, the door
// composer_test.go uses, which checks every table row.
func (s *ActivateSuite) TestACompositionRowHoldsOneWord() {
	build := func(row behavior.Answer) error {
		spec := compositionSpec("acts", "unused", 1)
		spec.Table["work"] = []behavior.Answer{row}
		config := behavior.ComposerConfig{Instances: []behavior.InstanceSpec{spec}, Vocabulary: compositionVocabulary()}
		_, err := behavior.NewComposer(&config)

		return err
	}
	err := build(behavior.Answer{Weight: 1, Activate: "second wind", Hold: true})
	s.Require().Error(err)
	s.Contains(err.Error(), "needs one outcome word or a line, not 2 words")
	s.NoError(build(behavior.Answer{Weight: 1, Activate: "second wind"}))
}

// TestTheRecordReadsActivate: the word rides the row's JSON and a row without
// it marshals as before.
func (s *ActivateSuite) TestTheRecordReadsActivate() {
	row := behavior.Answer{Weight: 1, Activate: "second wind"}
	raw, err := json.Marshal(row)
	s.Require().NoError(err)
	s.Contains(string(raw), `"Activate":"second wind"`)
	var back behavior.Answer
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Equal(row, back)

	plain, err := json.Marshal(behavior.Answer{Weight: 1, Hold: true})
	s.Require().NoError(err)
	s.NotContains(string(plain), "Activate")

	out := s.pick(secondWind(), behavior.Facts{CanActivate: []string{"second wind"}}, behavior.Temper{})
	pick, err := json.Marshal(out)
	s.Require().NoError(err)
	s.Contains(string(pick), `"Activate":"second wind"`)
}

// TestATemperamentSaysNothingAboutIt: a non-zero profile loads it at 100.
func (s *ActivateSuite) TestATemperamentSaysNothingAboutIt() {
	out := s.pick(secondWind(), behavior.Facts{CanActivate: []string{"second wind"}},
		behavior.Temper{Word: "coward", Profile: cowardProfile})
	s.Require().Len(out.Candidates, 2)
	s.Equal(100, out.Candidates[0].Percent)
}
