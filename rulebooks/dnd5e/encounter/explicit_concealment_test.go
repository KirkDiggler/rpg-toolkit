// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ExplicitConcealmentSuite distinguishes authored secret membership from
// ordinary room discovery. A hidden floor cell is not an implicit selection
// of every placed thing whose independently authored footprint touches it.
type ExplicitConcealmentSuite struct{ suite.Suite }

func TestExplicitConcealmentSuite(t *testing.T) {
	suite.Run(t, new(ExplicitConcealmentSuite))
}

func (s *ExplicitConcealmentSuite) setup(field encounter.FieldInput, at spatial.Position) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field:   field,
		Members: []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: at}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

func (s *ExplicitConcealmentSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	out, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)
	return out
}

func (s *ExplicitConcealmentSuite) TestDoorSupportIsNotConcealedFloorOrAnOccupancyReveal() {
	field := footprintDoorField(encounter.DoorIsOpen())
	field.Concealments = []encounter.ConcealmentInput{{
		ID: "door-secret", Checks: vaultCheck(), Doors: []encounter.DoorID{theLeaf},
	}}
	enc := s.searchSetup(field, cellAt(2, 1))
	for _, current := range []*encounter.Encounter{enc, s.reload(enc)} {
		full, err := current.Atlas()
		s.Require().NoError(err)
		view, err := current.AtlasFor("walker")
		s.Require().NoError(err)
		s.Equal(full.Cells, view.Cells, "there are no explicitly concealed cells")
		_, err = current.OpenDoor(&encounter.OpenDoorInput{Door: theLeaf, Actor: "walker"})
		s.Require().ErrorIs(err, encounter.ErrNoDoor, "standing on support alone must not discover the secret")
	}
	_, err := enc.Search(&encounter.SearchInput{Member: "walker", Region: "hall"})
	s.Require().NoError(err, "a floor-less secret remains reachable by its door geometry")
	_, err = enc.CloseDoor(&encounter.CloseDoorInput{Door: theLeaf, Actor: "walker"})
	s.Require().NoError(err, "discovery still finds the selected door")
}

func (s *ExplicitConcealmentSuite) TestDoorOnlyConcealmentPreservesAdjacentFloorAndAnonymousRefusal() {
	field := footprintDoorField(encounter.DoorIsClosed())
	shape := thinWall(0.25, 8, 0, spatial.Point{X: 7.5, Y: 0})
	field.Doors[0].Placement = &shape
	field.Concealments = []encounter.ConcealmentInput{{
		ID: "door-secret", Checks: vaultCheck(), Doors: []encounter.DoorID{theLeaf},
	}}
	enc := s.setup(field, cellAt(3, 0))
	_, err := enc.Step(&encounter.StepInput{Member: "walker", To: cellAt(2, 0)})
	s.Require().NoError(err, "floor beside the actual rectangle must not become a full-hex mask")
	_, err = enc.Step(&encounter.StepInput{Member: "walker", To: cellAt(1, 0)})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "cannot cross movement-blocking boundary")
	s.NotContains(err.Error(), string(theLeaf), "a physical refusal must not name an unfound door")
	s.NotContains(err.Error(), "shut")
}

func (s *ExplicitConcealmentSuite) TestConcealedCellDoesNotSelectUnlistedWallOrProp() {
	for _, listed := range []bool{false, true} {
		name := "unlisted wall remains visible"
		if listed {
			name = "explicitly listed wall is withheld"
		}
		s.Run(name, func() {
			secret := cellAt(2, 2)
			authoredSecret := spatial.Position{X: 2, Y: 2}
			field := placedField(
				placed("wall-section", coveredBox(6, centreOf(secret)), true, true),
				placed("bookcase", coveredBox(1, centreOf(secret)), false, false),
			)
			field.Props = []encounter.PropInput{{ID: "legacy-decoration", Ref: "dnd5e:props:vase", At: authoredSecret, BlocksMovement: boolPtr(false), BlocksLineOfSight: boolPtr(false)}}
			concealment := encounter.ConcealmentInput{ID: "secret", Checks: vaultCheck(), Cells: []spatial.Position{authoredSecret}}
			if listed {
				concealment.Props = []encounter.PropID{"wall-section"}
			}
			field.Concealments = []encounter.ConcealmentInput{concealment}
			enc := s.setup(field, cellAt(0, 0))
			full, err := enc.Atlas()
			s.Require().NoError(err)
			s.Require().Len(full.Placed, 2)
			var wall encounter.AtlasPlacedProp
			for _, p := range full.Placed {
				if p.ID == "wall-section" {
					wall = p
				}
			}
			s.Require().Contains(wall.Cells, secret, "fixture must exercise the overlap filter")
			for _, current := range []*encounter.Encounter{enc, s.reload(enc)} {
				view, err := current.AtlasFor("walker")
				s.Require().NoError(err)
				s.NotContains(view.Cells, secret, "explicitly concealed floor stays withheld")
				ids := []string{}
				for _, p := range view.Placed {
					ids = append(ids, p.ID)
					if p.ID == "wall-section" {
						s.Equal(wall.Placement, p.Placement)
						s.True(p.BlocksMovement)
						s.True(p.BlocksLineOfSight)
					}
				}
				s.Contains(ids, "bookcase", "unlisted prop is not implicitly concealed by its floor")
				if listed {
					s.NotContains(ids, "wall-section")
				} else {
					s.Contains(ids, "wall-section")
				}
				s.Require().Len(view.Props, 1, "cell props obey the same explicit membership rule")
				s.Equal("legacy-decoration", view.Props[0].ID)
			}
		})
	}
}

// searchSetup is [setup] with a resolver whose every check is beaten, so a
// discovery scene can actually reveal its secret through the existing Search
// path.
func (s *ExplicitConcealmentSuite) searchSetup(field encounter.FieldInput, at spatial.Position) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsEverything{}, Witness: nobodyPerceives{},
		Field:   field,
		Members: []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: at}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// TestWallPresenceMembershipHidesIdentityAndSpansButNotUnlistedThings: the
// source names a wall; the lowering turned that into the wall's identity-only
// presence entry plus its blocking spans. Explicit membership withholds ALL of
// them, while a thing not listed on the same floor stays. Repeat after a
// save/load so the compiled membership survives the round trip.
func (s *ExplicitConcealmentSuite) TestWallPresenceMembershipHidesIdentityAndSpansButNotUnlistedThings() {
	for _, afterReload := range []bool{false, true} {
		name := "before reload"
		if afterReload {
			name = "after reload"
		}
		s.Run(name, func() {
			secret := cellAt(2, 2)
			authoredSecret := spatial.Position{X: 2, Y: 2}
			field := placedField(
				placed("wall-1", coveredBox(1, centreOf(secret)), false, false),
				placed("wall/wall-1/span/0", coveredBox(1, centreOf(secret)), true, true),
				placed("wall/wall-1/span/1", coveredBox(1, centreOf(secret)), true, true),
				placed("bench", coveredBox(1, centreOf(secret)), false, false),
			)
			field.Concealments = []encounter.ConcealmentInput{{
				ID:     "vault",
				Checks: vaultCheck(),
				Cells:  []spatial.Position{authoredSecret},
				Props:  []encounter.PropID{"wall-1", "wall/wall-1/span/0", "wall/wall-1/span/1"},
			}}
			enc := s.setup(field, cellAt(0, 0))
			if afterReload {
				enc = s.reload(enc)
			}
			full, err := enc.Atlas()
			s.Require().NoError(err)
			s.Require().Len(full.Placed, 4, "the whole field carries every row")

			view, err := enc.AtlasFor("walker")
			s.Require().NoError(err)
			ids := map[string]bool{}
			for _, p := range view.Placed {
				ids[p.ID] = true
			}
			s.False(ids["wall-1"], "the presence entry is withheld by membership")
			s.False(ids["wall/wall-1/span/0"], "and the first span")
			s.False(ids["wall/wall-1/span/1"], "and the second")
			s.True(ids["bench"], "an unlisted thing on the same floor stays")
			s.NotContains(view.Cells, secret, "the concealed floor stays withheld")
		})
	}
}

// TestConcealedCellsAloneDoNotHideAnUnlistedWallPresence is the ordinary
// concealment law applied to the new presence row: a hidden hex does not
// implicitly select the unlisted wall whose blocker touches it, nor its spans.
func (s *ExplicitConcealmentSuite) TestConcealedCellsAloneDoNotHideAnUnlistedWallPresence() {
	secret := cellAt(2, 2)
	authoredSecret := spatial.Position{X: 2, Y: 2}
	field := placedField(
		placed("unlisted-wall", coveredBox(1, centreOf(secret)), false, false),
		placed("wall/unlisted-wall/span/0", coveredBox(1, centreOf(secret)), true, true),
	)
	field.Concealments = []encounter.ConcealmentInput{{
		ID: "vault", Checks: vaultCheck(), Cells: []spatial.Position{authoredSecret},
	}}
	enc := s.setup(field, cellAt(0, 0))
	for _, current := range []*encounter.Encounter{enc, s.reload(enc)} {
		view, err := current.AtlasFor("walker")
		s.Require().NoError(err)
		s.NotContains(view.Cells, secret, "the concealed floor stays withheld")
		ids := map[string]bool{}
		for _, p := range view.Placed {
			ids[p.ID] = true
		}
		s.True(ids["unlisted-wall"], "an unlisted presence is not implied by its floor")
		s.True(ids["wall/unlisted-wall/span/0"], "nor is its blocking span")
	}
}

// TestDiscoveryRevealsTheSelectedWallIdentities proves the existing reveal
// mechanism reaches the new membership: after Search finds the secret, the
// wall's presence and its span appear for the searcher.
func (s *ExplicitConcealmentSuite) TestDiscoveryRevealsTheSelectedWallIdentities() {
	secret := cellAt(2, 2)
	authoredSecret := spatial.Position{X: 2, Y: 2}
	field := placedField(
		placed("wall-1", coveredBox(1, centreOf(secret)), false, false),
		placed("wall/wall-1/span/0", coveredBox(1, centreOf(secret)), true, true),
	)
	field.Concealments = []encounter.ConcealmentInput{{
		ID:     "vault",
		Checks: vaultCheck(),
		Cells:  []spatial.Position{authoredSecret},
		Props:  []encounter.PropID{"wall-1", "wall/wall-1/span/0"},
	}}
	enc := s.searchSetup(field, cellAt(0, 0))

	before, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(before.Placed, "an unaware observer sees no wall identity")

	_, err = enc.Search(&encounter.SearchInput{Member: "walker", Region: "hall"})
	s.Require().NoError(err)

	after, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	ids := map[string]bool{}
	for _, p := range after.Placed {
		ids[p.ID] = true
	}
	s.True(ids["wall-1"], "discovery reveals the presence entry")
	s.True(ids["wall/wall-1/span/0"], "and the span")
	s.Contains(after.Cells, secret, "and the floor the secret hid")
}

func (s *ExplicitConcealmentSuite) TestOrdinaryUndiscoveredRoomStillWithholdsItsContents() {
	field := encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 3, 4), rectRegion("vault", 3, 0, 3, 4)},
		Walls:   seamWallExcept(2, 4, 1),
		Doors:   []encounter.DoorInput{{ID: "ordinary-door", Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed()}},
		Placed:  []encounter.PlacedPropInput{placed("unknown-wall", coveredBox(1, centreOf(cellAt(4, 2))), true, true)},
		Props:   []encounter.PropInput{{ID: "unknown-vase", Ref: "dnd5e:props:vase", At: cellAt(4, 3), BlocksMovement: boolPtr(false), BlocksLineOfSight: boolPtr(false)}},
	}
	enc := s.setup(field, cellAt(2, 1))
	for _, current := range []*encounter.Encounter{enc, s.reload(enc)} {
		before, err := current.AtlasFor("walker")
		s.Require().NoError(err)
		s.Empty(before.Placed)
		s.Empty(before.Props)
		s.Require().Len(before.Regions, 1)
		s.Equal("hall", before.Regions[0].ID)
		_, err = current.OpenDoor(&encounter.OpenDoorInput{Door: "ordinary-door", Actor: "walker"})
		s.Require().NoError(err)
		after, err := current.AtlasFor("walker")
		s.Require().NoError(err)
		s.Require().Len(after.Placed, 1)
		s.Equal("unknown-wall", after.Placed[0].ID)
		s.Require().Len(after.Props, 1)
		s.Equal("unknown-vase", after.Props[0].ID)
	}
}
