// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// props_test.go is A PROP IS A THING, NOT A HOLE IN THE SIGHTLINE
// (rpg-toolkit#1128).
//
// What a room could place used to be a bare cell, and the module decided for it:
// every one blocked sight and none blocked movement, hardcoded. That is the
// inverse of nearly everything a dungeon contains — measured on main, a member
// stood inside a coffin, walked out through the far side of it, and could not
// see the wight beyond. A pillar you can walk through is not a pillar.
//
// So a prop now says what it is (a ref the module never reads) and what it does
// (two flags, independent, REQUIRED). The four combinations are all real, and
// this file walks each of them as a scene rather than asserting on fields:
//
//	blocks both    — a pillar, a statue: you go round it and you cannot see past
//	blocks movement— the reference tomb's coffin, a low altar: see over, not through
//	blocks sight   — a curtain, a fog bank: walk through it blind
//	blocks neither — candles, a rug: there, and in nobody's way
//
// The tomb's coffin is authored `blocks_los: false` and is the case this exists
// for: void.go promised "a low altar you can see over" stayed expressible, and
// it was the one thing that was not.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type PropsSuite struct {
	suite.Suite
}

func TestPropsSuite(t *testing.T) {
	suite.Run(t, new(PropsSuite))
}

// The chamber is anchored away from the origin so a projection that forgets to
// project is as visible here as one that projects the wrong way — the lesson
// placement_test.go's fixtures are built on.
var propsOrigin = spatial.Position{X: 20, Y: 10}

const (
	// The run is THREE CELLS TALL on purpose. spatial v0.9.1 lets a viewer
	// lean around a single occupied cell, so a lone prop blocks nothing and a
	// fixture built on one would be asserting the opposite of what it claims
	// (testwalls_test.go's own finding).
	propRunTop = 3
	propRunLen = 3

	propAtX = 5

	delver = core.EntityID("delve")
	beyond = core.EntityID("wight")
)

func propTrue() *bool  { t := true; return &t }
func propFalse() *bool { f := false; return &f }

// propSeat is a chamber-local cell as AUTHORED — the anchor plus the pair,
// in offset columns and rows.
func propSeat(x, y float64) spatial.Position {
	return spatial.Position{X: x, Y: y}.Add(propsOrigin)
}

// propAbs is the same cell in the dungeon's absolute AXIAL frame, which is
// what every verb takes and the map reports.
func propAbs(x, y float64) spatial.Position {
	seat := propSeat(x, y)
	return cellAt(int(seat.X), int(seat.Y))
}

// chamber is one room with a three-cell run of the same prop standing across
// the middle of it, a player on each side, and nothing else to explain a
// result.
func (s *PropsSuite) chamber(ref string, blocksMovement, blocksSight *bool) *encounter.Encounter {
	props := make([]encounter.PropInput, 0, propRunLen)
	for y := propRunTop; y < propRunTop+propRunLen; y++ {
		props = append(props, encounter.PropInput{
			Ref:               ref,
			At:                propSeat(propAtX, float64(y)),
			BlocksMovement:    blocksMovement,
			BlocksLineOfSight: blocksSight,
		})
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("crypt", int(propsOrigin.X), int(propsOrigin.Y), 12, 8)},
			Props:   props,
		},
		Members: []encounter.MemberInput{
			{ID: delver, Kind: encounter.KindPlayer, Position: propSeat(2, 4)},
			{ID: beyond, Kind: encounter.KindPlayer, Position: propSeat(8, 4)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

func (s *PropsSuite) sees(enc *encounter.Encounter, observer, subject core.EntityID) bool {
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject != intel.Subject(subject) {
			continue
		}
		for _, via := range h.CurrentVia {
			if via == intel.Sight {
				return true
			}
		}
	}

	return false
}

// standsOn reports whether the delver can end a step on the prop's own cell.
//
// ONTO, not THROUGH, and the distinction is the module's rather than this
// test's: [Encounter.Step] deliberately does not check adjacency, because that
// is a rule about walking and it lives with the walk (StepOutput.Doors' own doc
// comment). So a step is a placement question, and what a movement-blocking
// prop denies is a place to stand. A seam that walks a path cell by cell asks
// this once per cell and gets the wall it expects.
func (s *PropsSuite) standsOn(enc *encounter.Encounter) error {
	_, err := enc.Step(&encounter.StepInput{Member: delver, To: propAbs(propAtX, 4)})
	return err
}

// TestAPillarIsSolidAndOpaque — the ordinary case, and the one main got least
// wrong: it did block sight. It just did not stop anybody.
func (s *PropsSuite) TestAPillarIsSolidAndOpaque() {
	enc := s.chamber("dnd5e:props:pillar", propTrue(), propTrue())

	s.False(s.sees(enc, delver, beyond), "stone between them")
	s.False(s.sees(enc, beyond, delver), "and geometry is mutual")
	s.Require().ErrorIs(s.standsOn(enc), encounter.ErrBadPlacement,
		"a pillar is not somewhere to stand")
}

// TestTheTombsCoffinIsSolidAndSeenOver is the case this whole issue is about:
// `{ ref: dnd5e:props:coffin, at: [6, 3], blocks_los: false }`, straight out of
// reference-tomb.yaml.
func (s *PropsSuite) TestTheTombsCoffinIsSolidAndSeenOver() {
	enc := s.chamber("dnd5e:props:coffin", propTrue(), propFalse())

	s.True(s.sees(enc, delver, beyond), "she can see clean over a coffin")
	s.True(s.sees(enc, beyond, delver), "and he over her")
	s.Require().ErrorIs(s.standsOn(enc), encounter.ErrBadPlacement,
		"and neither of them can stand in it")
}

// TestACurtainIsPassedThroughBlind — the other asymmetry, which nothing could
// say before either.
func (s *PropsSuite) TestACurtainIsPassedThroughBlind() {
	enc := s.chamber("dnd5e:props:curtain", propFalse(), propTrue())

	s.False(s.sees(enc, delver, beyond), "she cannot see through it")
	s.Require().NoError(s.standsOn(enc), "but she can walk into it")
}

// TestCandlesAreThereAndInNobodysWay — a prop that blocks nothing is still a
// prop. It has a ref, it is on the map, and the client draws it.
func (s *PropsSuite) TestCandlesAreThereAndInNobodysWay() {
	enc := s.chamber("dnd5e:props:candles", propFalse(), propFalse())

	s.True(s.sees(enc, delver, beyond), "candles hide nobody")
	s.Require().NoError(s.standsOn(enc), "and stop nobody")

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Props, propRunLen,
		"and they are on the map, which is the whole reason a prop that does nothing exists")
}

// TestTheMapSaysWHICHThingIsWhere — identity, the half rpg-project#227 recorded
// as "a pillar and a statue are the same cell".
func (s *PropsSuite) TestTheMapSaysWHICHThingIsWhere() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("tomb", int(propsOrigin.X), int(propsOrigin.Y), 12, 8)},
			Props: []encounter.PropInput{
				{Ref: "dnd5e:props:statue-reaper", At: propSeat(1, 1),
					BlocksMovement: propTrue(), BlocksLineOfSight: propTrue()},
				{Ref: "dnd5e:props:altar", At: propSeat(9, 3),
					BlocksMovement: propTrue(), BlocksLineOfSight: propFalse()},
			},
		},
		Members: []encounter.MemberInput{
			{ID: delver, Kind: encounter.KindPlayer, Position: propSeat(4, 6)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	props := atlas.Props
	s.Require().Len(props, 2, "sorted by cell, the one order every list on the map shares")

	s.Equal("dnd5e:props:statue-reaper", props[0].Ref)
	s.Equal(propAbs(1, 1), props[0].At, "absolute, like everything else the map reports")
	s.True(props[0].BlocksLineOfSight)

	s.Equal("dnd5e:props:altar", props[1].Ref)
	s.Equal(propAbs(9, 3), props[1].At)
	s.False(props[1].BlocksLineOfSight, "you can see over an altar, and the map says so")
}

// TestAPropMustSayWhatItDoes — #1033's law. A bare false is a legal-looking
// answer nobody wrote (void.go's argument for sealing Void), so absence has to
// be absence.
func (s *PropsSuite) TestAPropMustSayWhatItDoes() {
	build := func(p encounter.PropInput) error {
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
				Regions: []encounter.RegionInput{rectRegion("crypt", int(propsOrigin.X), int(propsOrigin.Y), 12, 8)},
				Props:   []encounter.PropInput{p},
			},
			Members: []encounter.MemberInput{
				{ID: delver, Kind: encounter.KindPlayer, Position: propSeat(2, 4)},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		return err
	}

	at := propSeat(5, 4)

	s.Run("no movement answer", func() {
		err := build(encounter.PropInput{Ref: "dnd5e:props:pillar", At: at, BlocksLineOfSight: propTrue()})
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "blocks_movement")
	})

	s.Run("no sight answer", func() {
		err := build(encounter.PropInput{Ref: "dnd5e:props:pillar", At: at, BlocksMovement: propTrue()})
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "blocks_line_of_sight")
	})

	s.Run("no name", func() {
		err := build(encounter.PropInput{At: at, BlocksMovement: propTrue(), BlocksLineOfSight: propTrue()})
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "ref")
	})
}

// TestAPropSurvivesASave — the ref and both answers, or the reload is a
// different dungeon.
func (s *PropsSuite) TestAPropSurvivesASave() {
	enc := s.chamber("dnd5e:props:coffin", propTrue(), propFalse())

	data := enc.ToData()
	s.Require().Len(data.Field.Props, propRunLen)
	s.Equal("dnd5e:props:coffin", data.Field.Props[0].Ref)
	s.Require().NotNil(data.Field.Props[0].BlocksMovement)
	s.True(*data.Field.Props[0].BlocksMovement)
	s.Require().NotNil(data.Field.Props[0].BlocksLineOfSight)
	s.False(*data.Field.Props[0].BlocksLineOfSight)

	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
	})
	s.Require().NoError(err)

	s.True(s.sees(back, delver, beyond), "still seen over after a reload")
	_, err = back.Step(&encounter.StepInput{Member: delver, To: propAbs(propAtX, 4)})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement, "and still solid")
}

// TestAnOldBlobsRoomsAreRefusedLoudly — the #1053/#1068 precedent, applied
// to the room chain (rpg-project#256). A field whose floor was rooms must not
// load as a field with no regions: that is a party dropped into a dungeon
// whose floor silently vanished, reported as "no regions" with nothing to say
// which dialect the blob is written in.
func (s *PropsSuite) TestAnOldBlobsRoomsAreRefusedLoudly() {
	enc := s.chamber("dnd5e:props:pillar", propTrue(), propTrue())

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)

	var generic map[string]any
	s.Require().NoError(json.Unmarshal(raw, &generic))

	field, _ := generic["field"].(map[string]any)
	s.Require().NotEmpty(field["regions"])

	// Rewind the field to the dialect that described the floor as rooms.
	delete(field, "regions")
	field["rooms"] = []any{map[string]any{
		"id": "crypt", "width": 12.0, "height": 8.0, "grid": "hex",
		"origin": map[string]any{"x": 20.0, "y": 10.0},
	}}

	rewound, err := json.Marshal(generic)
	s.Require().NoError(err)

	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(rewound, &data))

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
	})
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Contains(err.Error(), "rooms",
		"named, so whoever holds the blob knows which dialect it is written in")
	s.Contains(err.Error(), "rpg-project#256", "and which change to recreate it for")
}

// TestAPersistedPropMustSayWhatItDoesToo — the load seam's half of
// TestAPropMustSayWhatItDoes.
//
// Construction refusing a nil answer proves nothing about LOAD, which builds
// its rooms from a blob rather than from a caller's struct. A blob written by a
// build that did not ask is exactly the case the required-at-load rule exists
// for, and it is reachable by hand-editing JSON, which is what a stored
// encounter IS. Both mutants that defaulted a missing persisted answer survived
// the battery until this test existed.
func (s *PropsSuite) TestAPersistedPropMustSayWhatItDoesToo() {
	loadWith := func(mutate func(*encounter.PropData)) error {
		enc := s.chamber("dnd5e:props:pillar", propTrue(), propTrue())
		data := enc.ToData()
		mutate(&data.Field.Props[0])
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
			Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		})
		return err
	}

	s.Run("no movement answer", func() {
		err := loadWith(func(p *encounter.PropData) { p.BlocksMovement = nil })
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "blocks_movement")
		s.Contains(err.Error(), "dnd5e:props:pillar", "named, so the blob's owner can find it")
	})

	s.Run("no sight answer", func() {
		err := loadWith(func(p *encounter.PropData) { p.BlocksLineOfSight = nil })
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "blocks_line_of_sight")
	})
}

// TestEditingTheSetupAfterwardsCannotChangeTheSavedDungeon is T6 review M4's
// guarantee, asked of a field the caller holds by POINTER.
//
// compileField promises the persistence source never aliases caller-owned
// state, and copying the Props slice looked like it kept that promise — the
// elements are copied, after all. But a prop's two answers are pointers, so the
// copies pointed straight back at the caller's bools: flipping one afterwards
// rewrote what ToData would save, while the running encounter kept behaving the
// way it was built. A promise in a comment is not a mechanism, and this is the
// mechanism.
func (s *PropsSuite) TestEditingTheSetupAfterwardsCannotChangeTheSavedDungeon() {
	solid := true
	setup := &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("crypt", int(propsOrigin.X), int(propsOrigin.Y), 12, 8)},
			Props: []encounter.PropInput{{
				Ref:               "dnd5e:props:pillar",
				At:                propSeat(5, 4),
				BlocksMovement:    &solid,
				BlocksLineOfSight: &solid,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: delver, Kind: encounter.KindPlayer, Position: propSeat(2, 4)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	}

	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)

	// The caller changes its mind about its own struct, after the fact.
	solid = false

	data := enc.ToData()
	s.Require().Len(data.Field.Props, 1)
	s.Require().NotNil(data.Field.Props[0].BlocksMovement)
	s.True(*data.Field.Props[0].BlocksMovement,
		"the dungeon was built with a solid pillar and saves one")
	s.Require().NotNil(data.Field.Props[0].BlocksLineOfSight)
	s.True(*data.Field.Props[0].BlocksLineOfSight)

	// And the encounter that is actually running never wavered.
	_, err = enc.Step(&encounter.StepInput{Member: delver, To: propAbs(5, 4)})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
}

// TestASavedPropIsNotAliasedByTheSnapshot — snapshot immunity, the direction
// ToData's own doc comment promises: "mutating the returned EncounterData will
// not affect this Encounter".
func (s *PropsSuite) TestASavedPropIsNotAliasedByTheSnapshot() {
	enc := s.chamber("dnd5e:props:coffin", propTrue(), propFalse())

	first := enc.ToData()
	*first.Field.Props[0].BlocksMovement = false
	*first.Field.Props[0].BlocksLineOfSight = true

	second := enc.ToData()
	s.True(*second.Field.Props[0].BlocksMovement,
		"a second snapshot must not carry the first one's edits")
	s.False(*second.Field.Props[0].BlocksLineOfSight)
}
