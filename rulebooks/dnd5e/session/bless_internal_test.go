// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

type BlessProjectionSuite struct{ suite.Suite }

func TestBlessProjectionSuite(t *testing.T) { suite.Run(t, new(BlessProjectionSuite)) }

func (s *BlessProjectionSuite) TestMissSurvivesOutcomeConversion() {
	spell := SpellRef{Ref: refs.Spells.Bless().String(), Name: "Bless"}
	targets, pushes, err := castOutcome(resolution.CastOutcome{
		Spell: *refs.Spells.Bless(), CasterID: "cleric",
		Targets: []resolution.CastTargetOutcome{{TargetID: "ally", Missed: true}},
	}, "cleric", spell)
	s.Require().NoError(err)
	s.Require().Len(targets, 1)
	s.True(targets[0].Missed)
	s.Nil(targets[0].Save)
	s.Empty(targets[0].Results)
	s.Empty(pushes)
}
