// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MonsterTurnTestSuite is the tomb: the gate this whole wave (rpg-project#254)
// lands or does not land on. Five claims, one seam under test throughout —
// session's own Striker bound to the live scope.enc, the shared member
// record filled at Join and Spawn, and sight read from data rather than a
// flat constant.
type MonsterTurnTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
}

func TestMonsterTurnSuite(t *testing.T) {
	suite.Run(t, new(MonsterTurnTestSuite))
}

func (s *MonsterTurnTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
}

// tombRoom is one open hall, wide enough to close a few cells of distance
// in — everything these tests need except (b), which adds its own wall.
func tombRoom(width, height int) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{
			{ID: "tomb", Width: width, Height: height},
		}},
		Endings:   []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		panic(err) // construction-time only; every call site is a fixed literal
	}
	data := enc.ToData()
	return &data
}

// tombManager builds a manager over this suite's stores, with the driver
// and dice each test declares — a fighter and any monsters go through
// session's own Join/Spawn, never authored straight into MemberInput, so
// every gate test exercises the real member-record-filling this wave built.
func (s *MonsterTurnTestSuite) tombManager(driver session.TurnDriver, dice session.Roller) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Dice: dice, TurnDriver: driver,
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(armedFighter("fighter")),
		Events:     session.DiscardEvents{},
	})
	s.Require().NoError(err)
	return mgr
}

// storyBeats reads member's whole story and returns just the beat kinds, in
// order — the shape every gate test below asserts its sequence against.
func (s *MonsterTurnTestSuite) storyBeats(mgr *session.Manager, member string) []string {
	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	beats := make([]string, len(entries))
	for i, e := range entries {
		var body struct {
			Beat string `json:"beat"`
		}
		s.Require().NoError(json.Unmarshal(e.Payload, &body))
		beats[i] = body.Beat
	}
	return beats
}

// (a) A skeleton four cells off closes the distance and attacks: moved
// beats to adjacent, a struck (or missed) beat naming a real attack, then
// the turn hands cleanly back.
//
// The dice are deliberately plain (testDice{}'s flat 10): the skeleton's
// shortsword compiles to +4, and armedFighter's effective AC against an
// unarmoured DEX-14 defender is 12 (see attack_test.go's duelAC) — 14
// against 12 is a hit with no scripting required, which is the point:
// nothing about THIS gate is about the dice.
func (s *MonsterTurnTestSuite) TestSkeletonClosesAndAttacks() {
	ctx := context.Background()
	mgr := s.tombManager(session.Behavior(), testDice{})

	_, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 4, Y: 0}, // four cells off, well within the shared 24-cell default range
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "spawning in sight and in range must start the fight on the spot")
	s.Equal([]string{"fighter", "skel-1"}, spawned.Formed.Order, "fighter's ID sorts first, so the tied roll breaks to her")

	before := len(s.storyBeats(mgr, "fighter"))
	out, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err, "the skeleton's whole turn — move, strike, end — drives inside this one call")

	// Isolated to what THIS EndTurn call itself produced, with the earlier
	// joined/joined/bubble-formed prefix left out — this gate is about the
	// driven turn, not about arrival.
	beats := s.storyBeats(mgr, "fighter")[before:]
	s.Require().NotEmpty(beats)
	s.Equal("turn-ended", beats[0], "fighter's own end-turn beat comes first")

	// At least one moved beat (closing the distance), exactly one struck-or-
	// missed beat (the swing), and the skeleton's own turn ending last —
	// the shape the brief itself describes: "moved beats to adjacent, then
	// struck/missed ... then turn_ended".
	middle := beats[1 : len(beats)-1]
	s.NotEmpty(middle, "the skeleton had four cells to close before it could swing")
	for _, b := range middle[:len(middle)-1] {
		s.Equal("moved", b, "everything before the swing is the approach")
	}
	swing := middle[len(middle)-1]
	s.Contains([]string{"struck", "missed"}, swing)
	s.Equal("turn-ended", beats[len(beats)-1], "the skeleton's own turn closes the round")

	s.Equal("fighter", out.Next, "a two-member fight wraps straight back to whoever led it")
	s.True(out.RoundWrapped)

	// The swing itself: recorded with a real attack identity, not a blank
	// one — the exact gap #1196 closed in resolution.
	entries, err := mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	entries = entries[before:]
	var swingBody struct {
		Beat   string            `json:"beat"`
		Actor  string            `json:"actor"`
		Target []string          `json:"targets"`
		Attack session.AttackRef `json:"attack"`
	}
	for _, e := range entries {
		var peek struct {
			Beat string `json:"beat"`
		}
		s.Require().NoError(json.Unmarshal(e.Payload, &peek))
		if peek.Beat == "struck" || peek.Beat == "missed" {
			s.Require().NoError(json.Unmarshal(e.Payload, &swingBody))
			break
		}
	}
	s.Equal("skel-1", swingBody.Actor)
	s.Equal([]string{"fighter"}, swingBody.Target)
	s.NotEmpty(swingBody.Attack.Ref, "Attack{ref, name}: the ref side")
	s.NotEmpty(swingBody.Attack.Name, "Attack{ref, name}: the name side (resolution#1196)")

	if swing == "struck" {
		char, err := s.encounters.GetEncounter(ctx, "world") // sanity: world persisted at all
		s.Require().NoError(err)
		s.NotNil(char)
	}
}

// (b) A skeleton behind a wall from the only player never sees them: sight
// is LOS-bounded, not range alone, so spawning it starts no fight — there
// is no monster's turn here to drive, which is the point.
func (s *MonsterTurnTestSuite) TestBlindSkeletonBehindAWallNeverJoinsTheFight() {
	ctx := context.Background()
	mgr := s.tombManager(session.Behavior(), testDice{})

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{
			{ID: "tomb", Width: 12, Height: 6, Boundaries: fullColumnWall(6, 6)},
		}},
		Endings:   []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	world := enc.ToData()

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &world})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-2", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 8, Y: 0}, // two cells past the wall — in range, blocked by it
	})
	s.Require().NoError(err)
	s.Nil(spawned.Formed, "a wall in the way must keep this a spawn, not an ambush")

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "skel-2"})
	s.Require().NoError(err)
	s.Equal(session.ClockWorld, turn.Clock, "nobody drove a turn nobody's clock ever reached")
}

// (c) A driver that declares an attack out of reach does not abort the
// call it was discovered in: the bad intent just ends that member's own
// turn, exactly as a Pass would, and the caller's own verb still succeeds.
func (s *MonsterTurnTestSuite) TestBadIntentEndsOnlyTheMonstersTurn() {
	ctx := context.Background()
	mgr := s.tombManager(reachlessAttacker{}, testDice{})

	_, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 4, Y: 0},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed)

	before := len(s.storyBeats(mgr, "fighter"))
	out, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err, "a bad intent must not surface as an error on the caller's own verb")
	s.Equal("fighter", out.Next)
	s.True(out.RoundWrapped)

	beats := s.storyBeats(mgr, "fighter")[before:]
	// Two turn-ended beats and nothing else: no moved (this driver never
	// moves), no struck or missed (the attack it declared was never in
	// reach to execute).
	s.Equal([]string{"turn-ended", "turn-ended"}, beats)
}

// (d) Sight is read from data: a monster with no stated senses shares the
// SAME default a character does (rpg-project#254 design §5) — sees a
// player at 24 cells and not at 25, defaultSightFeet's own boundary
// (120 feet — sight.go).
func (s *MonsterTurnTestSuite) TestSightRangeGatesContact() {
	ctx := context.Background()

	s.Run("twenty-five cells: out of range", func() {
		s.SetupTest()
		mgr := s.tombManager(session.Pass{}, testDice{})
		_, err := mgr.StartSession(ctx, &session.StartSessionInput{
			Session: "sess", Encounter: "world", World: tombRoom(30, 6),
		})
		s.Require().NoError(err)
		_, err = mgr.Join(ctx, &session.JoinInput{
			Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
		})
		s.Require().NoError(err)
		spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
			Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
			Position: spatial.Position{X: 25, Y: 0},
		})
		s.Require().NoError(err)
		s.Nil(spawned.Formed, "twenty-five cells is one past the shared 24-cell default")
	})

	s.Run("twenty-four cells: in range", func() {
		s.SetupTest()
		mgr := s.tombManager(session.Pass{}, testDice{})
		_, err := mgr.StartSession(ctx, &session.StartSessionInput{
			Session: "sess", Encounter: "world", World: tombRoom(30, 6),
		})
		s.Require().NoError(err)
		_, err = mgr.Join(ctx, &session.JoinInput{
			Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
		})
		s.Require().NoError(err)
		spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
			Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
			Position: spatial.Position{X: 24, Y: 0},
		})
		s.Require().NoError(err)
		s.Require().NotNil(spawned.Formed, "twenty-four cells is exactly the shared default's own boundary")
	})
}

// (e) session.Pass{} still passes: the reference no-op driver, wired
// through Striker's whole new capability chain, produces exactly what it
// always did — the clock reaches the monster and comes straight back.
func (s *MonsterTurnTestSuite) TestPassDriverStillPasses() {
	ctx := context.Background()
	mgr := s.tombManager(session.Pass{}, testDice{})

	_, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 4, Y: 0},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed)

	before := len(s.storyBeats(mgr, "fighter"))
	out, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal("fighter", out.Next)
	s.True(out.RoundWrapped)

	s.Equal([]string{"turn-ended", "turn-ended"}, s.storyBeats(mgr, "fighter")[before:])
}

// fullColumnWall blocks line of sight the full height of a room at column
// atX — no gap, unlike squareSeam's own doorway — because (b) is testing
// that a wall with nothing to peek through blocks contact outright.
func fullColumnWall(atX, rows int) []spatial.Boundary {
	out := make([]spatial.Boundary, 0, rows)
	for y := 0; y < rows; y++ {
		out = append(out, spatial.Boundary{
			From:              spatial.Position{X: float64(atX), Y: float64(y)},
			To:                spatial.Position{X: float64(atX + 1), Y: float64(y)},
			BlocksMovement:    true,
			BlocksLineOfSight: true,
		})
	}
	return out
}

// reachlessAttacker always declares an attack against the fighter using the
// first action it is told about, regardless of whether it is anywhere near
// reach — (c)'s bad intent, by construction rather than by accident.
type reachlessAttacker struct{}

func (reachlessAttacker) Act(view session.MonsterView) (session.TurnIntent, error) {
	for _, seen := range view.Seen {
		if seen.Kind == session.KindPlayer {
			if len(view.Actions) == 0 {
				return session.Pass{}, nil
			}
			return session.Attack{Target: seen.ID, Action: view.Actions[0].Ref}, nil
		}
	}
	return session.Pass{}, nil
}
