// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/stretchr/testify/suite"
)

type BlessProjectionSuite struct{ suite.Suite }

func TestBlessProjectionSuite(t *testing.T) { suite.Run(t, new(BlessProjectionSuite)) }

func (s *BlessProjectionSuite) TestMissHasTypedBodyAndRequiresIdentity() {
	valid := `{"beat":"cast_missed","actor":"cleric","target":"ally","spell":{"ref":"dnd5e:spells:bless","name":"Bless"}}`
	kind, body := decodeBeat([]byte(valid))
	s.Equal(EventCastMissed, kind)
	s.Equal(CastMissedBody{Actor: "cleric", Target: "ally", Spell: SpellRef{Ref: refs.Spells.Bless().String(), Name: "Bless"}}, body)
	for _, key := range []string{"actor", "target", "spell"} {
		var malformed map[string]any
		s.Require().NoError(json.Unmarshal([]byte(valid), &malformed))
		delete(malformed, key)
		raw, err := json.Marshal(malformed)
		s.Require().NoError(err)
		kind, body = decodeBeat(raw)
		s.Equal(EventCastMissed, kind)
		s.Nil(body)
	}
}

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
