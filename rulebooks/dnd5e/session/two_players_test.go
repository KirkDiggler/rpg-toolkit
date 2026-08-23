// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TwoPlayersTestSuite is rpg-project#257 slice 3's own gate: the stream has
// never been proven whole with two real humans in one session. Every prior
// wave's own fixtures (MonsterTurnTestSuite, MoveTurnClockSuite,
// TestRoundTwoStruckReachesTheLiveSubscriber) join at most one real player
// and either discard events or read the log — nothing before this suite has
// ever had two joined characters and two independent EventStream recipients
// watching the same fight at once.
//
// armedBarbarian ("barbarian") joins alongside armedFighter ("fighter") —
// the design doc's own two-browser walk (rpg-project's ideas/stream-whole
// design.md on the origin/ideas/stream-whole branch, §6), reduced to what a
// fake EventStream can pin. skel-1 is spawned in plain sight of both and
// pulls them into one bubble; skel-2 is spawned behind a genuine
// LOS+movement wall, matching
// TestBlindSkeletonBehindAWallNeverJoinsTheFight's own fixture shape, and
// never joins.
//
// Both players are given 100 HP (armedFighter's own 24/28 raised the same
// way TestRoundTwoStruckReachesTheLiveSubscriber's own fixture is: "high hit
// points keep two ordinary rounds of being hit from wandering into a
// defeat-ends-the-fight scene by accident") — deliberately, since this
// suite's own killing-blow case is written separately and this suite's own
// turn-order walk must stay clear of a defect found while writing this
// suite (reported to team-lead separately, not yet issue-numbered): a
// driven monster that downs one of two players without deciding the
// fight re-enters the drive loop for an undocumented second turn. NOT
// exercised here on purpose.
type TwoPlayersTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestTwoPlayersSuite(t *testing.T) {
	suite.Run(t, new(TwoPlayersTestSuite))
}

// armedBarbarian is armedFighter's own twin, wearing the other class this
// suite needs to prove two DIFFERENT real players rather than two copies of
// the same one — everything else about the sheet (weapon, proficiency,
// ability scores, effective AC) stays identical on purpose, so the fight's
// own arithmetic is exactly duelAC's, proven once and not re-derived here.
func armedBarbarian(id string) *character.Data {
	c := armedFighter(id)
	c.ClassID = classes.Barbarian
	return c
}

// twoPlayerTomb is one open hall wide enough to close a few cells of
// distance in, walled at column atX exactly as
// TestBlindSkeletonBehindAWallNeverJoinsTheFight's own fixture is — the
// same fullColumnWall this file's sibling already defines, reused rather
// than re-derived so a wall in this suite means what it already means
// everywhere else in this package.
func twoPlayerTomb(t fataler, width, height, atX int) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{
			{ID: "tomb", Width: width, Height: height, Boundaries: fullColumnWall(atX, height)},
		}},
		Endings:   []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the two-player tomb: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *TwoPlayersTestSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.stream = &fakeStream{}

	fighter := armedFighter("fighter")
	fighter.HitPoints, fighter.MaxHitPoints = 100, 100
	barbarian := armedBarbarian("barbarian")
	barbarian.HitPoints, barbarian.MaxHitPoints = 100, 100

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(fighter, barbarian),
		Events:     s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	ctx := context.Background()
	world := twoPlayerTomb(s.T(), 12, 6, 6)
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: world})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0}})
	s.Require().NoError(err)
	_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "barbarian", Position: spatial.Position{X: 1, Y: 0}})
	s.Require().NoError(err)
}

// recipientSeqs returns, in the order they were published, the Seq values
// of every event in events addressed to recipient.
func recipientSeqs(events []session.Event, recipient string) []uint64 {
	var out []uint64
	for _, e := range events {
		if e.Recipient == recipient {
			out = append(out, e.Seq)
		}
	}
	return out
}

// assertContiguous fails unless seqs is non-empty, strictly increasing, and
// gap-free — (b)'s own "no gap, no duplicate" stated as one check on the
// values rather than a description of them.
func (s *TwoPlayersTestSuite) assertContiguous(seqs []uint64, who string) {
	s.Require().NotEmpty(seqs, "%s must have received something across two full rounds", who)
	for i := 1; i < len(seqs); i++ {
		s.Equal(seqs[i-1]+1, seqs[i], "%s: seq %d must immediately follow %d — no gap, no duplicate", who, seqs[i], seqs[i-1])
	}
}

// TestTwoPlayersOneSession is the proof itself (rpg-project#257 slice 3,
// design rpg-project#258 §4): fighter and barbarian, two independently
// addressed recipients on one fake EventStream, walking two full rounds
// against a driven skeleton while a second, walled-off skeleton never hears
// a thing worth keeping quiet about.
func (s *TwoPlayersTestSuite) TestTwoPlayersOneSession() {
	ctx := context.Background()

	spawned1, err := s.mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 3, Y: 0},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned1.Formed, "skel-1 is in plain sight of both players and must pull them into one bubble")
	// testDice{}'s flat rolls tie every initiative roll, so the order falls
	// to the ID tie-break the seam documents (initiative.go) — alphabetical,
	// same as TestSkeletonClosesAndAttacks's own "fighter's ID sorts first"
	// comment. "barbarian" < "fighter" < "skel-1", so the REAL order this
	// dice+ID combination produces is barbarian first, not the fighter-first
	// prose a walkthrough might expect — asserted here rather than fought.
	s.Equal([]string{"barbarian", "fighter", "skel-1"}, spawned1.Formed.Order)

	spawned2, err := s.mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-2", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 8, Y: 0}, // past the wall at column 6
	})
	s.Require().NoError(err)
	s.Require().Nil(spawned2.Formed, "a wall in the way must keep skel-2 a spawn, not an ambush")

	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "skel-2"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "skel-2 never joins the fight it cannot perceive")

	// --- (d): a player's Move/Attack on the other player's turn ---
	//
	// It is barbarian's turn (first in the real order). fighter tries to act
	// anyway, on both verbs the brief names, and neither may publish
	// anything: a refusal this early must not have touched the story.
	beforeRefusal := len(s.stream.published)

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{{X: 0, Y: 1}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotYourTurn)

	_, err = s.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "fighter", Target: "skel-1"})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotYourTurn)

	s.Equal(beforeRefusal, len(s.stream.published), "a refusal this early must publish nothing at all")

	// --- (b)/(c): two full rounds, turn order and Next walked exactly ---
	roundsStart := len(s.stream.published)

	// Round 1, barbarian: passes her own turn (a player's turn needs no
	// declared action before EndTurn — the same convention every prior
	// suite's own driven-turn gates use).
	out1, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "barbarian"})
	s.Require().NoError(err)
	s.Equal("fighter", out1.Next)
	s.False(out1.RoundWrapped)

	// Round 1, fighter: her own end drives skel-1's WHOLE turn inside this
	// one call — closing one cell to reach barbarian (the closer of the two,
	// at distance 2 against fighter's 3), striking, then wrapping the round
	// back to barbarian, exactly as TestSkeletonClosesAndAttacks's own
	// single-player shape does with a second real player added.
	beforeRound1Drive := len(s.stream.published)
	out2, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal("barbarian", out2.Next)
	s.True(out2.RoundWrapped, "skel-1 was last in the order — its own end wraps the round")

	round1Drive := s.stream.published[beforeRound1Drive:]
	fighterRound1 := recipientSeqs(round1Drive, "fighter")
	s.Require().Len(fighterRound1, 4, "turn-ended(fighter), moved(skel-1), struck(skel-1), turn-ended(skel-1) — one beat set, fanned to fighter")

	// Round 2, barbarian: same shape as round 1's own opening.
	out3, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "barbarian"})
	s.Require().NoError(err)
	s.Equal("fighter", out3.Next)
	s.False(out3.RoundWrapped)

	// Round 2, fighter: skel-1 is already adjacent to barbarian from round
	// 1's own approach, so this round's driven turn is a bare strike —
	// exactly the "closer standing target already in reach" case
	// buildMonsterView's own Path-truncation exists for, with no move beat
	// at all.
	beforeRound2Drive := len(s.stream.published)
	out4, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal("barbarian", out4.Next)
	s.True(out4.RoundWrapped)

	round2Drive := s.stream.published[beforeRound2Drive:]
	fighterRound2 := recipientSeqs(round2Drive, "fighter")
	s.Require().Len(fighterRound2, 3, "turn-ended(fighter), struck(skel-1), turn-ended(skel-1) — no move beat once already adjacent")

	// (b): seqs are contiguous per recipient across the whole two-round
	// window, for BOTH real players — no gap, no duplicate.
	twoRounds := s.stream.published[roundsStart:]
	s.assertContiguous(recipientSeqs(twoRounds, "fighter"), "fighter")
	s.assertContiguous(recipientSeqs(twoRounds, "barbarian"), "barbarian")

	// Each player sees the other's moved/struck beats, not only her own turn
	// ending — the actual content check behind (a)'s "each sees the other's"
	// clause, independent of the audience-scoping question below.
	for _, who := range []string{"fighter", "barbarian"} {
		var sawStruck, sawMoved bool
		for _, e := range twoRounds {
			if e.Recipient != who {
				continue
			}
			switch e.Kind {
			case session.EventStruck:
				sawStruck = true
			case session.EventMoved:
				sawMoved = true
			}
		}
		s.True(sawStruck, "%s must see skel-1's struck beats", who)
		s.True(sawMoved, "%s must see skel-1's round-1 approach", who)
	}

	// --- (a), as a documented negative control ---
	//
	// NOT what a perception-scoped audience would do — see
	// rulebooks/dnd5e/session/events_test.go's own
	// TestOneBeatBecomesPerRecipientEvents, whose comment this one mirrors.
	// skel-2 is walled off with no line of sight to the fight (confirmed
	// above: ClockOf never leaves ClockWorld) and TODAY still receives every
	// beat of it: Record, appendClockBeat and appendMovementBeat
	// (rulebooks/dnd5e/encounter) address every beat to rosterIDs() — every
	// CURRENT member of the whole encounter, unconditionally, with no sight,
	// LOS, or bubble-membership check anywhere in that path. That is
	// rpg-toolkit#940 ("story beats are addressed to every member regardless
	// of perception"), open, and docs/ideas/session-sdk/scenes.md's own
	// Scene 7 names the identical gap for a human two rooms away. When #940
	// lands and audiences become perception-scoped, skel-2's own count below
	// flips to zero and THAT failure is the reminder to update this test —
	// not a change made speculatively here.
	skel2Seqs := recipientSeqs(twoRounds, "skel-2")
	s.NotEmpty(skel2Seqs,
		"TODAY the composition addresses every beat to every member (rpg-toolkit#940, rpg-project#257); "+
			"when audiences are perception-scoped this becomes Empty")

	// --- (e): the driven killing-blow case, two players ---
	// Covered in TestDrivenKillingBlowDissolvesCleanlyWithTwoPlayers, its own
	// test method — deliberately not folded into this walk, since forcing a
	// down here would risk this suite's own turn-order assertions above
	// wandering into the newly found double-turn defect (reported separately) by accident.
}

// TestDrivenKillingBlowDissolvesCleanlyWithTwoPlayers is (e): #1202's own
// dissolved-bubble case, reproduced with two real players rather than one —
// the shape unreachable before rpg-toolkit#1207's own fix, since a driven
// strike that downs a teammate without deciding the fight used to hand the
// SAME monster an undocumented second turn (see two_players_test.go's own
// suite doc). Held as its own standalone test rather than a case inside
// TestTwoPlayersOneSession precisely because it needs low HP to force both
// downs, which that suite's own survival walk deliberately avoids.
//
// fighter and barbarian both start at 10 HP — one hit (12 damage under
// testDice{}, verified against duelAC 12 the same way every other fixture
// in this package is) downs either outright. skel-1 spawns adjacent to
// barbarian, one cell short of fighter.
//
// Round 1: barbarian's own turn passes. Fighter's own EndTurn drives skel-1,
// whose ONE attack downs barbarian — the fight is NOT decided (fighter still
// stands) — and skel-1's own turn ends normally, wrapping back to fighter.
// This is the exact scenario rpg-toolkit#1207 fixed: without it, skel-1
// would have gone on to a second, undocumented turn and struck fighter too,
// right here.
//
// Round 2: fighter's own EndTurn drives skel-1 again. skel-1's ONE attack
// this time downs fighter — the last player standing — deciding the fight:
// #1202's own dissolved-bubble path fires, EndTurnOutput.Next reports the
// CALLER (fighter) rather than a bubble that no longer exists, and both
// players — barbarian already down, fighter just now — receive fight_ended.
func TestDrivenKillingBlowDissolvesCleanlyWithTwoPlayers(t *testing.T) {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	fighter := armedFighter("fighter")
	fighter.HitPoints, fighter.MaxHitPoints = 10, 10
	barbarian := armedBarbarian("barbarian")
	barbarian.HitPoints, barbarian.MaxHitPoints = 10, 10
	stream := &fakeStream{}

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: sessions, Encounters: encounters,
		Characters: newFakeCharacters(fighter, barbarian),
		Events:     stream,
	})
	require.NoError(t, err)

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	require.NoError(t, err)

	_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0}})
	require.NoError(t, err)
	_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "barbarian", Position: spatial.Position{X: 1, Y: 0}})
	require.NoError(t, err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 0},
	})
	require.NoError(t, err)
	require.NotNil(t, spawned.Formed)
	require.Equal(t, []string{"barbarian", "fighter", "skel-1"}, spawned.Formed.Order,
		"same tie-break as TestTwoPlayersOneSession's own — barbarian sorts first")

	// Round 1: barbarian passes her own turn.
	out1, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "barbarian"})
	require.NoError(t, err)
	require.Equal(t, "fighter", out1.Next)

	// Round 1: fighter's own end drives skel-1's WHOLE turn — the fix under
	// test is that this is exactly ONE attack, not two.
	stream.published = nil
	out2, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err)
	require.Equal(t, "fighter", out2.Next, "a two-member order (barbarian spliced) wraps skel-1's own end back to fighter")
	require.True(t, out2.RoundWrapped)

	round1Drive := stream.published
	round1Fighter := recipientSeqs(round1Drive, "fighter")
	require.Len(t, round1Fighter, 6,
		"turn-ended(fighter), struck(barbarian), down(barbarian), transferred(barbarian), moved(skel-1), turn-ended(skel-1)")

	var round1Struck []session.Event
	for _, e := range round1Drive {
		if e.Recipient == "fighter" && e.Kind == session.EventStruck {
			round1Struck = append(round1Struck, e)
		}
	}
	require.Len(t, round1Struck, 1, "the fix: skel-1's own turn is exactly one attack, not two")
	body, ok := round1Struck[0].Body.(session.StruckBody)
	require.True(t, ok)
	require.Equal(t, "barbarian", body.Target)
	require.Equal(t, "skel-1", body.Attacker)

	// Fighter — the survivor — was never touched this round: not struck,
	// still standing, still in the fight.
	for _, e := range round1Drive {
		if e.Kind == session.EventStruck {
			if b, ok := e.Body.(session.StruckBody); ok {
				require.NotEqual(t, "fighter", b.Target, "fighter must not be attacked in round 1 — that is rpg-toolkit#1207's own defect")
			}
		}
	}
	clockOf, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, clockOf.Clock, "fighter is alive and the fight is still running")

	// Both players — barbarian already down, fighter still standing — heard
	// the same round-1 beats (rpg-toolkit#940's own current broadcast
	// behaviour, same as TestTwoPlayersOneSession's own negative control).
	round1Barbarian := recipientSeqs(round1Drive, "barbarian")
	require.Equal(t, round1Fighter, round1Barbarian, "both players receive the identical round-1 beat set")

	// Round 2: fighter's own end drives skel-1 a second time. This time
	// skel-1's one attack downs fighter too — the last player standing —
	// deciding the fight mid-drive: rpg-toolkit#1202's own case, now with
	// two players in it.
	stream.published = nil
	out3, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err, "a driven turn that wins the fight must not abort the caller's own EndTurn")
	require.Equal(t, "fighter", out3.Next,
		"there is no bubble left to name a turn IN; EndTurnOutput.Next reports the caller instead (rpg-toolkit#1202)")
	require.False(t, out3.RoundWrapped, "the fight ended mid-round, not by cycling back to its top")

	round2Drive := stream.published
	var round2Struck []session.Event
	var fightEnded []session.Event
	for _, e := range round2Drive {
		if e.Recipient != "fighter" {
			continue
		}
		switch e.Kind {
		case session.EventStruck:
			round2Struck = append(round2Struck, e)
		case session.EventFightEnded:
			fightEnded = append(fightEnded, e)
		}
	}
	require.Len(t, round2Struck, 1, "skel-1's own killing blow is still exactly one attack")
	body2, ok := round2Struck[0].Body.(session.StruckBody)
	require.True(t, ok)
	require.Equal(t, "fighter", body2.Target)

	require.Len(t, fightEnded, 1, "fighter — the caller — hears the fight end")
	endedBody, ok := fightEnded[0].Body.(session.FightEndedBody)
	require.True(t, ok)
	require.Equal(t, session.DissolveByDefeat, endedBody.Cause)

	// Both players get fight_ended — barbarian, already down since round 1,
	// hears it too (rpg-toolkit#940's own current broadcast behaviour).
	var barbarianFightEnded int
	for _, e := range round2Drive {
		if e.Recipient == "barbarian" && e.Kind == session.EventFightEnded {
			barbarianFightEnded++
		}
	}
	require.Equal(t, 1, barbarianFightEnded, "barbarian must also receive fight_ended")

	for _, who := range []string{"fighter", "barbarian"} {
		final, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: who})
		require.NoError(t, err)
		require.Equal(t, session.ClockWorld, final.Clock, "%s: the fight the killing blow ended left nobody still in it", who)
	}
}
