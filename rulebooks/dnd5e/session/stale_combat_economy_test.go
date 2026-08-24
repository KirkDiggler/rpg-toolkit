// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// StaleCombatEconomySuite pins rpg-project#253: a member's first turn in a
// running fight must start with a fresh action economy, never one left over
// from a DIFFERENT, already-ended fight earlier in the same session.
//
// # What the live report looked like, and what it actually is
//
// Kirk (live, two browsers, 2026-08-24): a member recruited into a fight
// already in progress started their own first turn with only 5 of 30 feet
// of movement. The natural first read — "the free-roam approach spent it" —
// does not hold: [Manager.priceWalk] decides free-roam-vs-turn-clock ONCE,
// from the clock state at the top of the Move call, before any step runs, so
// a walk that starts on the world clock is priced nothing for the whole
// call regardless of what a mid-path step does. A genuinely first-ever
// combat entrant seeds a full 30ft via [character.StartTurn] exactly as
// economy.go's readyForTurn already intends.
//
// What actually reproduces "5ft left, not 30" — traced to the character
// package directly, sheet in hand, no session or encounter involved — is
// that [character.Character.ExitCombat] has NO CALLER anywhere in this
// module (`grep -rn ExitCombat rulebooks/dnd5e/session
// rulebooks/dnd5e/encounter` finds only its own definition). So
// [character.Character.InCombat] never returns to false once a member has
// taken a single combat turn in a session, and readyForTurn's
// `!sheet.InCombat()` branch — the one that unconditionally reseeds via
// StartTurn — can never fire again for them. Every LATER fight falls to
// RefreshForTurn, whose documented no-op ("a second swing cannot refill
// what the first one spent") fires whenever the new fight's round happens
// to equal the stale TurnNumber left on the sheet — and because every
// bubble's own round counter starts fresh at 1 (play/clock's Turn is
// per-bubble, never global to the session), a round-1 collision between two
// otherwise unrelated fights is the ordinary case, not an edge one.
//
// This suite reproduces it end to end, on real sheets, through the real
// dissolve path fight_starts_test.go and death_test.go's own fixtures
// already exercise: alice fights and kills a skeleton (round 1, defeat ends
// the bubble with no EndTurn ever called — encounter/dissolve.go's ByDefeat
// is a fact the composition NOTICES, never something a caller declares),
// then a second, unrelated skeleton arrives and starts a second fight —
// also its own round 1.
type StaleCombatEconomySuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestStaleCombatEconomySuite(t *testing.T) { suite.Run(t, new(StaleCombatEconomySuite)) }

func (s *StaleCombatEconomySuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	s.stream = &fakeStream{}

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: cryptWorld(s.T()),
	})
	s.Require().NoError(err)
}

// spawnAdjacentSkeleton spawns a skeleton beside alice, in plain sight —
// cryptWorld's own (2,1), one cell from alice's (1,1), which
// death_test.go's spawnSkeleton already proves both starts a fight AND
// leaves alice able to swing without moving first.
func (s *StaleCombatEconomySuite) spawnAdjacentSkeleton(id string) *session.SpawnOutput {
	s.T().Helper()
	out, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: id, Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "arriving in plain sight of alice must start a fight")
	return out
}

// storedHP reads a spawned NPC's hit points back out of the session record —
// death_test.go's own helper, copied rather than shared (this package's
// established convention — see move_turnclock_test.go's own note on it).
func (s *StaleCombatEconomySuite) storedHP(id string) int {
	s.T().Helper()
	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == id {
			return npc.HitPoints
		}
	}
	s.Require().Fail("no stored sheet for " + id)
	return -1
}

// killAdjacentSkeleton swings alice's sword at id until it falls —
// death_test.go's swingUntilTheSkeletonFalls, this suite's own copy.
func (s *StaleCombatEconomySuite) killAdjacentSkeleton(id string) {
	s.T().Helper()
	const runaway = 25
	for i := 0; i < runaway && s.storedHP(id) > 0; i++ {
		_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
			Session: "sess", Attacker: "alice", Target: id,
		})
		s.Require().NoError(err)
	}
	s.Require().Zero(s.storedHP(id), "the blows really landed on the stored sheet")
}

func (s *StaleCombatEconomySuite) afford(member string) *session.AffordOutput {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out
}

// attackDecl finds the VerbAttack declaration for a specific target.
func (s *StaleCombatEconomySuite) attackDecl(out *session.AffordOutput, target string) session.Declaration {
	s.T().Helper()
	for _, d := range out.Declarations {
		if d.Verb == session.VerbAttack && d.Target != nil && *d.Target == target {
			return d
		}
	}
	s.Require().Fail("no VerbAttack declaration for "+target, "declarations: %+v", out.Declarations)
	return session.Declaration{}
}

// TestASecondFightsFirstTurnIsNotChargedForTheFirstFightsLastOne is Kirk's
// live observation (2026-08-24), reproduced end to end: alice's first turn
// in a NEW fight must start with a fresh action economy, not whatever her
// PREVIOUS, already-ended fight happened to leave on her sheet.
func (s *StaleCombatEconomySuite) TestASecondFightsFirstTurnIsNotChargedForTheFirstFightsLastOne() {
	// Fight 1: alice kills skel-1 without EndTurn ever being called — the
	// kill itself dissolves the bubble by defeat, exactly as
	// death_test.go's TestTheLastOneDownedEndsTheFightByDefeat pins. Her
	// sheet is left mid-turn: TurnNumber 1, no action remaining (spent on
	// the swing that landed), nothing telling it the fight is over.
	s.spawnAdjacentSkeleton("skel-1")
	s.killAdjacentSkeleton("skel-1")

	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "control: defeat returned alice to free roam on her own")

	// Fight 2: an entirely separate skeleton, entirely separate bubble. Its
	// own round counter starts fresh at 1 — the SAME number fight 1's stale
	// TurnNumber already holds.
	s.spawnAdjacentSkeleton("skel-2")

	turn2, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn2.Clock)
	s.Require().Equal("alice", turn2.Active, "alice alone forms this fight — first and only in its order")
	s.Require().Equal(1, turn2.Round, "control: this fight's own round is 1, same as fight 1's stale TurnNumber")

	// THE ASSERTION: alice's first swing in fight 2 must be affordable. If
	// her action slot carried fight 1's spent state forward, RefreshForTurn
	// sees TurnNumber already matching this round and silently no-ops,
	// leaving her with no action to swing at all.
	decl := s.attackDecl(s.afford("alice"), "skel-2")
	s.True(decl.Affordable,
		"alice's first swing in a NEW fight must be affordable — not refused for an action fight 1 already spent")
}
