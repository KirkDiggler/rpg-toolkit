// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package saves_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// DCSourceSuite checks each formula's arithmetic against hand-computed 5e
// values. Two of the three have no consumer in this lane; they are implemented
// and pinned now because the arithmetic is two lines and the alternative is a
// half-built enum whose unused arms nobody has ever run.
type DCSourceSuite struct {
	suite.Suite
}

func TestDCSourceSuite(t *testing.T) {
	suite.Run(t, new(DCSourceSuite))
}

// A static DC ignores the trigger entirely — that is what makes it static.
func (s *DCSourceSuite) TestStaticIgnoresDamage() {
	dc := saves.DCStatic(11)

	s.Assert().Equal(11, dc.DC(saves.DCInput{}))
	s.Assert().Equal(11, dc.DC(saves.DCInput{DamageTaken: 37}))
	s.Assert().Equal(saves.DCKindStatic, dc.Kind())
}

// Undead Fortitude: "the zombie must succeed on a Constitution saving throw
// with a DC of 5 + the damage taken".
func (s *DCSourceSuite) TestFivePlusDamageTaken() {
	dc := saves.DCFivePlusDamageTaken()

	s.Assert().Equal(5, dc.DC(saves.DCInput{DamageTaken: 0}), "no damage is still DC 5")
	s.Assert().Equal(6, dc.DC(saves.DCInput{DamageTaken: 1}))
	s.Assert().Equal(12, dc.DC(saves.DCInput{DamageTaken: 7}))
	s.Assert().Equal(25, dc.DC(saves.DCInput{DamageTaken: 20}))
	s.Assert().Equal(saves.DCKindFivePlusDamageTaken, dc.Kind())
}

// Concentration: "DC 10 or half the damage taken, whichever number is higher".
// The half is rounded down, which is where a wrong implementation hides: the
// floor only becomes visible on odd damage.
func (s *DCSourceSuite) TestHalfDamageFloorTen() {
	dc := saves.DCHalfDamageFloorTen()

	s.Run("the floor holds below 20 damage", func() {
		s.Assert().Equal(10, dc.DC(saves.DCInput{DamageTaken: 0}))
		s.Assert().Equal(10, dc.DC(saves.DCInput{DamageTaken: 1}))
		s.Assert().Equal(10, dc.DC(saves.DCInput{DamageTaken: 19}), "19/2 = 9, so the floor wins")
	})

	s.Run("the boundary", func() {
		s.Assert().Equal(10, dc.DC(saves.DCInput{DamageTaken: 20}), "20/2 = 10, floor and half agree")
		s.Assert().Equal(10, dc.DC(saves.DCInput{DamageTaken: 21}), "21/2 rounds DOWN to 10, not up to 11")
	})

	s.Run("odd damage rounds down above the floor", func() {
		s.Assert().Equal(11, dc.DC(saves.DCInput{DamageTaken: 22}))
		s.Assert().Equal(11, dc.DC(saves.DCInput{DamageTaken: 23}), "23/2 = 11, not 12")
		s.Assert().Equal(12, dc.DC(saves.DCInput{DamageTaken: 25}), "25/2 = 12, not 13")
		s.Assert().Equal(37, dc.DC(saves.DCInput{DamageTaken: 75}), "75/2 = 37")
	})

	s.Assert().Equal(saves.DCKindHalfDamageFloorTen, dc.Kind())
}

// Damage below zero is not a thing, and a formula that answered below its floor
// because someone passed a negative would be worse than the clamp.
func (s *DCSourceSuite) TestNegativeDamageIsClamped() {
	s.Assert().Equal(5, saves.DCFivePlusDamageTaken().DC(saves.DCInput{DamageTaken: -4}))
	s.Assert().Equal(10, saves.DCHalfDamageFloorTen().DC(saves.DCInput{DamageTaken: -4}))
}

// SaveGateSuite covers the declaration itself: what it refuses, and what it
// looks like as bytes.
type SaveGateSuite struct {
	suite.Suite
}

func TestSaveGateSuite(t *testing.T) {
	suite.Run(t, new(SaveGateSuite))
}

func (s *SaveGateSuite) wolfKnockdown() *saves.SaveGate {
	return &saves.SaveGate{
		Abilities:  []abilities.Ability{abilities.STR},
		DC:         saves.DCStatic(11),
		OnSuccess:  saves.Negated,
		Recurrence: saves.RecurrenceNone,
	}
}

// The common gate, spelled out once and then not spelled out again.
func (s *SaveGateSuite) TestNewSaveGateIsTheCommonShape() {
	s.Assert().Equal(s.wolfKnockdown(), saves.NewSaveGate(abilities.STR, 11))
}

// The wire form is kind-tagged, so a reader can tell which rule produced the
// number rather than finding a bare int and guessing.
func (s *SaveGateSuite) TestMarshalsKindTagged() {
	raw, err := json.Marshal(s.wolfKnockdown())
	s.Require().NoError(err)

	s.Assert().JSONEq(
		`{"abilities":["str"],"dc":{"kind":"static","n":11},"on_success":"negated","recurrence":"none"}`,
		string(raw))
}

func (s *SaveGateSuite) TestRoundTrips() {
	s.Run("static", func() {
		s.assertRoundTrips(s.wolfKnockdown())
	})

	s.Run("derived, and multi-ability", func() {
		s.assertRoundTrips(&saves.SaveGate{
			Abilities:  []abilities.Ability{abilities.CON, abilities.WIS},
			DC:         saves.DCFivePlusDamageTaken(),
			OnSuccess:  saves.Half,
			Recurrence: saves.RecurrenceEndOfTurn,
		})
	})

	s.Run("the concentration formula", func() {
		s.assertRoundTrips(&saves.SaveGate{
			Abilities:  []abilities.Ability{abilities.CON},
			DC:         saves.DCHalfDamageFloorTen(),
			OnSuccess:  saves.Negated,
			Recurrence: saves.RecurrenceNone,
		})
	})
}

// assertRoundTrips checks both directions: the gate comes back equal, and the
// bytes come back identical. Equality alone would miss a marshaller that
// dropped a field the unmarshaller then defaulted back to the same value.
func (s *SaveGateSuite) assertRoundTrips(gate *saves.SaveGate) {
	raw, err := json.Marshal(gate)
	s.Require().NoError(err)

	var back saves.SaveGate
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Require().Equal(gate, &back)

	again, err := json.Marshal(&back)
	s.Require().NoError(err)
	s.Require().JSONEq(string(raw), string(again))

	// And the formula survived, not just its name.
	s.Require().Equal(gate.DC.DC(saves.DCInput{DamageTaken: 25}), back.DC.DC(saves.DCInput{DamageTaken: 25}))
}

// An omitted OnSuccess or Recurrence means the common value. Authors write the
// two fields that vary and leave the two that almost never do.
func (s *SaveGateSuite) TestOmittedFieldsTakeTheCommonValue() {
	var gate saves.SaveGate
	s.Require().NoError(json.Unmarshal([]byte(`{"abilities":["str"],"dc":{"kind":"static","n":11}}`), &gate))

	s.Assert().Equal(saves.Negated, gate.OnSuccess)
	s.Assert().Equal(saves.RecurrenceNone, gate.Recurrence)
}

// A DC formula this build does not know came from a build that knew a rule this
// one does not. Dropping the gate would leave a consequence nobody can contest,
// with nothing saying so.
func (s *SaveGateSuite) TestUnknownDCKindIsRefused() {
	var gate saves.SaveGate
	err := json.Unmarshal([]byte(`{"abilities":["str"],"dc":{"kind":"twice_the_moon_phase"}}`), &gate)

	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "twice_the_moon_phase")
	s.Assert().Contains(err.Error(), "RAW", "and the error says what the price of a new one is")
}

func (s *SaveGateSuite) TestUnknownEnumValuesAreRefused() {
	var gate saves.SaveGate

	err := json.Unmarshal([]byte(`{"abilities":["str"],"dc":{"kind":"static","n":11},"on_success":"quarter"}`), &gate)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "quarter")

	err = json.Unmarshal([]byte(`{"abilities":["str"],"dc":{"kind":"static","n":11},"recurrence":"hourly"}`), &gate)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "hourly")
}

// A gate nobody can roll against, and a gate nobody can fail, are both a field
// left empty rather than a rule.
func (s *SaveGateSuite) TestRefusesAGateNobodyCouldMake() {
	s.Run("no ability", func() {
		err := (&saves.SaveGate{DC: saves.DCStatic(11)}).Validate()
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "no ability")
	})

	s.Run("no DC source", func() {
		err := (&saves.SaveGate{Abilities: []abilities.Ability{abilities.STR}}).Validate()
		s.Require().Error(err)
	})

	s.Run("a static DC of zero", func() {
		err := (&saves.SaveGate{
			Abilities: []abilities.Ability{abilities.STR},
			DC:        saves.DCStatic(0),
			OnSuccess: saves.Negated,
		}).Validate()
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "positive")
	})

	s.Run("a nil gate", func() {
		var gate *saves.SaveGate
		s.Require().Error(gate.Validate())
	})
}

// An invalid gate must not become bytes: a stored gate nobody could save
// against is the stat block lying again.
func (s *SaveGateSuite) TestRefusesToSerializeAnInvalidGate() {
	_, err := json.Marshal(&saves.SaveGate{DC: saves.DCStatic(11)})

	s.Require().Error(err)
}

func (s *SaveGateSuite) TestStringReadsLikeAStatBlock() {
	s.Assert().Equal("DC 11 str save, negated on success, recurrence none", s.wolfKnockdown().String())

	var absent *saves.SaveGate
	s.Assert().Equal("no save", absent.String())
}
