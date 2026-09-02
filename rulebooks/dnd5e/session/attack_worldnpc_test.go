// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// AttackWorldNPCSuite pins rpg-toolkit#1404's attack-candidate exclusion:
// buildTargetPreflight itself has no kind gate at all — visibility and
// range only — so a world NPC standing in plain sight, well within attack
// range, must still never appear as a candidate. Reusing aFight's already-
// formed fight (alice vs. skeleton) and placing the vendor mid-scene through
// PlaceNPC proves the exclusion is real, not an accident of the vendor
// never being visible in the first place.
type AttackWorldNPCSuite struct {
	suite.Suite
}

func TestAttackWorldNPCSuite(t *testing.T) { suite.Run(t, new(AttackWorldNPCSuite)) }

func (s *AttackWorldNPCSuite) TestAWorldNPCInPlainSightIsNeverAnAttackCandidate() {
	alice := armedFighter("alice")
	mgr, _, _, _ := aFight(s.T(), alice, nil)

	// Well within the hall, well within reach — same room as alice and the
	// skeleton, so if this shows up as a candidate it is because Kind was
	// never checked, not because it stood somewhere unreachable.
	placeOut, err := mgr.PlaceNPC(context.Background(), &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: spatial.Position{X: 3, Y: 1}, NPC: merchantData(),
	})
	s.Require().NoError(err)
	s.Nil(placeOut.Formed, "a world NPC arriving mid-fight must not join it")

	afford, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	var attack *session.Declaration
	for i := range afford.Declarations {
		if afford.Declarations[i].Verb == session.VerbAttack {
			attack = &afford.Declarations[i]
		}
	}
	s.Require().NotNil(attack, "expected an Attack declaration")

	for _, c := range attack.Candidates {
		s.NotEqual("vendor", c.Member, "a world NPC must never appear as an attack candidate, available or not")
	}
	s.NotEmpty(attack.Candidates, "the skeleton must still be offered — this pins exclusion, not an empty candidate list")
}
