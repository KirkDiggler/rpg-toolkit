// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// arrival_test.go is ONE ENDING RULE, EVERY WAY IN (rpg-toolkit#1108).
//
// A ReachedPosition ending has been implemented twice before: #1059 found the
// pair — encounter.go's firedReachedPosition and an inline copy inside Join —
// and S0 deleted the copy. This file is what stops a third from growing back,
// from the outside; the structural half of that pin lives in
// oneplace_internal_test.go.
//
// The rules the trigger carries, all four:
//
//	R1  a PLAYER arrives, filter empty          -> fires
//	R2  a MONSTER arrives, filter empty         -> does NOT fire (empty means players)
//	R3  a MONSTER arrives, filter names it      -> fires (a named filter overrides kind)
//	R4  a PLAYER arrives, filter names SOMEBODY ELSE -> does NOT fire
//
// The ways in: Step (a host walks somebody), Pump (a monster acts on its own
// intel) and Join (somebody arrives mid-scene, possibly straight onto the
// tile). Pump carries only the monster rules, because Pump only moves monsters
// — stated rather than quietly skipped.
//
// The fixture seals the vault off from the annex where the watching player
// stands. That is load-bearing rather than scenery: sight is geometry since S0,
// a player who sees a monster starts a fight, and a member in a fight can
// neither Step nor be Pumped. The wall is what keeps each case's arrival the
// only thing happening.
type ArrivalSuite struct {
	suite.Suite
}

func TestArrivalSuite(t *testing.T) {
	suite.Run(t, new(ArrivalSuite))
}

const (
	arrivalVault = "vault"
	arrivalAnnex = "annex"
)

var (
	arrivalVaultOrigin = spatial.Position{X: 20, Y: 7}
	arrivalAnnexOrigin = spatial.Position{X: 28, Y: 7}

	// The ending tile, and the cell beside it a walker starts on — both
	// room-local in the vault.
	arrivalTarget = spatial.Position{X: 4, Y: 4}
	arrivalStart  = spatial.Position{X: 3, Y: 4}
)

// arrivalField is two chambers with NO doorway and a fully sealed seam.
func arrivalField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
		Regions: []encounter.RegionInput{
			rectRegion(arrivalVault, int(arrivalVaultOrigin.X), int(arrivalVaultOrigin.Y), 8, 8),
			rectRegion(arrivalAnnex, int(arrivalAnnexOrigin.X), int(arrivalAnnexOrigin.Y), 4, 8),
		},
		Walls: seamWallRows(int(arrivalVaultOrigin.X)+7, int(arrivalVaultOrigin.Y), 8),
	}
}

// vaultCell is the dungeon-absolute axial cell a vault-local pair names.
func vaultCell(local spatial.Position) spatial.Position {
	seat := local.Add(arrivalVaultOrigin)
	return cellAt(int(seat.X), int(seat.Y))
}

// arrivalEncounter opens a scene whose only ending is a ReachedPosition on the
// vault's tile. dave always watches from the sealed annex, and is who "somebody
// else" means. A walker is placed in the vault only when the case needs one to
// move; Join's cases need the vault empty.
func (s *ArrivalSuite) arrivalEncounter(filter encounter.MemberID, walker encounter.MemberKind) *encounter.Encounter {
	members := []encounter.MemberInput{
		{ID: dave, Kind: encounter.KindPlayer, Position: arrivalAnnexOrigin},
	}
	if walker != "" {
		w := encounter.MemberInput{ID: alice, Kind: walker, Position: arrivalStart.Add(arrivalVaultOrigin)}
		if walker == encounter.KindMonster {
			w.Decider = &patrolDecider{positions: []spatial.Position{vaultCell(arrivalTarget)}}
		}
		members = append(members, w)
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   arrivalField(),
		Members: members,
		Endings: []encounter.EndingInput{
			{Key: "found-it", Trigger: encounter.TriggerReachedPosition{
				Position: arrivalTarget.Add(arrivalVaultOrigin), Member: filter}},
		},
	})
	s.Require().NoError(err)
	return enc
}

// whoFilter names whose ID an ending's filter carries, resolved per path so
// that "the arriver" means the member each path actually moves.
type whoFilter int

const (
	filterNobody whoFilter = iota
	filterTheArriver
	filterSomebodyElse
)

// arrivalCases is the rule table every path below runs against.
var arrivalCases = []struct {
	name   string
	kind   encounter.MemberKind
	filter whoFilter
	fires  bool
}{
	{"R1 player, unfiltered", encounter.KindPlayer, filterNobody, true},
	{"R2 monster, unfiltered", encounter.KindMonster, filterNobody, false},
	{"R3 monster, filtered to the arriver", encounter.KindMonster, filterTheArriver, true},
	{"R4 player, filtered to somebody else", encounter.KindPlayer, filterSomebodyElse, false},
}

// filterID resolves a case's filter against whoever is arriving.
func filterID(f whoFilter, arriver encounter.MemberID) encounter.MemberID {
	switch f {
	case filterTheArriver:
		return arriver
	case filterSomebodyElse:
		return dave
	case filterNobody:
		return ""
	}
	return ""
}

// TestAStepDecidesTheEndingByTheSameRules — the host walks somebody onto the
// tile.
func (s *ArrivalSuite) TestAStepDecidesTheEndingByTheSameRules() {
	for _, tc := range arrivalCases {
		s.Run(tc.name, func() {
			enc := s.arrivalEncounter(filterID(tc.filter, alice), tc.kind)
			out, err := enc.Step(&encounter.StepInput{
				Member: alice, To: vaultCell(arrivalTarget)})
			s.Require().NoError(err)
			s.assertFired(tc.fires, out.Outcome != nil, enc)
		})
	}
}

// TestAPumpDecidesTheEndingByTheSameRules — a monster walks itself onto the
// tile on its own intel. Only the monster rules apply.
func (s *ArrivalSuite) TestAPumpDecidesTheEndingByTheSameRules() {
	for _, tc := range arrivalCases {
		if tc.kind != encounter.KindMonster {
			continue
		}
		s.Run(tc.name, func() {
			enc := s.arrivalEncounter(filterID(tc.filter, alice), tc.kind)
			_, err := enc.Pump(&encounter.PumpInput{})
			s.Require().NoError(err)
			s.assertFired(tc.fires, !s.open(enc), enc)
		})
	}
}

// TestAJoinDecidesTheEndingByTheSameRules — somebody arrives mid-scene straight
// onto the tile, which is the path that used to carry its own copy of the rule.
func (s *ArrivalSuite) TestAJoinDecidesTheEndingByTheSameRules() {
	for _, tc := range arrivalCases {
		s.Run(tc.name, func() {
			enc := s.arrivalEncounter(filterID(tc.filter, bob), "")
			out, err := enc.Join(&encounter.JoinInput{
				Member: bob, Kind: tc.kind,
				Cell: vaultCell(arrivalTarget),
			})
			s.Require().NoError(err)
			s.assertFired(tc.fires, out.Outcome != nil, enc)
		})
	}
}

// TestAnEndingIsOneCellNotOneColumn pins that the comparison is a CELL, both
// axes, in both directions.
//
// The ending is compiled to a single absolute cell and an arrival is an
// equality against it (compileEndings, firedReachedPosition). An equality that
// dropped either axis would still pass every scene above, because each of them
// arrives exactly on the tile — so the cells that matter here are the ones that
// share ONE coordinate with it and miss on the other.
func (s *ArrivalSuite) TestAnEndingIsOneCellNotOneColumn() {
	near := []struct {
		name string
		cell spatial.Position
	}{
		{"same column, one row up", spatial.Position{X: arrivalTarget.X, Y: arrivalTarget.Y - 1}},
		{"same row, one column back", spatial.Position{X: arrivalTarget.X - 1, Y: arrivalTarget.Y}},
	}
	for _, tc := range near {
		s.Run(tc.name, func() {
			enc := s.arrivalEncounter("", encounter.KindPlayer)
			out, err := enc.Step(&encounter.StepInput{
				Member: alice, To: vaultCell(tc.cell)})
			s.Require().NoError(err)
			s.assertFired(false, out.Outcome != nil, enc)

			// And from there, the tile itself still closes it — so the miss
			// above was the cell, not the walk.
			out, err = enc.Step(&encounter.StepInput{
				Member: alice, To: vaultCell(arrivalTarget)})
			s.Require().NoError(err)
			s.assertFired(true, out.Outcome != nil, enc)
		})
	}
}

// assertFired checks the ending and the encounter's own state agree: a fired
// ending closes the encounter, and a closed encounter fired one.
func (s *ArrivalSuite) assertFired(want, got bool, enc *encounter.Encounter) {
	s.Equal(want, got, "the ending")
	s.Equal(want, !s.open(enc), "and the encounter's state must agree with it")
}

func (s *ArrivalSuite) open(enc *encounter.Encounter) bool {
	status, err := enc.Status()
	s.Require().NoError(err)
	return status.Open
}
