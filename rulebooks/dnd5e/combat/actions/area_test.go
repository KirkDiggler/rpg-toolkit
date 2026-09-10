// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// AreaSuite covers the declaration a cast makes when the ENGINE derives its
// recipients from a shape rather than the caller naming them.
type AreaSuite struct {
	suite.Suite
}

func TestAreaSuite(t *testing.T) {
	suite.Run(t, new(AreaSuite))
}

func thunderclapShape() *actions.CastArea {
	return &actions.CastArea{
		Footprint: actions.Footprint{
			Shape: actions.AreaRadius, SizeFeet: 5, Origin: actions.AreaOriginCaster,
		},
		Catches: actions.AreaCatchesOthers,
	}
}

func areaProfile(mutate func(p *actions.CastProfile)) actions.CastProfile {
	p := actions.CastProfile{
		RangeFeet: 5,
		Target:    actions.CastTargetArea,
		Area:      thunderclapShape(),
		Damage:    []damage.Damage{{Dice: "1d6", Type: damage.Thunder}},
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}

// TestTheRuleAndTheShapeAreBoundInBothDirections is the point of making
// CastTargetArea a real target rule rather than a nil-check on Area.
//
// Neither half may appear without the other. The failure this prevents is not
// hypothetical: resolution's self arm rewrites the target list to the caster,
// so a profile that said "self" while carrying an area would resolve a spell
// against the one creature it must never hit — and would look right doing it.
func (s *AreaSuite) TestTheRuleAndTheShapeAreBoundInBothDirections() {
	s.Run("an area rule with a shape is valid", func() {
		s.NoError(areaProfile(nil).Validate())
	})
	s.Run("an area rule with no shape is refused", func() {
		err := areaProfile(func(p *actions.CastProfile) { p.Area = nil }).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "must declare an area")
	})
	s.Run("a shape on a self cast is refused", func() {
		err := areaProfile(func(p *actions.CastProfile) { p.Target = actions.CastTargetSelf }).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "target rule")
	})
	s.Run("a shape on a creature-targeted cast is refused", func() {
		err := areaProfile(func(p *actions.CastProfile) {
			p.Target = actions.CastTargetOneCreature
			p.MinTargets, p.MaxTargets = 1, 1
		}).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "target rule")
	})
}

// TestAnAreaCastNamesNobody — Min/MaxTargets bound what the CALLER may name,
// and the caller of an area cast names nobody. Who receives it is a different
// question, answered by the engine from the shape. A profile declaring both a
// shape and a target count is saying two contradictory things about who decides.
func (s *AreaSuite) TestAnAreaCastNamesNobody() {
	err := areaProfile(func(p *actions.CastProfile) { p.MinTargets, p.MaxTargets = 1, 3 }).Validate()
	s.Require().Error(err)
	s.Contains(err.Error(), "zero targets")
}

// TestAFootprintMustBeAShapeSomebodyCanStandIn.
func (s *AreaSuite) TestAFootprintMustBeAShapeSomebodyCanStandIn() {
	for _, tc := range []struct {
		name   string
		mutate func(f *actions.Footprint)
		want   string
	}{
		{"an unknown shape", func(f *actions.Footprint) { f.Shape = "cone" }, "unknown area shape"},
		{"an unknown origin", func(f *actions.Footprint) { f.Origin = "point" }, "unknown area origin"},
		{"no extent at all", func(f *actions.Footprint) { f.SizeFeet = 0 }, "positive size"},
		{"a negative extent", func(f *actions.Footprint) { f.SizeFeet = -5 }, "positive size"},
	} {
		s.Run(tc.name+" is refused", func() {
			err := areaProfile(func(p *actions.CastProfile) { tc.mutate(&p.Area.Footprint) }).Validate()
			s.Require().Error(err)
			s.Contains(err.Error(), tc.want)
		})
	}

	s.Run("an unknown projection is refused", func() {
		err := areaProfile(func(p *actions.CastProfile) { p.Area.Catches = "enemies" }).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "unknown area projection")
	})
}

// TestASubCellFootprintIsNotRefusedHere, and the absence is deliberate.
//
// A one-to-four-foot radius floors to zero cells and can then only ever catch
// something on the caster's own square — content that is wrong. It is not
// caught here because the feet-to-cells conversion lives in a module this one
// cannot import (rpg-toolkit#1625), so the refusal belongs to the layer that
// can measure. Pinned so the gap is a KNOWN cost with a test naming it, rather
// than something a later reader assumes was covered.
func (s *AreaSuite) TestASubCellFootprintIsNotRefusedHere() {
	s.NoError(areaProfile(func(p *actions.CastProfile) { p.Area.Footprint.SizeFeet = 4 }).Validate(),
		"four feet validates here and cannot reach anything on a five-foot grid")
}

// TestCloningCarriesTheShapeSomewhereElse — Clone exists so a running
// interaction cannot be rewritten by a caller reusing its definition. A shape
// left sharing a pointer would be exactly that hole.
func (s *AreaSuite) TestCloningCarriesTheShapeSomewhereElse() {
	original := areaProfile(nil)
	clone := original.Clone()

	s.Require().NotNil(clone.Area)
	s.Equal(*original.Area, *clone.Area)
	s.NotSame(original.Area, clone.Area, "the clone must not share the original's shape")

	clone.Area.Footprint.SizeFeet = 30
	s.Equal(5, original.Area.Footprint.SizeFeet, "editing a clone must not reach the original")
}
