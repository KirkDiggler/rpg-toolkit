// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const joinsPath = "testdata/joins-site.yaml"

// joinKinds are the three kinds of join (rpg-project#971, design R30).
var joinKinds = map[readback.Kind]bool{
	readback.KindJoinDoor: true, readback.KindJoinOpen: true, readback.KindJoinPassage: true,
}

// joinsOf is every join statement of a read-back, in reading order.
func joinsOf(rb *readback.ReadBack) []readback.Statement {
	var out []readback.Statement
	for _, st := range rb.Statements() {
		if joinKinds[st.Kind] {
			out = append(out, st)
		}
	}
	return out
}

// connected is whether a statement carries the connection.
func connected(st readback.Statement, kind, id, role string) bool {
	for _, c := range st.Connects {
		if c.Kind == kind && c.ID == id && c.Role == role {
			return true
		}
	}
	return false
}

// regionsOf is the regions a join connects, by role.
func regionsOf(st readback.Statement) map[string]string {
	out := map[string]string{}
	for _, c := range st.Connects {
		if c.Kind == "region" {
			out[c.ID] = c.Role
		}
	}
	return out
}

// JoinsCastleSuite holds the castle's joins to its map.
type JoinsCastleSuite struct {
	suite.Suite
	rb       *readback.ReadBack
	compiled dungeonspec.Compiled
	regions  []dungeonspec.RoomRegion
	doors    int
}

func TestJoinsCastleSuite(t *testing.T) { suite.Run(t, new(JoinsCastleSuite)) }

func (s *JoinsCastleSuite) SetupSuite() {
	raw, err := os.ReadFile(castlePath)
	s.Require().NoError(err)
	s.rb, err = readback.Of(raw)
	s.Require().NoError(err)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.regions = decoded.Spec.Room.Gameplay.Regions
	s.doors = len(decoded.Spec.Room.Gameplay.DoorBindings)
	s.compiled, err = dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: decoded.Spec})
	s.Require().NoError(err)
}

// byConnection is the joins of kind carrying the connection.
func (s *JoinsCastleSuite) byConnection(kind readback.Kind, c readback.Connection) []readback.Statement {
	var out []readback.Statement
	for _, st := range joinsOf(s.rb) {
		if st.Kind == kind && connected(st, c.Kind, c.ID, c.Role) {
			out = append(out, st)
		}
	}
	return out
}

// TestJoins_Castle_GateDoorJoinsGatehouseAndCourtyard: the gate wall runs at
// scene z 9.75; its door at the wall's middle (scene x 0) stands between the
// Courtyard's row 6 (z 9) and the Gatehouse's row 7 (z 10.5), as the castle's
// region rows place them. One door join, by the door, between those two.
func (s *JoinsCastleSuite) TestJoins_Castle_GateDoorJoinsGatehouseAndCourtyard() {
	got := s.byConnection(readback.KindJoinDoor, readback.Connection{Kind: "door", ID: "gate-wall-door", Role: "by"})
	s.Require().Len(got, 1, "the gate door is one join")
	j := got[0]
	s.Equal(map[string]string{"region-gatehouse": "joins", "region-courtyard": "joins"}, regionsOf(j))
	s.Equal("join", j.About.Kind)
	s.Contains(j.Sentence, "Courtyard (region-courtyard)")
	s.Contains(j.Sentence, "Gatehouse (region-gatehouse)")
	s.Contains(j.Sentence, "`gate-wall-door`")
	s.Equal("room.room.doorBindings.gate-wall-door", j.Path)
}

// TestJoins_Castle_EveryDoorIsAJoin: each of the castle's seven doors stands
// between two regions, so each says one door join.
func (s *JoinsCastleSuite) TestJoins_Castle_EveryDoorIsAJoin() {
	s.Require().Equal(7, s.doors)
	for _, d := range s.compiled.Field.Doors {
		id := strings.TrimPrefix(string(d.ID), s.compiled.Key+"/")
		got := s.byConnection(readback.KindJoinDoor, readback.Connection{Kind: "door", ID: id, Role: "by"})
		s.Require().Len(got, 1, "door %s", id)
		s.Len(regionsOf(got[0]), 2, "door %s joins two regions", id)
	}
}

// TestJoins_Castle_GrateIsAPassageToChapelAndCells: the grate stands in the
// Kitchen, and its two things to do set the doer down in the Chapel and in
// the Cells. Each is a passage carried by the grate, hidden by the secret
// that hides the grate.
func (s *JoinsCastleSuite) TestJoins_Castle_GrateIsAPassageToChapelAndCells() {
	got := s.byConnection(readback.KindJoinPassage, readback.Connection{Kind: "prop", ID: "grate", Role: "by"})
	s.Require().Len(got, 2)
	to := map[string]bool{}
	for _, j := range got {
		s.True(connected(j, "region", "region-kitchen", "from"), j.Sentence)
		s.True(connected(j, "concealment", "kitchen-passage", "hidden-by"), j.Sentence)
		for id, role := range regionsOf(j) {
			if role == "to" {
				to[id] = true
			}
		}
		s.True(strings.HasPrefix(j.Path, "room.room.propBindings.grate.options["), j.Path)
	}
	s.Equal(map[string]bool{"region-chapel": true, "region-cells": true}, to)
}

// TestJoins_Castle_ArrestIsAPassageToCells: the gate's arrest takes the party
// from its area to the Cells, and its jail sets them down. The area is the
// Gatehouse and the road cells of rows 13 and 14 just outside its south
// doorway, which the castle's rows give to the outer region `room-1-region`
// (rows 7 to 17 around the gatehouse).
func (s *JoinsCastleSuite) TestJoins_Castle_ArrestIsAPassageToCells() {
	got := s.byConnection(readback.KindJoinPassage, readback.Connection{Kind: "conversation", ID: "front-gate", Role: "by"})
	s.Require().Len(got, 1)
	j := got[0]
	s.Equal(map[string]string{"region-gatehouse": "from", "room-1-region": "from", "region-cells": "to"}, regionsOf(j))
	s.Equal("interactions.front-gate.outcomes.arrested[0].arrest", j.Path)
	s.Contains(j.Sentence, "`jailed`")
}

// TestJoins_Castle_EveryRegionIsJoinedOrSaidUnjoined: a region with cells is
// in at least one join or is said to be in none, never both.
func (s *JoinsCastleSuite) TestJoins_Castle_EveryRegionIsJoinedOrSaidUnjoined() {
	joined := map[string]bool{}
	for _, j := range joinsOf(s.rb) {
		for id := range regionsOf(j) {
			joined[id] = true
		}
	}
	unjoined := map[string]bool{}
	for _, st := range s.rb.Statements() {
		if st.Kind == readback.KindRegionUnjoined {
			unjoined[st.About.ID] = true
		}
	}
	for _, r := range s.regions {
		if len(r.Cells) == 0 {
			continue
		}
		s.True(joined[r.ID] != unjoined[r.ID], "region %s: joined %v, said unjoined %v", r.ID, joined[r.ID], unjoined[r.ID])
	}
}

// TestJoins_Castle_OpenJoinsShareAPartition: the hazard. Two regions whose
// cells touch across a wall are not joined; an open join's two regions are
// always one wall partition of the floor ([encounter.PartitionFloor]).
func (s *JoinsCastleSuite) TestJoins_Castle_OpenJoinsShareAPartition() {
	parts, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: s.compiled.Field})
	s.Require().NoError(err)
	partOf := map[spatial.Position]int{}
	for i, cells := range parts.Components {
		for _, c := range cells {
			partOf[c] = i
		}
	}
	partsOfRegion := map[string]map[int]bool{}
	for _, r := range s.compiled.Field.Regions {
		partsOfRegion[string(r.ID)] = map[int]bool{}
		for _, c := range r.Cells {
			partsOfRegion[string(r.ID)][partOf[c]] = true
		}
	}
	for _, j := range joinsOf(s.rb) {
		if j.Kind != readback.KindJoinOpen {
			continue
		}
		var ids []string
		for id := range regionsOf(j) {
			ids = append(ids, id)
		}
		s.Require().Len(ids, 2)
		shared := false
		for p := range partsOfRegion[ids[0]] {
			shared = shared || partsOfRegion[ids[1]][p]
		}
		s.True(shared, "%s: its two regions share no partition", j.Sentence)
	}
}

// JoinsSiteSuite reads the small joins site: two regions on one floor.
type JoinsSiteSuite struct {
	suite.Suite
	raw string
}

func TestJoinsSiteSuite(t *testing.T) { suite.Run(t, new(JoinsSiteSuite)) }

func (s *JoinsSiteSuite) SetupTest() {
	raw, err := os.ReadFile(joinsPath)
	s.Require().NoError(err)
	s.raw = string(raw)
}

// wallAt is one wall along scene x from z -2 to z 2, blocking feet and sight
// as told, with a door in its middle when door is not empty.
func wallAt(id string, x float64, movement, sight bool, door string) string {
	openings := "        openings: []\n"
	if door != "" {
		openings = "        openings:\n          - id: " + id + "-gap\n            position: 2\n            width: 1.5\n" +
			"            door: {id: " + door + ", assetRef: 'dnd5e:env:test:door'}\n"
	}
	return fmt.Sprintf(`      - id: %s
        label: %s
        line:
          start: {x: %g, z: -2}
          end: {x: %g, z: 2}
        appearance: {assetRef: 'dnd5e:env:test:wall', height: 3, thickness: 0.3, elevation: 0}
        blocker:
          blocksMovement: %t
          blocksLineOfSight: %t
          footprint: {width: 4, depth: 0.5, offsetX: 0, offsetZ: 0}
`, id, id, x, x, movement, sight) + openings
}

// seamX runs between q 2 (scene x 3.46) and q 3 (x 5.20) of row 0; edgeX runs
// just east of q 5 (x 8.66), where the floor ends.
const seamX, edgeX = 4.33, 9.53

// withWalls is the joins site with the walls (and the door bindings) given.
func (s *JoinsSiteSuite) withWalls(walls string, doors ...string) []byte {
	block := "    walls:\n" + walls
	if len(doors) > 0 {
		block += "    doorBindings:\n"
		for _, d := range doors {
			block += "      " + d + ": {closed: true}\n"
		}
	}
	edited := strings.Replace(s.raw, "    monsterDeclarations: []\n", "    monsterDeclarations: []\n"+block, 1)
	s.Require().NotEqual(s.raw, edited)
	return []byte(edited)
}

// ofKind is the statements of a kind.
func ofKind(rb *readback.ReadBack, kind readback.Kind) []readback.Statement {
	var out []readback.Statement
	for _, st := range rb.Statements() {
		if st.Kind == kind {
			out = append(out, st)
		}
	}
	return out
}

// TestJoins_OpenFloor_TwoRegionsNoWall: nothing built between the rooms, so
// they meet on open floor — one join, and neither room is said unjoined.
func (s *JoinsSiteSuite) TestJoins_OpenFloor_TwoRegionsNoWall() {
	rb, err := readback.Of([]byte(s.raw))
	s.Require().NoError(err)
	joins := joinsOf(rb)
	s.Require().Len(joins, 1)
	s.Equal(readback.KindJoinOpen, joins[0].Kind)
	s.Equal(map[string]string{"region-west": "joins", "region-east": "joins"}, regionsOf(joins[0]))
	s.Contains(joins[0].Sentence, "East room (region-east)")
	s.Contains(joins[0].Sentence, "West room (region-west)")
	for _, st := range rb.Statements() {
		s.NotEqual(readback.KindRegionUnjoined, st.Kind, st.Sentence)
	}
	s.Empty(rb.NotYetSaid)
}

// TestJoins_WallBetween_NoJoin: a wall on the seam leaves the two rooms
// touching but walled apart: no join, and each is said joined to nothing.
func (s *JoinsSiteSuite) TestJoins_WallBetween_NoJoin() {
	rb, err := readback.Of(s.withWalls(wallAt("seam-wall", seamX, true, true, "")))
	s.Require().NoError(err)
	s.Empty(joinsOf(rb))
	s.Empty(ofKind(rb, readback.KindSeamSight))
	unjoined := map[string]string{}
	for _, st := range ofKind(rb, readback.KindRegionUnjoined) {
		unjoined[st.About.ID] = st.Path
	}
	s.Equal(map[string]string{
		"region-west": "room.room.regions[1]",
		"region-east": "room.room.regions[2]",
	}, unjoined)
}

// TestJoins_Window_IsASightSeamNotAJoin: a wall that stops feet and lets
// sight through is no way between the rooms. It is said as a sight seam,
// and each room is still joined to nothing.
func (s *JoinsSiteSuite) TestJoins_Window_IsASightSeamNotAJoin() {
	rb, err := readback.Of(s.withWalls(wallAt("window", seamX, true, false, "")))
	s.Require().NoError(err)
	s.Empty(joinsOf(rb))
	seams := ofKind(rb, readback.KindSeamSight)
	s.Require().Len(seams, 1)
	s.Equal(readback.Subject{Kind: "seam", ID: "sight:region-east:region-west"}, seams[0].About)
	s.Equal(map[string]string{"region-west": "sees", "region-east": "sees"}, regionsOf(seams[0]))
	s.Len(ofKind(rb, readback.KindRegionUnjoined), 2)
}

// TestJoins_Curtain_IsAJoinSightCannotCross: a wall that lets feet through
// and stops sight is a way between the rooms, said as open floor whose
// sentence says sight is blocked there.
func (s *JoinsSiteSuite) TestJoins_Curtain_IsAJoinSightCannotCross() {
	rb, err := readback.Of(s.withWalls(wallAt("curtain", seamX, false, true, "")))
	s.Require().NoError(err)
	joins := joinsOf(rb)
	s.Require().Len(joins, 1)
	s.Equal(readback.KindJoinOpen, joins[0].Kind)
	s.Contains(joins[0].Sentence, "sight does not cross there")
	s.NotContains(joins[0].Sentence, "sight, hearing and feet alike")
	s.Empty(ofKind(rb, readback.KindRegionUnjoined))
}

// TestJoins_DoorWithinOneRegion_IsNoJoin: a door whose two sides are one
// region's floor joins nothing, and is said to stand within that region.
func (s *JoinsSiteSuite) TestJoins_DoorWithinOneRegion_IsNoJoin() {
	one := strings.Replace(s.raw, "          - r: 0\n            q: [[0, 2]]", "          - r: 0\n            q: [[0, 5]]", 1)
	one = strings.Replace(one, `      - id: region-east
        name: East room
        lighting: { intensity: 1 }
        cells:
          - r: 0
            q: [[3, 5]]
`, "", 1)
	s.Require().NotContains(one, "region-east")
	s.raw = one
	rb, err := readback.Of(s.withWalls(wallAt("inner-wall", seamX, true, true, "inner-door"), "inner-door"))
	s.Require().NoError(err)
	s.Empty(joinsOf(rb), "a door within one region joins it to nothing")
	within := ofKind(rb, readback.KindDoorWithin)
	s.Require().Len(within, 1)
	s.Equal(readback.Subject{Kind: "door", ID: "inner-door"}, within[0].About)
	s.Equal(map[string]string{"region-west": "within"}, regionsOf(within[0]))
	s.Equal("room.room.doorBindings.inner-door", within[0].Path)
}

// TestJoins_DoorOffTheFloor_IsSaidWithItsRegion: the east room is walled off
// from the west, and its one door leads off the floor. The door is said at
// the edge of the east room, and the east room's "joins no other region"
// names that door rather than saying it has none.
func (s *JoinsSiteSuite) TestJoins_DoorOffTheFloor_IsSaidWithItsRegion() {
	rb, err := readback.Of(s.withWalls(
		wallAt("seam-wall", seamX, true, true, "")+wallAt("outer-wall", edgeX, true, true, "outer-door"), "outer-door"))
	s.Require().NoError(err)
	s.Empty(joinsOf(rb))
	edge := ofKind(rb, readback.KindDoorEdge)
	s.Require().Len(edge, 1)
	s.Equal(readback.Subject{Kind: "door", ID: "outer-door"}, edge[0].About)
	s.Equal(map[string]string{"region-east": "edge"}, regionsOf(edge[0]))
	var east readback.Statement
	for _, st := range ofKind(rb, readback.KindRegionUnjoined) {
		if st.About.ID == "region-east" {
			east = st
		}
	}
	s.Contains(east.Sentence, "The door `outer-door` at East room leads off the floor.")
	s.True(connected(east, "door", "outer-door", "edge"))
}

// TestJoins_StableIDs: a join's id is built from its regions and what carries
// it, so reading the same file twice names every join alike, and no two
// joins share an id.
func (s *JoinsSiteSuite) TestJoins_StableIDs() {
	first, err := readback.Of([]byte(s.raw))
	s.Require().NoError(err)
	second, err := readback.Of([]byte(s.raw))
	s.Require().NoError(err)
	s.Require().Len(joinsOf(first), 1)
	s.Equal("open:region-east:region-west", joinsOf(first)[0].About.ID)
	s.Equal(joinsOf(first), joinsOf(second))
}

// TestJoins_Castle_IDsNameTheirRegions: every castle join's id is built from
// the regions it connects and what carries it, and no two share one. The
// list is the castle's map read by hand: the seven doors, the two open wings
// of the courtyard, the grate's two ways and the arrest.
func (s *JoinsCastleSuite) TestJoins_Castle_IDsNameTheirRegions() {
	var ids []string
	for _, j := range joinsOf(s.rb) {
		ids = append(ids, j.About.ID)
		for id := range regionsOf(j) {
			s.Contains(j.About.ID, id, "the id names its regions")
		}
	}
	s.Equal([]string{
		"door:region-gatehouse:room-1-region:26369b41-e58f-475d-963a-95086d4cd868",
		"door:region-barracks:region-cells:cells-wall-door",
		"door:region-chapel:region-great-hall:chapel-wall-door",
		"door:region-courtyard:region-gatehouse:gate-wall-door",
		"door:region-courtyard:region-great-hall:hall-wall-door",
		"door:region-great-hall:region-kitchen:kitchen-wall-door",
		"door:region-great-hall:region-throne-room:throne-wall-door",
		"open:region-armoury:region-courtyard",
		"open:region-barracks:region-courtyard",
		"passage:region-kitchen:region-chapel:prop-grate",
		"passage:region-kitchen:region-cells:prop-grate",
		"passage:region-gatehouse+room-1-region:region-cells:conversation-front-gate",
	}, ids)
}
