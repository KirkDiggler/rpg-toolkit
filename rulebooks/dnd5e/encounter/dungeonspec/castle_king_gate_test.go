// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_gate_test.go is THE GATE, FOUR WAYS (rpg-project#605, use case
// §3): spear a guard, talk your way in as the feast's troupe, fail to, and
// talk to the wrong guard. Each route is the compiled castle driven by the
// verbs a player has; what it asserts is the use case's "asserted outcome"
// for that row, and nothing about how the rest of the castle spent its time.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestMadKingEveryRoomHasOneWayIn is the castle's stone. The void is
// transparent (the single-room dialect accepts nothing else), so walls, not
// the void between the rooms, close each room. Opaque void alone would not:
// it stops only the direct ray, and a sight lane can start on a void cell past
// a wall's end (rpg-toolkit#2068).
//
// The gatehouse: with the gate shut the hero sees only the gatehouse, from the
// party start and from the gate, and the open gate shows the courtyard.
//
// The kitchen, the chapel, the great hall and the throne room: with that
// room's door shut and every other door open, the hero's route from the party
// start toward any of the room's floor never enters the room, and no cell of
// it is in sight from anywhere in the yard. With the door open the route walks
// in, so the shut door is what refuses.
func TestMadKingEveryRoomHasOneWayIn(t *testing.T) {
	t.Parallel()

	t.Run("the gatehouse", func(t *testing.T) {
		t.Parallel()
		s := openTheCastle(t, partyStart(t))
		gatehouse, ok := s.enc.RegionAt(castleCell(0, 15))
		require.True(t, ok, "the party start is the gatehouse's floor")
		courtyard, ok := s.enc.RegionAt(castleCell(0, 4))
		require.True(t, ok, "the courtyard's floor")
		onlyTheGatehouse := func(where string) {
			for region := range s.seenRegions() {
				require.Equal(t, gatehouse, region, "%s, with the gate shut, the hero sees only the gatehouse", where)
			}
		}

		onlyTheGatehouse("from the party start")
		s.walkTo(castleCell(0, 10))
		onlyTheGatehouse("from the gate")
		s.open(kingGateDoor)
		require.NotZero(t, s.seenRegions()[courtyard], "the open gate shows the courtyard")
	})

	doors := []encounter.DoorID{kingGateDoor, kingHallDoor, kingKitchenDoor, kingChapelDoor, kingThroneDoor}
	onlyShut := func(shut encounter.DoorID) map[encounter.DoorID]encounter.DoorState {
		states := map[encounter.DoorID]encounter.DoorState{}
		for _, d := range doors {
			states[d] = encounter.DoorIsOpen()
		}
		if shut != "" {
			states[shut] = encounter.DoorIsClosed()
		}
		return states
	}
	// The yard: the courtyard, the barracks and the armoury, along the hall
	// wall and its partitions, at their far corners and in the middle.
	yard := []spatial.Position{
		castleCell(0, -6), castleCell(-14, -6), castleCell(14, -6), castleCell(-15, -6), castleCell(15, -6),
		castleCell(-26, -6), castleCell(26, -6), castleCell(-20, -5), castleCell(20, -5),
		castleCell(-20, 2), castleCell(20, 2), castleCell(0, 4),
	}

	for _, room := range []struct {
		name string
		door encounter.DoorID
		in   spatial.Position
	}{
		{"the kitchen", kingKitchenDoor, castleCell(-20, -12)},
		{"the chapel", kingChapelDoor, castleCell(20, -12)},
		{"the great hall", kingHallDoor, castleCell(0, -12)},
		{"the throne room", kingThroneDoor, castleCell(0, -20)},
	} {
		t.Run(room.name, func(t *testing.T) {
			t.Parallel()
			s := openTheCastleWithDoors(t, partyStart(t), onlyShut(room.door))
			region, ok := s.enc.RegionAt(room.in)
			require.True(t, ok, "%v is %s's floor", room.in, room.name)
			floor := s.regionCells(region)
			require.NotEmpty(t, floor)

			// The route stops beside its anchor, so what it proves is where it
			// goes: with the door shut, toward no cell of the room does it set
			// foot in the room.
			for _, cell := range floor {
				for _, step := range s.routeFor(cell) {
					in, _ := s.enc.RegionAt(step)
					require.NotEqual(t, region, in,
						"with its door shut, the hero walks toward %v and never into %s", cell, room.name)
				}
			}

			for _, from := range yard {
				v := openTheCastleWithDoors(t, seatAt(from), onlyShut(room.door))
				require.Zero(t, v.seenRegions()[region],
					"with its door shut, %s is out of sight from the yard at %v", room.name, from)
			}

			open := openTheCastleWithDoors(t, partyStart(t), onlyShut(""))
			path := open.routeFor(room.in)
			require.NotEmpty(t, path)
			last, _ := open.enc.RegionAt(path[len(path)-1])
			require.Equal(t, region, last, "control: with every door open, the hero walks into %s", room.name)
		})
	}
}

// unseenByTheKing is that the hero and the king see neither the other's
// cell, where the hero stands now.
func (s *kingScene) unseenByTheKing(where string) {
	s.t.Helper()
	sees := func(who encounter.MemberID, cell spatial.Position) bool {
		out, err := s.enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: who})
		require.NoError(s.t, err)
		for _, c := range out.Cells {
			if c == cell {
				return true
			}
		}
		return false
	}
	require.False(s.t, sees(hero, s.at(theKing)), "%s, the hero does not see the king", where)
	require.False(s.t, sees(theKing, s.at(hero)), "%s, the king does not see the hero", where)
}

// seatAt is the authored offset cell a member is placed at for a
// dungeon-absolute cell: [castleCell] read backwards.
func seatAt(cell spatial.Position) spatial.Position {
	col, row := encounter.OffsetOf(encounter.HexesArePointyTop(), cell)

	return spatial.Position{X: float64(col), Y: float64(row)}
}

// seenRegions is how many cells of each region the hero sees now.
func (s *kingScene) seenRegions() map[encounter.RegionID]int {
	s.t.Helper()
	out, err := s.enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: hero})
	require.NoError(s.t, err)
	by := map[encounter.RegionID]int{}
	for _, cell := range out.Cells {
		region, ok := s.enc.RegionAt(cell)
		require.True(s.t, ok, "%v is floor", cell)
		by[region]++
	}

	return by
}

// regionCells is every floor cell the file compiles into a region, as
// dungeon-absolute cells. The compiled field authors its cells as offset
// pairs, the frame [encounter.MemberInput.Position] takes; every answer this
// file reads back ([encounter.Encounter.RegionAt], a route, a sight read) is
// absolute.
func (s *kingScene) regionCells(id encounter.RegionID) []spatial.Position {
	s.t.Helper()
	for _, r := range s.compiled.Field.Regions {
		if r.ID == id {
			out := make([]spatial.Position, 0, len(r.Cells))
			for _, c := range r.Cells {
				cell := castleCell(int(c.X), int(c.Y))
				in, ok := s.enc.RegionAt(cell)
				require.True(s.t, ok && in == id, "%v is %s's floor", cell, id)
				out = append(out, cell)
			}
			return out
		}
	}
	require.FailNowf(s.t, "no such region", "%s", id)

	return nil
}

// TestMadKingFrontDoor is the front door: the hero comes in at the party
// start and spears the nearest gate guard. Nobody authored a rule for it —
// the aggression law turns the garrison, the fight forms at the gate, the
// gate patrol's own row shouts the alarm, and the barracks empty.
func TestMadKingFrontDoor(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, partyStart(t))
	require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty),
		"precondition: the gate is civil to the party")
	require.NotContains(t, memberIDsOf(t, s.enc), kingReserve, "precondition: the reserves are in the barracks, unseen")

	t.Run("the gatehouse's one way into the courtyard is its shut door", func(t *testing.T) {
		yard := castleCell(0, 4)
		path := s.routeFor(yard)
		if len(path) > 0 {
			require.Greater(t, s.enc.Distance(path[len(path)-1], yard), 1.0,
				"with the gate shut the pathfinder gets the hero no nearer the yard than the gatehouse")
		}
	})

	s.struck(kingGate1, false)

	t.Run("the aggression law turns the garrison on the party", func(t *testing.T) {
		require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty))
	})

	t.Run("the fight forms at the gate", func(t *testing.T) {
		for _, id := range []encounter.MemberID{hero, kingGate1, kingGate2} {
			require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, id), "%s fights at the gate", id)
		}
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, theKing), "the king is no part of the gate's fight")
	})

	t.Run("a gate guard who sees the party raises the alarm, and the gate hears it", func(t *testing.T) {
		shouters := s.answeredWith("alarm-raised", hero)
		require.NotEmpty(t, shouters, "the hero saw somebody raise the alarm")
		for _, who := range shouters {
			require.Contains(t, []encounter.MemberID{kingGate1, kingGate2}, who,
				"the alarm is the gate patrol's row: only a gate guard says it")
		}
		require.True(t, s.learners("alarm-raised")[hero], "and heard it")
	})

	t.Run("the reserves arrive on the alarm", func(t *testing.T) {
		require.Contains(t, memberIDsOf(t, s.enc), kingReserve)
	})

	// NOT ASSERTED, AND WHY: the use case's "loyalists in the keep walk to
	// the throne room on the fact". The keep-patrol row is in the file, but no
	// knight ever holds `alarm-raised`: a wall-attached door links no regions
	// for hearing, so the shout at the gate never crosses into the hall
	// (rpg-toolkit#2066). That issue's fix brings this assertion.
}

// TestMadKingTroupe is the troupe: the cook's gossip teaches the hero that
// there is a feast tonight, the hero walks to the gate and passes as the
// feast's players, and walks on into the great hall — with the garrison
// civil the whole way and nobody having swung at anybody.
func TestMadKingTroupe(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheCook)

	s.said(kingCook, "gossip", "persuasion", true)
	require.True(t, s.learners("feast-tonight")[hero], "the cook told the hero")

	s.outOfTheKitchen()
	s.throughTheGate(kingCaptain)
	s.said(kingCaptain, "troupe", "performance", true)
	require.Empty(t, s.learners("party-recognized"), "a troupe that passes writes nothing")

	gatehouse, ok := s.enc.RegionAt(castleCell(0, 15))
	require.True(t, ok, "the party start is the gatehouse's floor")
	// THE KING SITS OUT OF THE DOOR'S LINE. The throne door stands open, and
	// a king in its line from the hall would see the party the moment the
	// hall door opened. From the hall door and from the hall's middle, the
	// hero and the king do not see each other.
	s.walkTo(castleCell(0, -6))
	t.Run("the party crosses the hall unseen by the king", func(t *testing.T) {
		s.unseenByTheKing("at the hall door")
		middle := castleCell(0, -12)
		s.walkTo(middle)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "the hero walks into the hall and into no fight")
		hall, _ := s.enc.RegionAt(middle)
		here, _ := s.enc.RegionAt(s.at(hero))
		require.Equal(t, hall, here, "precondition: the hero is in the hall")
		require.LessOrEqual(t, s.enc.Distance(s.at(hero), middle), 1.0, "precondition: beside its middle")
		s.unseenByTheKing("in the middle of the hall")
	})
	s.walkTo(castleCell(-6, -10))

	// THE GATE'S HALF, AND ONLY IT. The king and his knights are authored
	// out of the open throne door's line from the hall door and the hall's
	// middle, and the king is pinned above. Where a pacing knight stands at
	// any other moment is still the patrol's phase, set by how much world
	// time the route has spent, not by anything the troupe said. A knight's
	// sighting is a loyalist fight, which the disguise never covered (§1), so
	// this subtest asserts the garrison and says nothing either way about the
	// keep.
	t.Run("the party came through the gate and into the hall", func(t *testing.T) {
		here, ok := s.enc.RegionAt(s.at(hero))
		require.True(t, ok)
		require.NotEqual(t, gatehouse, here, "the hero is past the gate")
	})

	t.Run("the garrison is civil, and no garrison member fought", func(t *testing.T) {
		require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty))
		for _, id := range s.garrisonMembers() {
			require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, id), "%s is in no fight", id)
		}
		for _, st := range *s.strikes {
			require.NotEqual(t, garrison, s.factionOf(st.attacker), "no garrison member swung")
			require.NotEqual(t, garrison, s.factionOf(st.target), "nobody swung at the garrison")
		}
		for _, f := range s.enc.ToData().Journal.Facts {
			if strings.HasPrefix(f.Kind, "settled:hostile") {
				require.NotContains(t, f.Kind, string(garrison), "no blow turned the garrison")
			}
		}
	})
}

// TestMadKingTroupeBlown is the troupe failing its check: the captain's
// failure line writes `party-recognized` on the gate's witnesses, the
// garrison turns because its mind witnessed it, and the fight forms in the
// line's own call — the guards were in sight all along, and what changed was
// the stance.
func TestMadKingTroupeBlown(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheCook)
	s.said(kingCook, "gossip", "persuasion", true)
	s.outOfTheKitchen()
	s.throughTheGate(kingCaptain)
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, kingCaptain),
		"precondition: the hero stands in front of the captain, and nobody is fighting")

	s.said(kingCaptain, "troupe", "performance", false)

	t.Run("the gate witnessed the party being recognized", func(t *testing.T) {
		learned := s.learners("party-recognized")
		require.True(t, learned[kingCaptain], "the captain, the garrison's mind, saw through it")
		require.True(t, learned[hero])
		for _, id := range []encounter.MemberID{theKing, kingKeep1, kingKeep2} {
			require.False(t, learned[id], "%s, in the keep, did not witness it: the court is not fooled or warned", id)
		}
	})

	t.Run("the garrison turned on a fact its mind witnessed", func(t *testing.T) {
		require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty))
	})

	t.Run("and the fight formed in the same call, at the gate", func(t *testing.T) {
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero))
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, kingCaptain))
	})
}

// TestMadKingScout is the scout who knows the truth and tells it to the
// wrong guard: the hero holds `king-burned-village` from the cook, walks to
// the gate and says the captain's line to a gate guard. A gate guard carries
// no such line, so the line is refused by name; nothing turns, and nobody
// learns the guards have turned.
func TestMadKingScout(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheCook)

	t.Run("control: the captain withholds the line from a hero who has not heard about Ashford", func(t *testing.T) {
		options, err := s.enc.OptionsFor(hero, kingCaptain)
		require.NoError(t, err)
		for _, o := range options {
			require.NotEqual(t, "turn", o.ID, "the line waits on king-burned-village")
		}
		_, err = s.speak(kingCaptain, "turn", "persuasion", true)
		require.ErrorIs(t, err, encounter.ErrNoOption)
	})

	s.said(kingCook, "gossip", "persuasion", true)
	s.said(kingCook, "ashford", "persuasion", true)
	require.True(t, s.learners("king-burned-village")[hero], "precondition: the hero knows")

	s.outOfTheKitchen()
	s.throughTheGate(kingGate1)

	t.Run("control: the captain would hear the line from this hero", func(t *testing.T) {
		options, err := s.enc.OptionsFor(hero, kingCaptain)
		require.NoError(t, err)
		var ids []string
		for _, o := range options {
			ids = append(ids, o.ID)
		}
		require.Contains(t, ids, "turn")
	})

	t.Run("the gate guard carries no such line", func(t *testing.T) {
		_, err := s.speak(kingGate1, "turn", "persuasion", true)
		require.ErrorIs(t, err, encounter.ErrNoOption)
	})

	t.Run("nothing turned", func(t *testing.T) {
		require.Empty(t, s.learners("guards-turned"))
		require.Equal(t, encounter.StanceNeutral, s.stance(garrison, loyalists))
	})
}
