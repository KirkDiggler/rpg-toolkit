// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// startfacing_test.go is the composition's half of the start point's optional
// direction (rpg-project#374): the field carries it, every member's map says
// the same thing about it, it survives being stored, and nothing reads it.

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestTheAtlasSaysWhereTheDungeonBegins is the projection claim: a way in is
// structure, so it is the same for everybody and survives the per-member
// filter unfiltered, exactly as an exit does.
func (s *HoldingsSuite) TestTheAtlasSaysWhereTheDungeonBegins() {
	field := heirloomField()
	field.Start = &encounter.FieldStart{At: raiderCell, Facing: "e"}
	enc := s.openWithField(field)

	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().NotNil(full.Start)
	s.Equal("e", full.Start.Facing)
	s.Equal(cellAt(int(raiderCell.X), int(raiderCell.Y)), full.Start.At,
		"the authored cell, converted exactly once — the same conversion the exits get")

	for _, member := range []core.EntityID{raider, partner} {
		atlas, err := enc.AtlasFor(member)
		s.Require().NoError(err)
		s.Require().NotNil(atlas.Start, "%s's own map says where the dungeon begins", member)
		s.Equal(*full.Start, *atlas.Start,
			"a way in is a fact about the building: nothing about it varies by who is asking")
	}
}

// TestAFieldWithNoStartProjectsNone is the zero-value claim. A nil is the
// honest answer, because a zero-valued start is a real dungeon somebody could
// author — the party arriving at [0,0] looking nowhere — and "nobody said"
// has to be distinguishable from "they said that".
func (s *HoldingsSuite) TestAFieldWithNoStartProjectsNone() {
	enc := s.openWithField(heirloomField())

	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Nil(full.Start, "this field declares no way in, and says so by saying nothing")

	atlas, err := enc.AtlasFor(raider)
	s.Require().NoError(err)
	s.Nil(atlas.Start)
}

// TestAStartWithNoFacingIsStillAStart separates the two absences. The cell is
// authored and the direction is not, which is every dungeon that existed
// before this word did.
func (s *HoldingsSuite) TestAStartWithNoFacingIsStillAStart() {
	field := heirloomField()
	field.Start = &encounter.FieldStart{At: raiderCell}
	enc := s.openWithField(field)

	atlas, err := enc.AtlasFor(raider)
	s.Require().NoError(err)
	s.Require().NotNil(atlas.Start)
	s.Empty(atlas.Start.Facing, "open the camera however it opened before")
}

// TestTheStartSurvivesASaveAndLoad is the reason the start rides the FIELD
// rather than the compiled spec. A host stores the encounter and throws the
// spec away, so a facing kept only on the compile would be gone the first
// time anybody reloaded — and a live session's map could never answer for it.
func (s *HoldingsSuite) TestTheStartSurvivesASaveAndLoad() {
	field := heirloomField()
	field.Start = &encounter.FieldStart{At: raiderCell, Facing: "nw"}
	enc := s.openWithField(field)

	data := enc.ToData()
	s.Require().NotNil(data.Field.Start, "and it is written into the blob")
	s.Equal("nw", data.Field.Start.Facing)

	reloaded := s.reload(enc)
	atlas, err := reloaded.Atlas()
	s.Require().NoError(err)
	s.Require().NotNil(atlas.Start)
	s.Equal("nw", atlas.Start.Facing, "the way in came back exactly as it went in")
	s.Equal(cellAt(int(raiderCell.X), int(raiderCell.Y)), atlas.Start.At)
}

// TestABlobWithNoStartLoadsWithNone is the compatibility half: every stored
// encounter written before this field existed has no `start` key, and must
// load as a field that declares none rather than as one that declares [0,0].
func (s *HoldingsSuite) TestABlobWithNoStartLoadsWithNone() {
	enc := s.openWithField(heirloomField())
	data := enc.ToData()
	s.Require().Nil(data.Field.Start, "no key at all — the bytes a pre-start blob has")

	reloaded := s.reload(enc)
	atlas, err := reloaded.Atlas()
	s.Require().NoError(err)
	s.Nil(atlas.Start)
}

// TestTheStartIsCarriedNeverRead is the charter claim, and it is checked as a
// difference rather than asserted in prose: two encounters identical but for
// the start must behave identically in everything that is not the atlas.
//
// The facing is PRESENTATION. It gates no movement, no sight and no verb, and
// a composition that started reading it would be deciding a rule from a
// camera hint.
func (s *HoldingsSuite) TestTheStartIsCarriedNeverRead() {
	plain := s.openWithField(heirloomField())

	faced := heirloomField()
	faced.Start = &encounter.FieldStart{At: spatial.Position{X: 9, Y: 9}, Facing: "s"}
	withStart := s.openWithField(faced)

	s.Run("the same cells are standable", func() {
		a, err := plain.Atlas()
		s.Require().NoError(err)
		b, err := withStart.Atlas()
		s.Require().NoError(err)
		s.Equal(a.Cells, b.Cells)
		s.Equal(a.Sealed, b.Sealed)
		s.Equal(a.Props, b.Props, "and a start on a cell changes nothing standing on it")
	})

	s.Run("the same story, beat for beat", func() {
		s.Equal(s.storyBytes(plain, raider), s.storyBytes(withStart, raider))
	})

	s.Run("a start on a cell nobody can stand on is not refused", func() {
		// [9,9] is outside the fixture's floor. The composition carries it
		// anyway, because it never places anybody from it — the authoring
		// dialect is where an unreachable start is somebody's mistake, and
		// it refuses one there, by name.
		atlas, err := withStart.Atlas()
		s.Require().NoError(err)
		s.Require().NotNil(atlas.Start)
		s.NotContains(atlas.Cells, atlas.Start.At)
	})
}
