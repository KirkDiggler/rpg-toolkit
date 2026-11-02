// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// one_load_test.go holds rpg-project#753's T12 finding shut: a verb loads its
// encounter from stored data exactly once. Every resolution a verb runs is
// lent the instance the verb holds, so none loads its own, and the landing
// adopts that same instance instead of loading the world a resolution handed
// back. Before this a world-clock Move loaded twice and an Attack three times.
//
// The count is the session's own loads; resolution cannot load at all here,
// because every input this package builds lends the encounter and a landing
// refuses an output that carries a world (land.go, step 0).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// loadsOnce runs verb and requires it loaded exactly one encounter.
func loadsOnce(t *testing.T, loads func() int, label string, verb func() error) {
	t.Helper()
	before := loads()
	require.NoError(t, verb(), label)
	require.Equal(t, 1, loads()-before, "%s loads its encounter once", label)
}

// The gatehouse fight, verb by verb: a world-clock Move, two Speaks (the
// second turns the garrison and a fight forms), a turn-clock Move, an Attack,
// an EndTurn the captain and the world answer, and a Move that provokes the
// captain's opportunity attack.
func TestEveryVerbOfTheGatehouseFightLoadsOnce(t *testing.T) {
	g := openTheGatehouse(14, 5, 15)
	loads := session.CountEncounterLoadsForTest(t)
	ctx := context.Background()
	route := func(anchor func(*encounter.Encounter) spatial.Position, budget int) []spatial.Position {
		enc, err := g.mgr.LoadWorldForTest(ctx, "sess")
		require.NoError(t, err)
		r, err := enc.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: anchor(enc), Budget: budget})
		require.NoError(t, err)
		return r.Path
	}
	standAside := func() error {
		_, err := g.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
		return err
	}

	path := route(func(*encounter.Encounter) spatial.Position { return inTheGatehouse }, 200)
	if len(path) == 0 || path[len(path)-1] != inTheGatehouse {
		path = append(path, inTheGatehouse)
	}
	loadsOnce(t, loads, "Move on the world clock", func() error {
		_, err := g.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
		return err
	})
	loadsOnce(t, loads, "Speak", standAside)
	loadsOnce(t, loads, "Speak that forms a fight", standAside)

	path = route(func(enc *encounter.Encounter) spatial.Position { return standingAt(enc, "gate-1") }, 200)
	moveID := currentMoveID(t, g.mgr, "sess", "alice")
	loadsOnce(t, loads, "Move on the turn clock", func() error {
		_, err := g.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path, DeclarationID: moveID})
		return err
	})

	for i := range g.sessions.byID["sess"].NPCs {
		if g.sessions.byID["sess"].NPCs[i].ID == "gate-1" {
			g.sessions.byID["sess"].NPCs[i].HitPoints = 1
		}
	}
	attackID := currentAttackID(t, g.mgr, "sess", "alice")
	loadsOnce(t, loads, "Attack", func() error {
		_, err := g.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "alice", Target: "gate-1", DeclarationID: attackID})
		return err
	})

	endID := currentEndTurnID(t, g.mgr, "sess", "alice")
	loadsOnce(t, loads, "EndTurn the monsters and the world answer", func() error {
		_, err := g.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "alice", DeclarationID: endID})
		return err
	})

	path = route(func(*encounter.Encounter) spatial.Position { return spatial.Position{X: -10, Y: 12} }, 3)
	moveID = currentMoveID(t, g.mgr, "sess", "alice")
	loadsOnce(t, loads, "Move that provokes an opportunity attack", func() error {
		_, err := g.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path, DeclarationID: moveID})
		return err
	})
}

// The troupe's door: OpenDoor, whose opening brings the far room's guards in.
func TestOpenDoorLoadsOnce(t *testing.T) {
	g := openTheGatehouse(12, 12, 15)
	ctx := context.Background()
	walkOnto := func(anchor func(*encounter.Encounter) spatial.Position, onto bool) {
		enc, err := g.mgr.LoadWorldForTest(ctx, "sess")
		require.NoError(t, err)
		cell := anchor(enc)
		r, err := enc.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: cell, Budget: 200})
		require.NoError(t, err)
		path := r.Path
		if onto && (len(path) == 0 || path[len(path)-1] != cell) {
			path = append(path, cell)
		}
		_, err = g.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
		require.NoError(t, err)
	}
	walkOnto(func(enc *encounter.Encounter) spatial.Position { return standingAt(enc, "pedlar") }, false)
	_, err := g.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "pedlar", OptionID: "tonight"})
	require.NoError(t, err)
	walkOnto(func(enc *encounter.Encounter) spatial.Position { return standingAt(enc, "captain") }, false)
	_, err = g.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "troupe"})
	require.NoError(t, err)
	walkOnto(func(*encounter.Encounter) spatial.Position { return beneathTheGate }, true)

	loads := session.CountEncounterLoadsForTest(t)
	loadsOnce(t, loads, "OpenDoor", func() error {
		_, err := g.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: castleGate})
		return err
	})
}

// A short rest: the party's sheets, an hour on the world clock, one beat.
func (s *RestSuite) TestARestLoadsOnce() {
	s.start(freeRoamDuelWorld(), woundedFighter("alice"), armedFighter("bob"))
	loads := session.CountEncounterLoadsForTest(s.T())
	loadsOnce(s.T(), loads, "Rest", func() error {
		_, err := s.mgr.Rest(context.Background(), &session.RestInput{
			Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "alice", HitDice: 2}},
		})
		return err
	})
}

// A cast that pauses on an offer, and the React that answers it: the
// resumed machine is lent the encounter the React loaded, like any other.
func (s *CastSuite) TestACastAndTheReactThatAnswersItEachLoadOnce() {
	cleric := healingWordCleric()
	cleric.KnownSpells = append(cleric.KnownSpells, refs.Spells.GuidingBolt().String())
	s.scene(cleric, 4, 18, 1, 1, 1, 1)
	inspired, err := conditions.NewInspiredCondition("cleric", "bard", conditions.InspiredDie).ToJSON()
	s.Require().NoError(err)
	stored := s.characters.byID["cleric"]
	stored.Conditions = append(stored.Conditions, inspired)
	loads := session.CountEncounterLoadsForTest(s.T())

	row := s.castRow(spells.GuidingBolt)
	loadsOnce(s.T(), loads, "Cast", func() error {
		out, err := s.mgr.Cast(context.Background(), &session.CastInput{
			Session: "sess", Member: s.member, Target: "skeleton", DeclarationID: row.ID,
		})
		if err == nil {
			s.True(out.Posed, "the cast pauses on its offer")
		}
		return err
	})

	react := currentDeclaration(s.T(), s.mgr, "sess", "cleric", session.VerbReact)
	loadsOnce(s.T(), loads, "React", func() error {
		_, err := s.mgr.React(context.Background(), &session.ReactInput{
			Session: "sess", Member: "cleric", DeclarationID: react.ID, Answer: session.Decline(),
		})
		return err
	})
}

// An activation: rage, in a fight.
func TestActivateLoadsOnce(t *testing.T) {
	mgr, _, _, _ := aFight(t, ragingBarbarian("alice", 2), []int{1, 1})
	id := activationSelector(t, mgr, "alice", "dnd5e:features:rage")
	loads := session.CountEncounterLoadsForTest(t)
	loadsOnce(t, loads, "Activate", func() error {
		_, err := mgr.Activate(context.Background(), &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
		return err
	})
}

// A lock: the check against the DC, and the door it opens.
func (s *DoorsSuite) TestUnlockLoadsOnce() {
	s.startWith(gatedWorld(tombLock()), deftCharacter("alice", 14))
	loads := session.CountEncounterLoadsForTest(s.T())
	loadsOnce(s.T(), loads, "Unlock", func() error {
		_, err := s.mgr.Unlock(context.Background(), &session.UnlockInput{Session: "sess", Member: "alice", Door: "gate"})
		return err
	})
}

// A footprint door opened, crossed, backed off, and closed.
func (s *CloseDoorSuite) TestCloseDoorLoadsOnce() {
	ctx := context.Background()
	s.startWith(footprintLeafWorld(encounter.DoorIsClosed()))
	_, err := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)
	s.walkOntoLeaf(s.mgr)
	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(3, 1)}})
	s.Require().NoError(err)
	loads := session.CountEncounterLoadsForTest(s.T())
	loadsOnce(s.T(), loads, "CloseDoor", func() error {
		_, err := s.mgr.CloseDoor(ctx, &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
		return err
	})
}
