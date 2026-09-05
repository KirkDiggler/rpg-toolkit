// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// holdout_reserve_test.go is the hold-out, STEP B, at the SEAM (rpg-project#375,
// the hold-out design §3.7, §5, R6, A4, A5, A6): arrivals, on the SHIPPED
// raider camp — the letter that arrives at round 6, the three zombies that
// come when the chief falls — through the verbs a host uses and the streams a
// client reads.
//
//   - a monster spawned with a predicate goes into reserve: the response says
//     so, no beat is written, and it is on no roster and no map for anyone;
//   - a verb after that reloads the stored world and it is still waiting;
//   - the chief falls: on the next verb three `arrived` beats reach every
//     recipient in dense numbering, the roster lists the reinforcements on
//     their side, and they are in the fight;
//   - the letter is nowhere until round 6 — Hold refuses it as a thing that is
//     not here — and lies at the gate for everyone once round 6 starts;
//   - a predicate nothing could fire is refused by name.

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

var campReinforcements = []string{"reinforcement-1", "reinforcement-2", "reinforcement-3"}

// downTheChief puts the chief on the floor the way the session knows a body:
// by his stored sheet.
func (s *HoldOutSessionSuite) downTheChief() {
	s.T().Helper()
	stored := s.sessions.byID[campSession]
	for i := range stored.NPCs {
		if stored.NPCs[i].ID == campChief {
			stored.NPCs[i].HitPoints = 0
			return
		}
	}
	s.Require().Fail("the chief has no sheet to put on the floor")
}

// arrivalsOn is every EventArrived body on a recipient's stream, in order.
func (s *HoldOutSessionSuite) arrivalsOn(recipient string) []session.ArrivedBody {
	s.T().Helper()
	var out []session.ArrivedBody
	for _, e := range s.events(recipient) {
		if e.Kind != session.EventArrived {
			continue
		}
		body, ok := e.Body.(session.ArrivedBody)
		s.Require().True(ok, "an arrival crosses as its typed body: %+v", e)
		out = append(out, body)
	}
	return out
}

// assertDense fails unless a recipient's delivered numbers are consecutive.
func (s *HoldOutSessionSuite) assertDense(who string) {
	s.T().Helper()
	events := s.events(who)
	for i := 1; i < len(events); i++ {
		s.Require().Equal(events[i-1].Seq+1, events[i].Seq,
			"%s's own stream must be dense: seq %d follows %d", who, events[i].Seq, events[i-1].Seq)
	}
}

// TestASpawnWithAPredicateWaitsInReserveForEveryone is the reserve at the
// seam: the shipped camp's reinforcements arrive through Spawn with their
// predicate hand-carried, and go nowhere — the response says Reserved, nothing
// is narrated, and no roster, map or read shows them to anybody. A verb after
// that reloads the stored world, and they are still waiting.
func (s *HoldOutSessionSuite) TestASpawnWithAPredicateWaitsInReserveForEveryone() {
	s.startWith(campOptions{shipped: true, spawn: []string{campChief, campScout}})

	s.Run("the spawn says the monster went into reserve", func() {
		for _, id := range campReinforcements {
			m := s.placement(id)
			out := s.spawn(m)
			s.True(out.Reserved, "%s waits", id)
			s.Equal(id, out.Member.ID)
			s.Equal(absolute(m.At), out.Member.Position, "the cell it will arrive at, not one it stands on")
			s.Zero(out.Seq, "no beat was written")
			s.Nil(out.Formed)
			s.Empty(out.Discovered)
			s.Require().NotNil(out.NPC, "its sheet is recorded now; the run holds the member back")
			s.Zero(out.Delivery.Events)
		}
		s.Empty(s.stream.published, "the reserve is silent")
	})

	s.Run("on no roster, no map, and answerable by no read, for anyone", func() {
		rows := s.roster()
		s.Len(rows, 4, "alice, bob, the chief, the scout")
		for _, id := range campReinforcements {
			s.NotContains(rows, id)
			_, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: campSession, Member: id})
			s.ErrorIs(err, session.ErrNoMember, "%s is nowhere", id)
		}
		for _, who := range []string{"alice", "bob"} {
			s.NotContains(propIDs(s.atlas(who)), campLetter, "%s: the letter waits for round 6", who)
		}
	})

	s.Run("the stored world holds the reserve and the next verb keeps it", func() {
		stored := s.encounters.byID[campWorldID]
		s.Require().Len(stored.Reserve, 3)
		for i, r := range stored.Reserve {
			s.Equal(encounter.MemberID(campReinforcements[i]), r.ID)
			s.Equal(campFaction, r.Faction)
		}
		// A step at the gate: a verb that loads the blob back and refreshes
		// sight with the reserve seeded into the seams.
		s.walk("bob", s.freeNeighbour("bob"))
		s.Len(s.roster(), 4, "still waiting")
		s.Len(s.encounters.byID[campWorldID].Reserve, 3)
	})
}

// TestTheChiefsFallBringsTheReinforcementsToEveryone is A4 at the seam: alice
// fights the scout in the yard; the chief's sheet says zero; bob takes a step
// at the gate — and on that verb three zombies stand where the file drew them,
// on the raiders' side, in the fight, narrated to everyone in dense numbering
// after the fall that caused them.
func (s *HoldOutSessionSuite) TestTheChiefsFallBringsTheReinforcementsToEveryone() {
	s.startWith(campOptions{shipped: true})
	s.Require().Len(s.roster(), 4, "three in reserve")
	s.Require().NotNil(s.intoTheYard("alice").Formed, "precondition: alice and the scout are fighting")
	s.Require().Equal(session.ClockWorld, s.turn("bob").Clock, "precondition: bob is at the gate, out of it")
	s.downTheChief()
	s.stream.published = nil

	// The next verb: bob steps one cell at the gate. Its participation pass
	// notices the chief down, and the reinforcements arrive.
	s.walk("bob", s.freeNeighbour("bob"))

	s.Run("three zombies stand at the gate, on the raiders' side", func() {
		rows := s.roster()
		for _, id := range campReinforcements {
			s.Require().Contains(rows, id)
			s.Equal(session.KindMonster, rows[id].Kind)
			s.Equal(campFaction, rows[id].Faction)
		}
		s.Equal(absolute(s.placement(campReinforcements[0]).At), s.where(campReinforcements[0]), "where the author drew it")
	})

	s.Run("the arrivals are narrated to everyone, after the fall, in dense numbering", func() {
		// Everyone who was there hears all three; an arrival hears its own
		// and the ones after it — a member is told nothing from before it
		// existed, and its numbering starts at its first beat.
		expected := map[string]int{campReinforcements[0]: 3, campReinforcements[1]: 2, campReinforcements[2]: 1}
		for _, who := range s.recipients() {
			arrivals := s.arrivalsOn(who)
			want, isArrival := expected[who]
			if !isArrival {
				want = 3
			}
			s.Require().Len(arrivals, want, "%s heard %v", who, s.kinds(who))
			for i, a := range arrivals {
				id := campReinforcements[3-want+i]
				s.Equal(id, a.ID, "in id order")
				s.Equal(session.PlacementMonster, a.Kind)
				s.Equal(absolute(s.placement(id).At), a.Cell)
			}
			s.assertDense(who)
		}
		kinds := s.kinds("bob")
		downAt, arrivedAt := -1, -1
		for i, k := range kinds {
			switch {
			case k == session.EventDowned && downAt < 0:
				downAt = i
			case k == session.EventArrived && arrivedAt < 0:
				arrivedAt = i
			}
		}
		s.Require().NotEqual(-1, downAt, "the chief's fall is narrated: %v", kinds)
		s.Less(downAt, arrivedAt, "the fall is the cause")
		s.Contains(s.recipients(), campReinforcements[0], "the arrival hears itself arrive")
	})

	s.Run("the fight is joined: the zombies and bob are in it", func() {
		for _, id := range append([]string{"bob", "alice"}, campReinforcements...) {
			s.Equal(session.ClockTurn, s.turn(id).Clock, id)
		}
		s.NotContains(s.kinds("alice"), session.EventFightEnded, "one fight, grown")

		// PINNED, NOT ARGUED: joining a running fight is narrated by the
		// composition's `transferred` beat, which this seam has never named
		// — it reaches a client as EventUnknown, delivered but uninterpretable
		// (the delivery rule), so a client learns who joined only from the
		// next TURN_ENDED's order. A pre-existing gap the reinforcements
		// make visible; naming it is a wire decision (protos has no such
		// kind), recorded on the branch report rather than widened here.
		for _, e := range s.events("bob") {
			if e.Kind == session.EventUnknown {
				s.Contains(string(e.Payload), `"beat":"transferred"`, "the only unnamed beat is the transfer")
			}
		}
	})

	s.Run("nothing is waiting any more, and a reload agrees", func() {
		s.Nil(s.encounters.byID[campWorldID].Reserve)
		for _, id := range campReinforcements {
			s.Contains(s.roster(), id)
		}
	})
}

// TestTheLetterArrivesAtRoundSixThroughTheSeam is A5 at the seam, and R9:
// the letter is nowhere — refused by Hold as a thing that is not here, in no
// atlas — through five rounds of a fight, and lies at the gate for everyone the
// moment round 6 starts, with an EventArrived naming the prop and its cell.
func (s *HoldOutSessionSuite) TestTheLetterArrivesAtRoundSixThroughTheSeam() {
	s.startWith(campOptions{shipped: true})
	letterAt := absolute(spatial.Position{X: 1, Y: 3})

	absent := func(when string) {
		s.T().Helper()
		_, err := s.mgr.Hold(context.Background(), &session.HoldInput{
			Session: campSession, Member: "bob", Target: campLetter, Range: 9})
		s.Require().ErrorIs(err, session.ErrNoProp, "%s: the letter refuses as a thing that is not here", when)
		s.NotContains(propIDs(s.atlas("bob")), campLetter, "%s: bob's map shows the letter", when)
		s.Empty(s.arrivalsOn("bob"), "%s", when)
	}
	absent("at first light")

	s.Require().NotNil(s.intoTheYard("alice").Formed, "the fight is on")
	for round := 2; round <= 5; round++ {
		s.Require().NoError(s.endTurnOf("alice"), "round %d", round)
		absent(fmt.Sprintf("in round %d", round))
	}

	s.Require().NoError(s.endTurnOf("alice"), "round 6 starts")
	s.Run("round six: the letter lies at the gate for everyone", func() {
		s.Contains(propIDs(s.atlas("bob")), campLetter)
		s.Contains(propIDs(s.atlas("alice")), campLetter)
		for _, who := range []string{"alice", "bob"} {
			arrivals := s.arrivalsOn(who)
			s.Require().Len(arrivals, 1, "%s heard %v", who, s.kinds(who))
			s.Equal(session.ArrivedBody{ID: campLetter, Kind: session.PlacementProp, Cell: letterAt}, arrivals[0])
			s.assertDense(who)
		}
		_, err := s.mgr.Hold(context.Background(), &session.HoldInput{
			Session: campSession, Member: "bob", Target: campLetter, Range: 9})
		s.Require().NoError(err, "and it can be picked up")
		s.Equal(session.HeldBody{Holder: "bob", Prop: campLetter}, s.bodyOf("alice", session.EventHeld))
	})
}

// TestASpawnCannotWaitOnAPredicateNothingCouldFire is the fail-closed half:
// a monster waiting for its own fall would wait forever, and the composition
// refuses it by name before anything is reserved — crossing as this package's
// own sentinel, with nothing left behind.
func (s *HoldOutSessionSuite) TestASpawnCannotWaitOnAPredicateNothingCouldFire() {
	s.startWith(campOptions{shipped: true})
	at := s.compiled.PartyStart[0].At

	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: campSession, ID: "stray", Ref: refs.Monsters.Zombie().String(),
		Position: absolute(spatial.Position{X: at.X, Y: at.Y + 2}), Faction: campFaction,
		Arrives: session.ArrivesOnFall{Member: "stray"},
	})
	s.Require().ErrorIs(err, session.ErrNoMember)

	for _, npc := range s.sessions.byID[campSession].NPCs {
		s.NotEqual("stray", npc.ID, "the refusal left nothing behind")
	}
	s.Len(s.encounters.byID[campWorldID].Reserve, 3, "the shipped reserve alone")
	s.Empty(s.stream.published, "and told nobody")
}
