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
)

// TestMadKingTheGateIsTheOnlyView is the castle's stone: with the gate shut,
// the hero sees nothing outside the gatehouse, from the party start or from
// the gate itself, and opening the gate shows the courtyard. The play block's
// void is transparent (the single-room dialect accepts nothing else), so the
// outer walls are what close the view, not the void between the rooms.
// Opaque void alone would not do it: it stops only the direct ray, and a
// sight lane can start on a void cell past a wall's end (rpg-toolkit#2068).
func TestMadKingTheGateIsTheOnlyView(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, partyStart(t))
	gatehouse, ok := s.enc.RegionAt(castleCell(0, 15))
	require.True(t, ok, "the party start is the gatehouse's floor")
	courtyard, ok := s.enc.RegionAt(castleCell(0, 4))
	require.True(t, ok, "the courtyard's floor")
	seen := func() map[encounter.RegionID]int {
		out, err := s.enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: hero})
		require.NoError(t, err)
		by := map[encounter.RegionID]int{}
		for _, cell := range out.Cells {
			region, ok := s.enc.RegionAt(cell)
			require.True(t, ok, "%v is floor", cell)
			by[region]++
		}
		return by
	}
	onlyTheGatehouse := func(where string) {
		for region := range seen() {
			require.Equal(t, gatehouse, region, "%s, with the gate shut, the hero sees only the gatehouse", where)
		}
	}

	onlyTheGatehouse("from the party start")
	s.walkTo(castleCell(0, 10))
	onlyTheGatehouse("from the gate")
	s.open(kingGateDoor)
	require.NotZero(t, seen()[courtyard], "the open gate shows the courtyard")
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

	s.throughTheGate(kingCaptain)
	s.said(kingCaptain, "troupe", "performance", true)
	require.Empty(t, s.learners("party-recognized"), "a troupe that passes writes nothing")

	gatehouse, ok := s.enc.RegionAt(castleCell(0, 15))
	require.True(t, ok, "the party start is the gatehouse's floor")
	s.walkTo(castleCell(0, -6))
	s.open(kingHallDoor)
	s.walkTo(castleCell(-6, -10))

	// THE GATE'S HALF, AND ONLY IT. From the hall door the hero is in line
	// with the throne room's open door, and whether a pacing knight stands in
	// that line then is the patrol's phase: set by how much world time the
	// route has spent, not by anything the troupe said. A knight's sighting
	// there is a loyalist fight, which the disguise never covered (§1), so
	// this route asserts the garrison and says nothing either way about the
	// keep.
	t.Run("the party came through the gate and opened the hall", func(t *testing.T) {
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
