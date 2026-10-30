// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// partitions_test.go pins the field's two divisions (rpg-project#725, design
// R4 and R8): the authored regions are what the map names and lights, the
// wall partitions are the rooms the walls make, the two are independent, and
// the partitions are derived again at every load from what the data carries
// — never stored in it. The tests are [AtlasRegionsSuite]'s, which opens and
// reloads a field.

import (
	"encoding/json"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// yardAndGatehouse is an open yard painted as two authored regions — the yard
// lit at 1, the gatehouse at 0.3 — with nothing built between them, and a
// walled-off chapel beside the gatehouse behind a shut door. Alice stands in
// the yard at [6,4].
func yardAndGatehouse() encounter.FieldInput {
	gatehouse := rectRegion("region-gatehouse", 10, 4, 3, 3)
	gatehouse.Name = "Gatehouse"
	gatehouse.Lighting = &encounter.Lighting{Intensity: 0.3}
	yard := rectRegion("region-yard", 6, 4, 4, 3)
	yard.Name = "Yard"
	return encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{gatehouse, yard, rectRegion("chapel", 13, 4, 2, 3)},
		Walls:   seamWallRows(12, 4, 3, 5),
		Doors: []encounter.DoorInput{{ID: "chapel-door", State: encounter.DoorIsClosed(),
			Edges: []encounter.DoorEdge{{From: cellAt(12, 5), To: cellAt(13, 5)}}}},
	}
}

// TestAtlas_ListsAuthoredRegionsWithTheirLighting: the atlas lists the regions
// the author painted, each with its own name and lighting, sorted by id —
// not the rooms the walls make.
func (s *AtlasRegionsSuite) TestAtlas_ListsAuthoredRegionsWithTheirLighting() {
	atlas, err := s.open(yardAndGatehouse()).Atlas()
	s.Require().NoError(err)
	type entry struct {
		id, name  string
		intensity float64
	}
	var got []entry
	for _, r := range atlas.Regions {
		got = append(got, entry{r.ID, r.Name, r.Lighting.Intensity})
	}
	s.Equal([]entry{{"chapel", "chapel", 1}, {"region-gatehouse", "Gatehouse", 0.3}, {"region-yard", "Yard", 1}}, got)
}

// TestRegionAt_IsTheAuthoredRegion: a cell painted into the gatehouse answers
// the gatehouse, even though the room the walls make spans the yard too; the
// partition says the second thing and RegionAt never does.
func (s *AtlasRegionsSuite) TestRegionAt_IsTheAuthoredRegion() {
	enc := s.open(yardAndGatehouse())
	gate, yard, chapel := cellAt(11, 5), cellAt(7, 5), cellAt(13, 5)

	region, ok := enc.RegionAt(gate)
	s.Require().True(ok)
	s.Equal("region-gatehouse", region)
	region, _ = enc.RegionAt(yard)
	s.Equal("region-yard", region)

	gateRoom, _ := enc.PartitionAt(gate)
	yardRoom, _ := enc.PartitionAt(yard)
	chapelRoom, _ := enc.PartitionAt(chapel)
	s.Equal(yardRoom, gateRoom, "nothing is built between the yard and the gatehouse")
	s.NotEqual(gateRoom, chapelRoom, "the chapel wall makes another room")

	_, ok = enc.PartitionAt(cellAt(30, 30))
	s.False(ok, "void is in no partition")
}

// TestFieldLoad_DerivesTheSamePartitions: Setup and Load derive the walls'
// division from the same persisted regions, walls and doors, so every cell
// answers the same partition after a reload — and the data that crossed
// carries no partition at all (R8).
func (s *AtlasRegionsSuite) TestFieldLoad_DerivesTheSamePartitions() {
	for name, field := range map[string]encounter.FieldInput{
		"yard and gatehouse": yardAndGatehouse(),
		"the atlas field":    atlasField(),
	} {
		s.Run(name, func() {
			enc := s.open(field)
			back := s.reload(enc)
			atlas, err := enc.Atlas()
			s.Require().NoError(err)
			for _, c := range atlas.Cells {
				want, wantOK := enc.PartitionAt(c)
				got, gotOK := back.PartitionAt(c)
				s.Equal(wantOK, gotOK, "cell %v", c)
				s.Equal(want, got, "cell %v", c)
			}
			raw, err := json.Marshal(enc.ToData().Field)
			s.Require().NoError(err)
			s.False(strings.Contains(strings.ToLower(string(raw)), "partition"), "FieldData carries no partition")
		})
	}
}
