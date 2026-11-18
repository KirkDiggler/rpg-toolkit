// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_gate_hint_test.go is rpg-project#936 at the session door: at the
// admitted gate, the cleric — a stranger to the gate guards — asks after her
// friends, and gate-1 SHOUTS the hint that one of them can speak for her.
// Alice, admitted, is shown it as her own ears or her party's carry it.

// theShoutedHint is gate-1's answer to `friends-inside`.
const theShoutedHint = "The troupe? Aye, they’re inside. If one of them comes back and speaks for you, in you go."

var (
	// inTheYardOutOfSight is courtyard floor out of gate-1's sight, east of
	// the gate's line, one room in from the gatehouse through the gate's
	// open doorway. (Much of the yard in line with the open gate sees him.)
	inTheYardOutOfSight = spatial.Position{X: 4, Y: 0}
	// besideGateOne is gatehouse floor in gate-1's sight and reach.
	besideGateOne = spatial.Position{X: -4, Y: 8}
)

// theHintAsked restores the admitted gate at the sharing stop given, stands
// alice where given (the hall's door shut when she is in the great hall)
// and the cleric beside gate-1, has the cleric ask after her friends, and
// returns what alice was sent of the hint.
func theHintAsked(t *testing.T, sharing encounter.Sharing, alice spatial.Position) []found[session.SpeechAct] {
	t.Helper()
	c := atTheAdmittedGate(t)
	world := c.encounters.byID["sess"]
	world.Sharing = sharing
	if alice == inTheGreatHall {
		shut := false
		for i := range world.Doors {
			if world.Doors[i].ID == "the-mad-king/hall-wall-door" {
				world.Doors[i].State, shut = string(encounter.DoorClosed), true
			}
		}
		require.True(t, shut, "precondition: the hall's door")
	}
	c.placeInTheRecord(t, "cleric", besideGateOne)
	// Alice is stood a cell short and takes the last step herself, so she
	// looks from where she stands; the walk there is not what this is about.
	c.placeInTheRecord(t, "alice", spatial.Position{X: alice.X - 1, Y: alice.Y})
	c.move(t, []spatial.Position{alice})
	played := c.play(t, "the cleric asks after her friends", func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
			Session: "sess", Member: "cleric", Target: "gate-1", OptionID: friendsLine,
		})
		return err
	})
	require.NotEmpty(t, where[session.SpeechAct](played.to("cleric"), func(s session.SpeechAct) bool {
		return s.Words != nil && *s.Words == theShoutedHint
	}), "precondition: the cleric, spoken to, is told the hint")
	return where[session.SpeechAct](played.to("alice"), func(s session.SpeechAct) bool {
		return s.Words != nil && *s.Words == theShoutedHint
	})
}

// TestCastleTheHintIsShoutedToTheYard: at each their own, alice in the yard
// out of the gate guards' sight is shown the hint's words, from someone she
// did not see, said to nobody she saw, one room off, shouted.
func TestCastleTheHintIsShoutedToTheYard(t *testing.T) {
	got := theHintAsked(t, encounter.SharingOwn, inTheYardOutOfSight)
	require.Len(t, got, 1)
	s := got[0].body
	require.Equal(t, session.Party{}, s.Speaker, "no speaker")
	require.Equal(t, session.Party{}, s.SpokenTo, "no one spoken to")
	require.NotNil(t, s.Hops)
	require.Equal(t, 1, *s.Hops, "one room off")
	require.Equal(t, "shout", s.Loudness)
	require.Equal(t, &session.Perceived{Senses: []session.Sense{session.SenseHearing}}, got[0].act.Perceived, "her own ears")
}

// TestCastleTheHintDoesNotReachTheShutHall: at each their own, alice in the
// great hall behind its closed door is shown nothing of it.
func TestCastleTheHintDoesNotReachTheShutHall(t *testing.T) {
	require.Empty(t, theHintAsked(t, encounter.SharingOwn, inTheGreatHall))
}

// TestCastleTheHintReachesTheShutHallThroughAFriend: at the default, shared
// view, the same alice behind the same closed door IS shown the hint —
// through the cleric, who was spoken to and saw who said it — whole.
func TestCastleTheHintReachesTheShutHallThroughAFriend(t *testing.T) {
	got := theHintAsked(t, encounter.SharingView, inTheGreatHall)
	require.Len(t, got, 1)
	s := got[0].body
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "gate-1"}, s.Speaker, "the cleric saw who said it")
	require.Equal(t, clericParty(), s.SpokenTo, "and to whom")
	require.Nil(t, s.Hops, "a speaker named, and no distance of a friend's")
	require.Equal(t, []session.Party{clericParty()}, got[0].act.Perceived.Via, "through the cleric")
}

// walkedTo is alice walked, as a player gets there: one Move along the
// engine's own route from the admitted gate to cell, the engine looking
// again at every step. It asserts, step by step, that what she is told of
// gate-1 — the sightings the walk sends her, gained or lost — is exactly
// what a fresh look from each cell gives (the same steps taken on a copy of
// the run before the walk, asked with SeesWithin), and that at the end the
// sighting she keeps is the fresh one. A sighting kept past where it should
// end fails here. It returns whether she sees gate-1 at the end.
func (c *freeRoamCastle) walkedTo(t *testing.T, cell spatial.Position) bool {
	t.Helper()
	before := c.world(t)
	route, err := before.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: cell, Budget: 200})
	require.NoError(t, err)
	path := route.Path
	if len(path) == 0 || path[len(path)-1] != cell {
		path = append(path, cell)
	}

	// THE FRESH LOOK, cell by cell: the run as it stood before the walk,
	// with alice stood on each cell in turn and loaded afresh, asked with
	// SeesWithin. Whenever the answer changes, a sighting is owed.
	lookFrom := func(at spatial.Position) bool {
		t.Helper()
		sess, err := copyOf(c.sessions.byID["sess"])
		require.NoError(t, err)
		world, err := copyOf(c.encounters.byID["sess"])
		require.NoError(t, err)
		for i := range world.Members {
			if world.Members[i].ID == "alice" {
				world.Members[i].Cell = &encounter.PositionData{X: at.X, Y: at.Y}
			}
		}
		sess.ID, sess.Encounter = "look", "look"
		c.sessions.byID["look"], c.encounters.byID["look"] = sess, world
		defer delete(c.sessions.byID, "look")
		defer delete(c.encounters.byID, "look")
		enc, err := c.mgr.LoadWorldForTest(context.Background(), "look")
		require.NoError(t, err)
		sees, known := enc.SeesWithin("alice", "gate-1", 1_000_000)
		require.True(t, known)
		return sees
	}
	fresh, known := before.SeesWithin("alice", "gate-1", 1_000_000)
	require.True(t, known)
	var owed []string
	for _, step := range path {
		now := lookFrom(step)
		if now != fresh {
			owed = append(owed, map[bool]string{true: "gained", false: "lost"}[now]+fmt.Sprint(step))
		}
		fresh = now
	}

	walk := c.play(t, "alice walks", func() error {
		_, err := walkOn(context.Background(), c.mgr, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
		return err
	})
	var told []string
	at := spatial.Position{}
	for _, a := range walk.to("alice") {
		switch b := a.Body.(type) {
		case session.WalkAct:
			if len(b.Path) > 0 && b.Mover.ID == "alice" {
				at = b.Path[len(b.Path)-1].Cell
			}
		case session.Perception:
			if b.Sighted == nil || b.Observer.ID != "alice" {
				continue
			}
			if slices.Contains(b.Sighted.Gained, "gate-1") {
				told = append(told, "gained"+fmt.Sprint(at))
			}
			if slices.Contains(b.Sighted.Lost, "gate-1") {
				told = append(told, "lost"+fmt.Sprint(at))
			}
		}
	}
	require.Equal(t, owed, told, "what she is told of gate-1 along the walk is what a fresh look gives, step by step")

	after := c.world(t)
	held, err := after.View(&encounter.ViewInput{Member: "alice"})
	require.NoError(t, err)
	kept := false
	for _, h := range held {
		if h.Channel == perception.Sight && h.Subject == "gate-1" {
			kept = len(h.CurrentVia) > 0
		}
	}
	require.Equal(t, fresh, kept, "at the end, the sighting she keeps is the fresh one")
	return kept
}

// theHintAskedOf is the cleric, beside gate-1, asking after her friends, and
// what alice is sent of the hint.
func (c *freeRoamCastle) theHintAskedOf(t *testing.T) []found[session.SpeechAct] {
	t.Helper()
	c.placeInTheRecord(t, "cleric", besideGateOne)
	played := c.play(t, "the cleric asks after her friends", func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
			Session: "sess", Member: "cleric", Target: "gate-1", OptionID: friendsLine,
		})
		return err
	})
	return where[session.SpeechAct](played.to("alice"), func(s session.SpeechAct) bool {
		return s.Words != nil && *s.Words == theShoutedHint
	})
}

// A WALKED HERO HEARS THE HINT AS HER SIGHT ALLOWS, at each their own:
// alice, admitted at the captain, WALKS through the gate and across the
// yard, and what she is told of gate-1 is checked against a fresh look at
// every step ([freeRoamCastle.walkedTo]). The two tests below are its two
// ends.

// TestCastleAWalkedHeroAtTheEastEndHearsNoName: to the east end of the
// yard the gatehouse's wall comes between her and the gate, and the shout
// reaches her as words with no name, one room off.
func TestCastleAWalkedHeroAtTheEastEndHearsNoName(t *testing.T) {
	c := atTheAdmittedGate(t)
	c.encounters.byID["sess"].Sharing = encounter.SharingOwn
	require.False(t, c.walkedTo(t, spatial.Position{X: 6, Y: 2}), "out of gate-1's sight")
	got := c.theHintAskedOf(t)
	require.Len(t, got, 1)
	require.Equal(t, session.Party{}, got[0].body.Speaker, "no name")
	require.Equal(t, session.Party{}, got[0].body.SpokenTo)
	require.NotNil(t, got[0].body.Hops)
	require.Equal(t, 1, *got[0].body.Hops, "one room off")
}

// TestCastleAWalkedHeroInLineWithTheGateHearsHisName: across to the west
// of the yard, in line with the open gate, she still sees gate-1, and the
// same shout arrives with his name. The toolkit's castle tests, whose sight
// is a 60-foot stub, call this cell unseen; the session's sheets see 120
// feet, and play is the session.
func TestCastleAWalkedHeroInLineWithTheGateHearsHisName(t *testing.T) {
	c := atTheAdmittedGate(t)
	c.encounters.byID["sess"].Sharing = encounter.SharingOwn
	require.True(t, c.walkedTo(t, spatial.Position{X: -12, Y: 2}), "gate-1 in her sight through the open gate")
	got := c.theHintAskedOf(t)
	require.Len(t, got, 1)
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "gate-1"}, got[0].body.Speaker, "his name")
	require.Nil(t, got[0].body.Hops)
}
