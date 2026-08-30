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
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, width, height)},
		},
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

// (a) A skeleton four cells off attacks with its shortbow without inventing
// an approach: a struck (or missed) beat naming a real attack, then the turn
// hands cleanly back.
//
// The dice are deliberately plain (testDice{}'s flat 10): the skeleton's
// shortsword compiles to +4, and armedFighter's effective AC against an
// unarmoured DEX-14 defender is 12 (see attack_test.go's duelAC) — 14
// against 12 is a hit with no scripting required, which is the point:
// nothing about THIS gate is about the dice.
func (s *MonsterTurnTestSuite) TestSkeletonAttacksFromRange() {
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
	out, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter")})
	s.Require().NoError(err, "the skeleton's whole turn — strike, end — drives inside this one call")

	// Isolated to what THIS EndTurn call itself produced, with the earlier
	// joined/joined/bubble-formed prefix left out — this gate is about the
	// driven turn, not about arrival.
	beats := s.storyBeats(mgr, "fighter")[before:]
	s.Require().NotEmpty(beats)
	s.Equal("turn-ended", beats[0], "fighter's own end-turn beat comes first")

	// Exactly one struck-or-missed beat and the skeleton's own turn ending
	// last. Four cells is inside shortbow range, so no move beat is authored.
	middle := beats[1 : len(beats)-1]
	s.Require().Len(middle, 1, "the shortbow attack needs no approach")
	swing := middle[0]
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
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)},
			// Every crossing between columns 6 and 7, no gap — unlike a
			// seam's own doorway — because (b) is testing that a wall with
			// nothing to peek through blocks contact outright.
			Walls: hexSeamWalls(7, 6, -1),
		},
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
	out, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter")})
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
	out, err := mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter")})
	s.Require().NoError(err)
	s.Equal("fighter", out.Next)
	s.True(out.RoundWrapped)

	s.Equal([]string{"turn-ended", "turn-ended"}, s.storyBeats(mgr, "fighter")[before:])
}

// (f) A round-2+ driven strike reaches the LIVE SUBSCRIBER, not merely the
// log.
//
// rpg-project#254's live walk (the tomb evidence) found skeleton-1's own
// struck beats correctly appended to the story in rounds 2 and 3 — the log
// was right — but never delivered to the live client's stream: the panel
// went straight from "Skeleton's turn." to "Skeleton does nothing." both
// times, though round 1's own moved/missed beats DID arrive live. This gate
// asks the toolkit's own question: does [Manager.EndTurn]'s publish call
// actually hand a round-2 struck beat to the [EventStream] it was given, the
// same as round 1's?
//
// Two full rounds against a FAKE STREAM (not Story) is the point — a
// passing log read proves nothing here, since the evidence already showed
// the log was never the problem.
//
// This gate PASSES today, and pins the two seams a toolkit-side defect
// could hide in, both with an exact assertion rather than a black-box
// pass/fail:
//
//  1. Baseline: round 2's own publish batch must start exactly one seq
//     after round 1's last (asserted below) — openForWrite's baseline is
//     set once per EndTurn call, before that call's own writes, and
//     saveDirty (run inside Strike, before Record) touches neither
//     scope.baseline nor scope.enc, so there is no seam for round 2's
//     baseline to advance past or skip over what round 1 already
//     delivered.
//  2. Recipient: the delivered struck event's Recipient is byte-equal to
//     the exact string Join was called with (asserted below) — the same
//     string a real StreamEvents caller subscribes with
//     (h.broker.Subscribe(session, member), rpg-api's stream_events.go).
//
// Both clean means the mechanism has no seam for a round-2 beat to go
// missing that a round-1 beat would not also hit. The live walk's own
// anomaly is therefore not reproducible as a toolkit defect — this test is
// the negative control that rules the toolkit's own Publish call out, in
// writing, for whoever looks at rpg-api's broker/stream layer (a live,
// no-replay pub/sub — StreamEvents' own doc: "no replay obligation... a
// client resyncs via GetStory") or the live harness next.
func (s *MonsterTurnTestSuite) TestRoundTwoStruckReachesTheLiveSubscriber() {
	ctx := context.Background()
	stream := &fakeStream{}

	// A durable fighter: testDice{}'s flat rolls make every hit's damage a
	// large, repeatable number, and this gate is about DELIVERY, not
	// defeat. High hit points keep two ordinary rounds of being hit from
	// wandering into a defeat-ends-the-fight scene by accident.
	fighter := armedFighter("fighter")
	fighter.HitPoints, fighter.MaxHitPoints = 100, 100

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(fighter),
		Events:     stream,
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 1, Y: 0}, // adjacent: no run-up, every round is a swing
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed)

	// joinMember is the exact string handed to Join — the same string a
	// real StreamEvents caller subscribes with (h.broker.Subscribe(session,
	// member), rpg-api's stream_events.go). The recipient check below
	// compares against this constant, not a literal, so the two can never
	// silently drift apart.
	const joinMember = "fighter"

	skelStruckFighter := func(events []session.Event) (session.Event, bool) {
		for _, e := range events {
			if e.Recipient != joinMember || e.Kind != session.EventStruck {
				continue
			}
			if body, ok := e.Body.(session.StruckBody); ok && body.Attacker == "skel-1" && body.Target == joinMember {
				return e, true
			}
		}
		return session.Event{}, false
	}

	// Round 1: the fighter hands her turn to the skeleton, which swings
	// (testDice{}'s flat 10 clears duelAC 12 — TestSkeletonAttacksFromRange's
	// own math). Baseline: this is the round the live evidence says DID
	// stream correctly. The fighter never swings back — a player's own turn
	// needs no declared action before EndTurn, and skipping it keeps the
	// skeleton's own 13 hit points out of this gate's way entirely (the
	// evidence's own skeleton-1 survived every round it is about).
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter")})
	s.Require().NoError(err)
	_, ok := skelStruckFighter(stream.published)
	s.Require().True(ok, "round 1's own struck beat must reach the live subscriber (control)")

	// (1) Baseline seam, made explicit rather than merely implied by the
	// round 2 assertion below: round 2's own publish batch must pick up
	// EXACTLY where round 1's left off. saveDirty runs inside Strike,
	// before Record — it writes m.characters.SaveCharacter and mutates
	// scope.data.NPCs in place, and touches neither scope.baseline nor
	// scope.enc — so there is no seam here for round 2's own baseline to
	// advance past or skip over what round 1 already delivered. A gap
	// above 0 would mean some beat between the two calls reached nobody.
	s.Require().NotEmpty(stream.published)
	round1Last := stream.published[len(stream.published)-1].Seq

	// Round 2: the same call again. The skeleton's SECOND driven strike is
	// round 2's own struck beat — the one the live evidence says never
	// arrived at the client's stream, though the story log recorded it
	// correctly.
	stream.published = nil // isolate what THIS EndTurn's own publish call delivers
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter")})
	s.Require().NoError(err)
	s.Require().NotEmpty(stream.published)
	round2First := stream.published[0].Seq
	s.Equal(round1Last+1, round2First,
		"round 2's own publish batch starts exactly one seq after round 1's last — no gap, no bypass")

	struck, ok := skelStruckFighter(stream.published)
	s.Require().True(ok,
		"round 2's own struck beat must reach the live subscriber, exactly as round 1's did")

	// (2) Recipient seam, made explicit: the delivered event's Recipient is
	// byte-equal to the string a real subscriber would key off — not
	// merely non-empty or plausible-looking.
	s.Equal(joinMember, struck.Recipient,
		"the struck event's Recipient must be exactly the string Join was called with")
}

// (g) Live delivery and a Story catch-up are the SAME projection of the
// same story-log entries, entry by entry, for a driven monster turn —
// rpg-api-protos#239's ruling (rpg-project#257 slice 3), and the reason
// [projectEntry] exists: one producer, not two mappings that happen to
// agree today. The debug feed found this broken as kind=UNKNOWN body=null
// on every caught-up entry while the same beats arrived live typed
// correctly (moved, missed, ...); a client that notices a gap and
// re-queries Story from FromSeq:1 must see exactly what a live subscriber
// already received.
//
// A driven monster turn is the fixture rather than a single Move, because
// it is the shape #239 was found broken under, and because it exercises
// several kinds at once (joined, bubble-formed, struck-or-missed, turn-ended)
// rather than one kind that might happen to survive both paths
// by accident.
func (s *MonsterTurnTestSuite) TestLiveDeliveryAndStoryCatchUpAreByteEqual() {
	ctx := context.Background()
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(armedFighter("fighter")),
		Events:     stream,
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 4, Y: 0}, // four cells off: attacks from shortbow range (TestSkeletonAttacksFromRange)
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "the skeleton's driven turn is this test's whole point")

	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter")})
	s.Require().NoError(err, "the skeleton's whole turn — strike, end — drives inside this one call")

	// live is everything the fake stream delivered to fighter across the
	// WHOLE scene, join through the driven turn — never reset — so it lines
	// up against Story{FromSeq:1}, which answers from the very first entry
	// fighter's own audience appears in.
	var live []session.Event
	for _, e := range stream.published {
		if e.Recipient == "fighter" {
			live = append(live, e)
		}
	}
	s.Require().NotEmpty(live, "join, spawn and the driven turn must all have addressed fighter")
	foundRichStrike := false
	for _, event := range live {
		if event.Kind != session.EventStruck {
			continue
		}
		body, ok := event.Body.(session.StruckBody)
		s.Require().True(ok, "live struck event carries StruckBody, got %T", event.Body)
		s.Require().NotEmpty(body.DamageComponents,
			"the driven strike must carry the detail whose replay equality this test guards")
		foundRichStrike = true
	}
	s.True(foundRichStrike, "this deterministic driven turn lands a strike")

	caughtUp, err := mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "fighter", FromSeq: 1})
	s.Require().NoError(err)

	s.Equal(live, caughtUp,
		"live delivery and Story{FromSeq:1} catch-up must be the SAME projection, entry by "+
			"entry — kind, typed Body and Tags included — not two mappings that merely agree today")
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
