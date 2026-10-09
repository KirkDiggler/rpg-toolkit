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
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
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

// launchShippedCamp is startWith for the scenes that read the launch's own
// answer or refusal: a fresh manager around the SHIPPED camp with only the
// named placements on its board (every one when keep is nil), plus any extra
// placements, launched by the host's one verb. The stream is left as the
// launch delivered it.
func (s *HoldOutSessionSuite) launchShippedCamp(
	keep []string, cast []*character.Data, extra ...dungeonspec.MonsterPlacement,
) (*session.LaunchOutput, error) {
	s.T().Helper()
	if cast == nil {
		cast = []*character.Data{sharpEyed("alice"), dullEyed("bob")}
	}
	camp := s.canonical
	camp.Monsters = nil
	for _, m := range s.canonical.Monsters {
		if keep == nil || slices.Contains(keep, m.ID) {
			camp.Monsters = append(camp.Monsters, m)
		}
	}
	camp.Monsters = append(camp.Monsters, extra...)
	s.camp = camp
	s.stream = &fakeStream{}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(cast...)

	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	party := make([]string, 0, len(cast))
	for _, sheet := range cast {
		party = append(party, sheet.ID)
	}
	return mgr.Launch(context.Background(), &session.LaunchInput{
		Session: campSession, DungeonKey: campKey, Dungeon: &camp, Party: party,
	})
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
// seam: the shipped camp's reinforcements are launched with the predicate the
// file gave them, and go nowhere — the launch places none of them, nothing
// narrates them, and no roster, map or read shows them to anybody. A verb
// after that reloads the stored world, and they are still waiting.
func (s *HoldOutSessionSuite) TestASpawnWithAPredicateWaitsInReserveForEveryone() {
	out, err := s.launchShippedCamp(nil, nil)
	s.Require().NoError(err)

	s.Run("the launch holds the reinforcements back", func() {
		placed := map[string]bool{}
		for _, member := range out.Members {
			placed[member.ID] = true
		}
		sheets := map[string]bool{}
		for _, npc := range s.sessions.byID[campSession].NPCs {
			sheets[npc.ID] = true
		}
		for _, id := range campReinforcements {
			s.Require().NotNil(s.placement(id).Arrives, "%s: the file gave it a predicate", id)
			s.False(placed[id], "%s waits: it is not on the board", id)
			s.NotContains(out.Discovered, id, "%s saw nothing", id)
			for _, formed := range out.Formed {
				s.NotContains(formed.Order, id, "%s is in no fight", id)
			}
			s.True(sheets[id], "%s: its sheet is recorded now; the run holds the member back", id)
		}
		for _, who := range []string{"alice", "bob"} {
			s.Empty(s.arrivalsOn(who), "the reserve is silent to %s", who)
		}
		// Silent in every kind, not only arrivals: no event of any kind is
		// addressed to a reinforcement or carries one's id (joined, sighted,
		// arrived, or anything else).
		for _, event := range s.stream.published {
			for _, id := range campReinforcements {
				s.NotEqual(id, event.Recipient, "nothing is delivered to %s", id)
				s.NotContains(string(event.Payload), id, "no %s event names %s", event.Kind, id)
			}
		}
	})
	s.stream.published = nil

	s.Run("on no roster, no map, and answerable by no read, for anyone", func() {
		rows := s.roster()
		s.Len(rows, 4, "alice, bob, the chief, the scout")
		for _, id := range campReinforcements {
			s.NotContains(rows, id)
			_, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: campSession, Member: id})
			s.ErrorIs(err, session.ErrNoMember, "%s is nowhere", id)
		}
		for _, who := range []string{"alice", "bob"} {
			s.NotContains(knownCellProps(s.T(), s.mgr, campSession, who), campLetter, "%s: the letter waits for round 6", who)
		}
	})

	s.Run("the stored world holds the reserve and the next verb keeps it", func() {
		stored := s.encounters.byID[campSession]
		s.Require().Len(stored.Reserve, 3)
		for i, r := range stored.Reserve {
			s.Equal(encounter.MemberID(campReinforcements[i]), r.ID)
			s.Equal(campFaction, r.Faction)
			s.Equal(absolute(s.placement(campReinforcements[i]).At),
				spatial.Position{X: r.Cell.X, Y: r.Cell.Y}, "the cell it will arrive at, not one it stands on")
		}
		// A step at the gate: a verb that loads the blob back and refreshes
		// sight with the reserve seeded into the seams.
		s.walk("bob", s.freeNeighbour("bob"))
		s.Len(s.roster(), 4, "still waiting")
		s.Len(s.encounters.byID[campSession].Reserve, 3)
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
		s.Nil(s.encounters.byID[campSession].Reserve)
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
		s.NotContains(knownCellProps(s.T(), s.mgr, campSession, "bob"), campLetter, "%s: bob's map shows the letter", when)
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
		s.Contains(knownCellProps(s.T(), s.mgr, campSession, "bob"), campLetter)
		s.Contains(knownCellProps(s.T(), s.mgr, campSession, "alice"), campLetter)
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
// a monster waiting for its own fall would wait forever, and the launch
// refuses it by name before anything is reserved — crossing as this package's
// own sentinel, with nothing left behind.
func (s *HoldOutSessionSuite) TestASpawnCannotWaitOnAPredicateNothingCouldFire() {
	at := s.canonical.PartyStart[0].At
	stray := dungeonspec.MonsterPlacement{
		Ref: refs.Monsters.Zombie().String(), ID: "stray", MemberID: "stray",
		At: spatial.Position{X: at.X, Y: at.Y + 2}, Faction: campFaction,
		Arrives: encounter.TriggerMemberDown{Member: "stray"},
	}

	_, err := s.launchShippedCamp(nil, nil, stray)
	s.Require().ErrorIs(err, session.ErrNoMember)

	s.NotContains(s.sessions.byID, campSession, "the refusal left no session behind")
	s.NotContains(s.encounters.byID, campSession, "and no world, so nothing was reserved")
	s.Empty(s.stream.published, "and told nobody")
}

// weakenTheChief leaves the chief one blow from the floor and easy to hit, on
// his stored sheet — the session's own truth about a monster's hit points.
func (s *HoldOutSessionSuite) weakenTheChief() {
	s.T().Helper()
	stored := s.sessions.byID[campSession]
	for i := range stored.NPCs {
		if stored.NPCs[i].ID == campChief {
			stored.NPCs[i].HitPoints, stored.NPCs[i].ArmorClass = 1, 5
			return
		}
	}
	s.Require().Fail("the chief has no sheet")
}

// TestTheBlowThatFellsTheChiefBringsTheReinforcements is Kirk's walk 4
// (2026-09-05): the reinforcements wait on the chief's fall, and the fall
// comes from a PLAYER'S OWN SWING — the arrivals happen inside the Attack
// verb's record, on the world the resolution handed back. The swing lands,
// the chief is down, three zombies stand at the gate, and the attacker's
// action was spent on a blow that was told.
func (s *HoldOutSessionSuite) TestTheBlowThatFellsTheChiefBringsTheReinforcements() {
	_, err := s.launchShippedCamp(append([]string{campChief}, campReinforcements...),
		[]*character.Data{stout("alice"), stout("bob")})
	s.Require().NoError(err)
	s.stream.published = nil
	s.Require().Len(s.encounters.byID[campSession].Reserve, 3, "the zombies wait on the chief")

	// alice walks into the hut and stands beside the chief's own cell, and
	// the fight forms with the two of them in reach.
	chief := s.placement(campChief)
	at := absolute(chief.At)
	s.walk("alice", spatial.Position{X: at.X - 1, Y: at.Y})
	s.Require().Equal(session.ClockTurn, s.turn("alice").Clock, "alice is in the chief's face")
	s.weakenTheChief()
	s.stream.published = nil
	s.Require().Equal("alice", s.turn("alice").Active)

	out, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: campSession, Attacker: "alice", Target: campChief,
		DeclarationID: currentAttackID(s.T(), s.mgr, campSession, "alice"),
	})
	s.Require().NoError(err, "the blow that fells the chief is told, not refused")
	s.Require().NotNil(out)

	s.Run("the chief is down and the zombies stand at the gate", func() {
		s.Contains(s.kinds("alice"), session.EventStruck)
		s.Contains(s.kinds("alice"), session.EventDowned)
		rows := s.roster()
		for _, id := range campReinforcements {
			s.Require().Contains(rows, id)
			s.Equal(campFaction, rows[id].Faction)
		}
		s.Nil(s.encounters.byID[campSession].Reserve)
	})

	s.Run("the arrivals are narrated to everyone who was there, after the fall", func() {
		for _, who := range []string{"alice", "bob"} {
			arrivals := s.arrivalsOn(who)
			s.Require().Len(arrivals, 3, "%s heard %v", who, s.kinds(who))
			s.assertDense(who)
		}
		kinds := s.kinds("alice")
		downAt, arrivedAt := -1, -1
		for i, k := range kinds {
			switch {
			case k == session.EventDowned && downAt < 0:
				downAt = i
			case k == session.EventArrived && arrivedAt < 0:
				arrivedAt = i
			}
		}
		s.Less(downAt, arrivedAt, "the fall is the cause")
	})

	s.Run("the writes were one commit, not a split", func() {
		// BOB'S SHEET IS IN HERE BECAUSE THE CHIEF PAID HIM (rpg-project#496).
		// He never swung and never moved: the fall settles the party's
		// experience inside the same commit as the blow, so his total is
		// written beside alice's rather than waiting for a verb of his own —
		// which is the claim this sub-test makes, now covering a second kind
		// of write. Alice appears ONCE despite being written twice in the
		// verb (the swing's damage, then her share), because the report names
		// aggregates and not touches.
		s.Equal([]string{
			"character:alice", "character:bob", "encounter:" + campSession, "session:" + campSession,
		}, out.Saved.Written)
		s.Empty(out.Saved.Failed)
	})
}
