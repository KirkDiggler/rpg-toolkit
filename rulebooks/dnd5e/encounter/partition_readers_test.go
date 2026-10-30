// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// partition_readers_test.go pins the two outcome readers that decide on the
// wall partition and could silently drift back to the authored region
// (rpg-project#725, R4): presence transfer ([Encounter.sweepPresence]) and the
// search a blocked arrival makes ([Encounter.arrivalCell]). Each scene puts
// the deciding cells in two AUTHORED regions of ONE open partition, so the
// test passes only when the partition is what is read.

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// openYardAndHut is yardAndHut with nothing built between the yard and the
// hut, and a one-cell nook painted east of the hut, open to it: three
// authored regions, one room. The chief stands in the nook.
//
//	x: 0..5 yard | 6..11 hut | 12 nook (row 0)   alice [4,1], scroll [3,1], chief [12,0]
func (s *HoldOutSuite) openYardAndHut() (encounter.FieldInput, []encounter.MemberInput) {
	field, members := s.yardAndHut()
	field.Walls, field.Doors = nil, nil
	field.Regions = append(field.Regions, rectRegion("nook", 12, 0, 1, 1))
	for i := range members {
		if members[i].ID == campChief {
			members[i].Position = spatial.Position{X: 12, Y: 0}
		}
	}
	return field, members
}

// TestPresenceIsTheRoomTheWallsMake: alice picks the letter up in the yard
// and the chief, in the nook, learns it at once — the yard and the nook are
// two authored regions of one open room, and presence is the room.
func (s *HoldOutSuite) TestPresenceIsTheRoomTheWallsMake() {
	field, members := s.openYardAndHut()
	enc := s.open(field, members, withdrawn())
	aliceAt, chiefAt := cellAt(4, 1), cellAt(12, 0)
	aliceRegion, _ := enc.RegionAt(aliceAt)
	chiefRegion, _ := enc.RegionAt(chiefAt)
	s.Require().Equal("yard", aliceRegion)
	s.Require().Equal("nook", chiefRegion, "precondition: two authored regions")
	aliceRoom, _ := enc.PartitionAt(aliceAt)
	chiefRoom, _ := enc.PartitionAt(chiefAt)
	s.Require().Equal(aliceRoom, chiefRoom, "precondition: one room, nothing built between")
	s.Require().Equal(encounter.StanceHostile, s.stance(enc), "precondition: the camp is hostile")

	s.hold(enc, "alice", "yard-scroll")

	s.Equal(encounter.StanceNeutral, s.stance(enc), "the chief is in the room the letter is carried in, and learns it")
}

// TestABlockedArrivalLooksForRoomInTheRoomTheWallsMake: a prize drawn on the
// nook's only cell arrives while the chief stands on it. The nook, as a
// region, has no free cell; the room it is part of has plenty, and the prize
// lands beside him in the hut.
func (s *HoldOutSuite) TestABlockedArrivalLooksForRoomInTheRoomTheWallsMake() {
	field, members := s.openYardAndHut()
	prize := holdableProp("prize", "dnd5e:props:chest", spatial.Position{X: 12, Y: 0})
	prize.Arrives = encounter.TriggerFact{Fact: campFact}
	field.Props = append(append([]encounter.PropInput(nil), field.Props...), prize)
	enc := s.open(field, members, withdrawn())
	s.Require().NotContains(s.propIDs(enc), encounter.PropID("prize"))

	s.hold(enc, "alice", "yard-scroll")

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	var landed *spatial.Position
	for _, p := range atlas.Props {
		if p.ID == "prize" {
			at := p.At
			landed = &at
		}
	}
	s.Require().NotNil(landed, "the prize arrived")
	s.NotEqual(cellAt(12, 0), *landed, "not under the chief")
	s.Equal(1.0, enc.Distance(cellAt(12, 0), *landed), "the nearest free cell of the room")
	region, _ := enc.RegionAt(*landed)
	s.Equal("hut", region, "which is the hut's: the nook had none")
}
