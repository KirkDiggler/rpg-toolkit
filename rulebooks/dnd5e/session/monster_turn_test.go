// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func TestBehaviorRoundTripsRememberedRoute(t *testing.T) {
	view := session.MonsterView{
		Self: "goblin",
		Remembered: []session.RememberedMember{{
			ID: "billy", Kind: session.KindPlayer,
			DistanceCells: 2,
			Path:          []spatial.Position{{X: 1, Y: 2}, {X: 2, Y: 3}},
		}},
		Budget: session.TurnBudget{MovementFeet: 30},
	}
	intent, err := session.Behavior().Act(view)
	require.NoError(t, err)
	require.Equal(t, session.Move{Path: []spatial.Position{{X: 1, Y: 2}}}, intent)
}

// task6ArrivalFixture starts a real session fight, then arranges a persisted
// held-known sighting whose remembered cell is the skeleton's next step. The
// fighter's live cell is moved out of sight in the persisted encounter so the
// driven arrival must correct the stale testimony rather than refresh it.
func task6ArrivalFixture(t *testing.T) (*session.Manager, *fakeSessions, *fakeEncounters, *fakeStream) {
	t.Helper()
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: sessions, Encounters: encounters,
		Characters: newFakeCharacters(armedFighter("fighter")), Events: stream,
	})
	require.NoError(t, err)
	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(40, 6),
	})
	require.NoError(t, err)
	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 0, Y: 0},
	})
	require.NoError(t, err)
	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 1, Y: 0},
	})
	require.NoError(t, err)
	require.NotNil(t, spawned.Formed)

	data, err := encounters.GetEncounter(ctx, "world")
	require.NoError(t, err)
	oldCell := spatial.Position{X: 0, Y: 0}
	payload, err := encounter.EncodeLocationPayload(encounter.LocationKnowledge{
		State: encounter.LocationKnown, Position: oldCell,
	})
	require.NoError(t, err)
	holdings := data.Intel.Holdings[core.EntityID("skel-1")]
	holding, ok := holdings[intel.Subject("fighter")]
	require.True(t, ok, "fight-time skeleton must hold sight testimony for fighter")
	holding.Payload, holding.CurrentVia = payload, nil
	holdings[intel.Subject("fighter")] = holding
	for i := range data.Members {
		if data.Members[i].ID == encounter.MemberID("fighter") {
			data.Members[i].Cell = &encounter.PositionData{X: 30, Y: 5}
		}
	}
	require.NoError(t, encounters.SaveEncounter(ctx, "world", data))
	stream.published = nil
	return mgr, sessions, encounters, stream
}

func task6StoredLocation(t *testing.T, encounters *fakeEncounters) encounter.LocationKnowledge {
	t.Helper()
	data, err := encounters.GetEncounter(context.Background(), "world")
	require.NoError(t, err)
	holding := data.Intel.Holdings[core.EntityID("skel-1")][intel.Subject("fighter")]
	location, ok := encounter.DecodeLocationPayload(holding.Payload)
	require.True(t, ok, "stored sight testimony must be canonical")
	return location
}

// TestSessionMonsterArrivalSaveFailureRollsBackCorrection proves the session
// verb's save-before-publish law around the real driven monster arrival.
func TestSessionMonsterArrivalSaveFailureRollsBackCorrection(t *testing.T) {
	mgr, sessions, encounters, stream := task6ArrivalFixture(t)
	declarationID := currentEndTurnID(t, mgr, "sess", "fighter")
	errSave := errors.New("encounter store unavailable")
	failing, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: sessions, Encounters: &failingEncounters{fakeEncounters: encounters, saveErr: errSave},
		Characters: newFakeCharacters(armedFighter("fighter")), Events: stream,
	})
	require.NoError(t, err)

	out, err := failing.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "fighter", DeclarationID: declarationID,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, session.ErrSaveFailed)
	require.Nil(t, out)
	location := task6StoredLocation(t, encounters)
	require.Equal(t, encounter.LocationKnown, location.State)
	require.Equal(t, spatial.Position{X: 0, Y: 0}, location.Position)
	require.Empty(t, stream.published, "a failed save publishes no driven-turn events")
}

// TestSessionMonsterArrivalPersistsCorrection proves the success twin through
// the same load-act-save path and exposes only the correction's identities.
func TestSessionMonsterArrivalPersistsCorrection(t *testing.T) {
	mgr, _, encounters, _ := task6ArrivalFixture(t)
	out, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(t, mgr, "sess", "fighter"),
	})
	require.NoError(t, err)
	require.Equal(t, []session.IntelCorrection{{Observer: "skel-1", Subject: "fighter"}}, out.Corrected)
	location := task6StoredLocation(t, encounters)
	require.Equal(t, encounter.LocationUnknown, location.State)
	data, err := encounters.GetEncounter(context.Background(), "world")
	require.NoError(t, err)
	holding := data.Intel.Holdings[core.EntityID("skel-1")][intel.Subject("fighter")]
	require.Empty(t, holding.CurrentVia, "persisted corrected sight holding must remain Held")
}

// TestMalformedSightTestimonyFailsSessionLoadBeforeProjection proves a
// malformed persisted sight payload is rejected by the real session load path
// and never reaches a projected View result.
func TestMalformedSightTestimonyFailsSessionLoadBeforeProjection(t *testing.T) {
	_, sessions, encounters, _ := task6ArrivalFixture(t)
	data, err := encounters.GetEncounter(context.Background(), "world")
	require.NoError(t, err)
	holding := data.Intel.Holdings[core.EntityID("skel-1")][intel.Subject("fighter")]
	holding.Payload = nil
	data.Intel.Holdings[core.EntityID("skel-1")][intel.Subject("fighter")] = holding
	require.NoError(t, encounters.SaveEncounter(context.Background(), "world", data))

	// Use a fresh manager to make this a load-path assertion, not an in-memory
	// object assertion.
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: sessions, Encounters: encounters,
		Characters: newFakeCharacters(armedFighter("fighter")), Events: session.DiscardEvents{},
	})
	require.NoError(t, err)
	out, err := mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "fighter"})
	require.Error(t, err)
	require.ErrorIs(t, err, session.ErrInvalidWorld)
	require.Nil(t, out)
}

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
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
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
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
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

// (a2) A KindWorld member on the roster alongside the skeleton must not
// break the driven monster turn: compileResolutionCast, castFor, and
// boundaryCast all built their cast by checking KindMonster and otherwise
// ASSUMING a character, so a placed world NPC (no sheet at all) used to
// fail the whole EndTurn call with ErrNoCharacter (rpg-toolkit#1493).
func (s *MonsterTurnTestSuite) TestPlacedWorldNPCDoesNotBreakTheMonstersTurn() {
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

	merchant, err := npc.New(npc.Config{
		Ref: refs.NPCs.Merchant(), DisplayName: "Demo Merchant",
		Capabilities: []npc.Capability{npc.CapabilityVendor},
	})
	s.Require().NoError(err)
	_, err = mgr.PlaceNPC(ctx, &session.PlaceNPCInput{
		Session: "sess", Member: "vendor-1", Position: spatial.Position{X: 1, Y: 1}, NPC: merchant.ToData(),
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 4, Y: 0},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "spawning in sight and in range must start the fight on the spot")

	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err, "a placed world NPC on the roster must not break the skeleton's driven turn")
}

// (b) A skeleton behind a wall from the only player never sees them: sight
// is LOS-bounded, not range alone, so spawning it starts no fight — there
// is no monster's turn here to drive, which is the point.
func (s *MonsterTurnTestSuite) TestBlindSkeletonBehindAWallNeverJoinsTheFight() {
	ctx := context.Background()
	mgr := s.tombManager(session.Behavior(), testDice{})

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
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

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
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
	//
	// PER RECIPIENT, since rpg-toolkit#1375: Seq is each member's own
	// delivered numbering, so continuity is asserted member by member —
	// which is the stronger form of the same guarantee, and the form a
	// real client (who holds exactly one member's stream) experiences.
	s.Require().NotEmpty(stream.published)
	round1LastFor := map[string]uint64{}
	for _, e := range stream.published {
		round1LastFor[e.Recipient] = e.Seq
	}

	// Round 2: the same call again. The skeleton's SECOND driven strike is
	// round 2's own struck beat — the one the live evidence says never
	// arrived at the client's stream, though the story log recorded it
	// correctly.
	stream.published = nil // isolate what THIS EndTurn's own publish call delivers
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter")})
	s.Require().NoError(err)
	s.Require().NotEmpty(stream.published)
	round2FirstFor := map[string]uint64{}
	for _, e := range stream.published {
		if _, seen := round2FirstFor[e.Recipient]; !seen {
			round2FirstFor[e.Recipient] = e.Seq
		}
	}
	for member, last := range round1LastFor {
		s.Equal(last+1, round2FirstFor[member],
			"round 2's own publish batch starts exactly one seq after round 1's last "+
				"for %s — no gap, no bypass", member)
	}

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
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
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

// recordingBehavior keeps independent, plain-data copies of every view and
// intent that crosses the session seam. The fixture assertions deliberately
// read these recordings rather than a live encounter, so a concealed member's
// true placement cannot accidentally become test input.
type recordingBehavior struct {
	views   []session.MonsterView
	intents []session.TurnIntent
	next    session.TurnDriver
}

func (r *recordingBehavior) Act(view session.MonsterView) (session.TurnIntent, error) {
	r.views = append(r.views, cloneMonsterView(view))
	intent, err := r.next.Act(view)
	if err != nil {
		return nil, err
	}
	r.intents = append(r.intents, cloneTurnIntent(intent))
	return intent, nil
}

func cloneMonsterView(in session.MonsterView) session.MonsterView {
	out := in
	out.Actions = append([]session.ActionView(nil), in.Actions...)
	out.Seen = make([]session.SeenMember, len(in.Seen))
	for i, member := range in.Seen {
		out.Seen[i] = member
		out.Seen[i].Path = append([]spatial.Position(nil), member.Path...)
		out.Seen[i].InReach = make(map[string]bool, len(member.InReach))
		for ref, reachable := range member.InReach {
			out.Seen[i].InReach[ref] = reachable
		}
	}
	out.Remembered = make([]session.RememberedMember, len(in.Remembered))
	for i, member := range in.Remembered {
		out.Remembered[i] = member
		out.Remembered[i].Path = append([]spatial.Position(nil), member.Path...)
	}
	return out
}

func cloneTurnIntent(in session.TurnIntent) session.TurnIntent {
	switch intent := in.(type) {
	case session.Move:
		intent.Path = append([]spatial.Position(nil), intent.Path...)
		return intent
	case *session.Move:
		if intent == nil {
			return (*session.Move)(nil)
		}
		copy := *intent
		copy.Path = append([]spatial.Position(nil), intent.Path...)
		return &copy
	case session.Attack:
		return intent
	case *session.Attack:
		if intent == nil {
			return (*session.Attack)(nil)
		}
		copy := *intent
		return &copy
	case session.Pass:
		return intent
	case *session.Pass:
		if intent == nil {
			return (*session.Pass)(nil)
		}
		copy := *intent
		return &copy
	default:
		return in
	}
}

func requireRecordedView(t *testing.T, views []session.MonsterView, match func(session.MonsterView) bool) session.MonsterView {
	t.Helper()
	for _, view := range views {
		if match(view) {
			return view
		}
	}
	t.Fatalf("no recorded monster view matched among %d calls", len(views))
	return session.MonsterView{}
}

func seen(view session.MonsterView, subject string) bool {
	for _, member := range view.Seen {
		if member.ID == subject {
			return true
		}
	}
	return false
}

func remembered(view session.MonsterView, subject string) bool {
	for _, member := range view.Remembered {
		if member.ID == subject {
			return true
		}
	}
	return false
}

func rememberedAt(view session.MonsterView, subject string, position spatial.Position) bool {
	for _, member := range view.Remembered {
		if member.ID == subject && member.Position == position {
			return true
		}
	}
	return false
}

func requireNeverContainsPosition(t *testing.T, views []session.MonsterView, subject string, forbidden spatial.Position) {
	t.Helper()
	for i, view := range views {
		for _, member := range view.Seen {
			if member.ID == subject {
				require.NotEqual(t, forbidden, member.Position, "view %d Seen leaked concealed %s position", i, subject)
				for _, position := range member.Path {
					require.NotEqual(t, forbidden, position, "view %d Seen path leaked concealed %s position", i, subject)
				}
			}
		}
		for _, member := range view.Remembered {
			if member.ID == subject {
				require.NotEqual(t, forbidden, member.Position, "view %d Remembered leaked concealed %s position", i, subject)
				for _, position := range member.Path {
					require.NotEqual(t, forbidden, position, "view %d Remembered path leaked concealed %s position", i, subject)
				}
			}
		}
	}
}

func requireNeverContainsIntentPosition(t *testing.T, intents []session.TurnIntent, forbidden spatial.Position) {
	t.Helper()
	for i, intent := range intents {
		move, ok := intent.(session.Move)
		if !ok {
			if pointer, pointerOK := intent.(*session.Move); pointerOK && pointer != nil {
				move, ok = *pointer, true
			}
		}
		if !ok {
			continue
		}
		for _, position := range move.Path {
			require.NotEqual(t, forbidden, position, "intent %d leaked concealed Billy position", i)
		}
	}
}

func persistedMonsterPosition(t *testing.T, repo *fakeEncounters, id string) spatial.Position {
	t.Helper()
	data, err := repo.GetEncounter(context.Background(), "world")
	require.NoError(t, err)
	for _, member := range data.Members {
		if string(member.ID) != id {
			continue
		}
		require.NotNil(t, member.Cell, "persisted %s member must have a cell", id)
		return spatial.Position{X: member.Cell.X, Y: member.Cell.Y}
	}
	t.Fatalf("persisted encounter has no member %q", id)
	return spatial.Position{}
}

func requireUnknownStoredLocation(t *testing.T, repo *fakeEncounters, observer, subject string) {
	t.Helper()
	data, err := repo.GetEncounter(context.Background(), "world")
	require.NoError(t, err)
	holding, ok := data.Intel.Holdings[core.EntityID(observer)][intel.Subject(subject)]
	require.True(t, ok, "persisted %s testimony for %s must remain held", observer, subject)
	require.Empty(t, holding.CurrentVia)
	location, ok := encounter.DecodeLocationPayload(holding.Payload)
	require.True(t, ok, "persisted testimony must remain canonical")
	require.Equal(t, encounter.LocationUnknown, location.State)
}

func requireHeldKnownStoredLocation(
	t *testing.T, repo *fakeEncounters, observer, subject string, want spatial.Position,
) {
	t.Helper()
	data, err := repo.GetEncounter(context.Background(), "world")
	require.NoError(t, err)
	holding, ok := data.Intel.Holdings[core.EntityID(observer)][intel.Subject(subject)]
	require.True(t, ok, "persisted %s testimony for %s must exist", observer, subject)
	require.Empty(t, holding.CurrentVia, "broken sight must leave held testimony")
	location, ok := encounter.DecodeLocationPayload(holding.Payload)
	require.True(t, ok, "persisted testimony must remain canonical")
	require.Equal(t, encounter.LocationKnown, location.State)
	require.Equal(t, want, location.Position)
}

func persistedKnownLocation(t *testing.T, repo *fakeEncounters, observer, subject string) spatial.Position {
	t.Helper()
	data, err := repo.GetEncounter(context.Background(), "world")
	require.NoError(t, err)
	holding, ok := data.Intel.Holdings[core.EntityID(observer)][intel.Subject(subject)]
	require.True(t, ok, "persisted %s testimony for %s must exist", observer, subject)
	require.NotEmpty(t, holding.CurrentVia, "initial testimony must be current")
	location, ok := encounter.DecodeLocationPayload(holding.Payload)
	require.True(t, ok, "persisted testimony must be canonical")
	require.Equal(t, encounter.LocationKnown, location.State)
	return location.Position
}

func doubleDoorWorld(t *testing.T, withDavid bool) *encounter.EncounterData {
	t.Helper()
	walls := append(hexSeamWallsFrom(20, 0, 5, 1), hexSeamWallsFrom(22, 0, 5, -1)...)
	filtered := walls[:0]
	from, to := spatial.Position{X: 21, Y: 1}, spatial.Position{X: 22, Y: 0}
	for _, wall := range walls {
		if wall.From == from && wall.To == to {
			continue
		}
		filtered = append(filtered, wall)
	}
	walls = filtered
	if withDavid {
		// This wall blocks David from the goblin's starting cell, but not from
		// the cell reached by the first remembered-directed move. The next
		// refresh therefore makes David lawfully current-visible without any
		// recorder-side world access or manufactured view data.
		walls = append(walls, encounter.WallInput{Boundary: spatial.Boundary{
			From: spatial.Position{X: 0, Y: 1}, To: spatial.Position{X: 1, Y: 2},
			BlocksMovement: true, BlocksLineOfSight: true,
		}})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("left", 0, 0, 20, 5),
				rectRegion("carpet", 20, 0, 2, 5),
				rectRegion("right", 22, 0, 4, 5),
			},
			Walls: walls,
			Doors: []encounter.DoorInput{
				{ID: "door-a", Edges: []encounter.DoorEdge{{From: hexCell(19, 1), To: hexCell(20, 1)}}, State: encounter.DoorIsOpen()},
				{ID: "door-b", Edges: []encounter.DoorEdge{{From: hexCell(21, 1), To: hexCell(22, 0)}}, State: encounter.DoorIsOpen()},
			},
		},
		Endings:   []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()
	return &data
}

func doubleDoorFixture(t *testing.T, withDavid bool) (*session.Manager, *fakeSessions, *fakeEncounters, *recordingBehavior) {
	t.Helper()
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := []*character.Data{armedFighter("billy")}
	if withDavid {
		characters = append(characters, armedFighter("david"))
	}
	recorder := &recordingBehavior{next: session.Behavior()}
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: recorder, Sessions: sessions, Encounters: encounters,
		Characters: newFakeCharacters(characters...), Events: session.DiscardEvents{},
	})
	require.NoError(t, err)
	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: doubleDoorWorld(t, withDavid)})
	require.NoError(t, err)
	_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "billy", Position: hexCell(21, 1)})
	require.NoError(t, err)
	// Doors answers as one member now (rpg-toolkit#1375), so the fixture's
	// sanity read happens through Billy once he is seated — nothing here is
	// concealed, so his answer IS the whole truth.
	doors, err := mgr.Doors(ctx, &session.DoorsInput{Session: "sess", Member: "billy"})
	require.NoError(t, err)
	require.Equal(t, []session.Door{
		{ID: "door-a", State: "open"},
		{ID: "door-b", State: "open"},
	}, doors.Doors, "the scene must contain two real open session doors")
	if withDavid {
		_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "david", Position: hexCell(2, 3)})
		require.NoError(t, err)
	}
	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "goblin", Ref: refs.Monsters.Goblin().String(), Position: hexCell(0, 1),
	})
	require.NoError(t, err)
	require.NotNil(t, spawned.Formed, "the open first door must make Billy's carpet sight start a fight")
	return mgr, sessions, encounters, recorder
}

func TestSessionDoubleDoorGhostPursuit(t *testing.T) {
	ctx := context.Background()
	mgr, _, repo, recorder := doubleDoorFixture(t, false)
	firstSeen := hexCell(21, 1)
	carpet := hexCell(21, 1)
	hiddenRightCell := hexCell(23, 1)

	// Spawn's real sight refresh persists the first current testimony at the
	// carpet. Read that repository copy before any driven turn so the proof
	// does not confuse the driver's later stale-memory view with first sight.
	require.Equal(t, firstSeen, persistedKnownLocation(t, repo, "goblin", "billy"))

	var err error

	// Billy crosses the intervening carpet and passes behind Door B. The final
	// interior step is what puts the second seam's wall between him and the
	// goblin; no concealed location is supplied to the monster driver.
	_, err = mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "billy", DeclarationID: currentMoveID(t, mgr, "sess", "billy"),
		Path: []spatial.Position{hexCell(22, 0), hexCell(23, 0), hiddenRightCell},
	})
	require.NoError(t, err)
	requireHeldKnownStoredLocation(t, repo, "goblin", "billy", carpet)

	// Drive until the stale carpet is reached and corrected. The bounded loop
	// is only a safety guard; every assertion below is a narrative milestone,
	// not a prescribed turn count.
	finalRememberedMoveAt := -1
	for i := 0; i < 12; i++ {
		viewOffset := len(recorder.views)
		_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
			Session: "sess", Member: "billy", DeclarationID: currentEndTurnID(t, mgr, "sess", "billy"),
		})
		require.NoError(t, err)
		require.Equal(t, len(recorder.views), len(recorder.intents), "each recorded pursuit view must have its delegated intent")
		for call := viewOffset; call < len(recorder.views); call++ {
			move, moving := recorder.intents[call].(session.Move)
			if rememberedAt(recorder.views[call], "billy", carpet) && moving &&
				len(move.Path) == 1 && move.Path[0] == carpet {
				finalRememberedMoveAt = call
			}
		}
		data, loadErr := repo.GetEncounter(ctx, "world")
		require.NoError(t, loadErr)
		location, ok := encounter.DecodeLocationPayload(data.Intel.Holdings[core.EntityID("goblin")][intel.Subject("billy")].Payload)
		require.True(t, ok)
		if location.State == encounter.LocationUnknown {
			break
		}
	}
	ghostView := requireRecordedView(t, recorder.views, func(view session.MonsterView) bool {
		return rememberedAt(view, "billy", carpet) && !seen(view, "billy")
	})
	require.Empty(t, ghostView.Seen, "no current-visible player may explain the remembered pursuit")
	for _, memory := range ghostView.Remembered {
		if memory.ID == "billy" {
			require.NotEmpty(t, memory.Path)
			require.Equal(t, carpet, memory.Path[len(memory.Path)-1], "the remembered path must end on the exact ghost cell")
		}
	}
	require.Equal(t, carpet, persistedMonsterPosition(t, repo, "goblin"))
	requireUnknownStoredLocation(t, repo, "goblin", "billy")
	require.NotEqual(t, -1, finalRememberedMoveAt, "one remembered-directed move must arrive on the exact carpet")
	postArrivalAt := finalRememberedMoveAt + 1
	require.Less(t, postArrivalAt, len(recorder.views), "arrival correction must be followed by a new driver decision")
	postArrival := recorder.views[postArrivalAt]
	require.Equal(t, carpet, postArrival.Position, "the post-correction decision must be recorded from the arrived carpet cell")
	require.False(t, seen(postArrival, "billy"), "the arrived view must not regain concealed Billy")
	require.False(t, remembered(postArrival, "billy"), "the arrived view must not repeat the resolved ghost")
	_, passed := recorder.intents[postArrivalAt].(session.Pass)
	require.True(t, passed, "the immediate post-correction decision must pass instead of pursuing the carpet again")
	requireNeverContainsPosition(t, recorder.views, "billy", hiddenRightCell)
	requireNeverContainsIntentPosition(t, recorder.intents, hiddenRightCell)
}

func TestSessionDoubleDoorVisibleInterruptsGhostPursuit(t *testing.T) {
	ctx := context.Background()
	mgr, _, _, recorder := doubleDoorFixture(t, true)
	carpet := hexCell(21, 1)
	hiddenRightCell := hexCell(23, 1)

	_, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "billy", DeclarationID: currentMoveID(t, mgr, "sess", "billy"),
		Path: []spatial.Position{hexCell(22, 0), hexCell(23, 0), hiddenRightCell},
	})
	require.NoError(t, err)
	viewOffset, intentOffset := len(recorder.views), len(recorder.intents)
	require.Equal(t, viewOffset, intentOffset, "each recorded pre-interruption view must have its delegated intent")
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "billy", DeclarationID: currentEndTurnID(t, mgr, "sess", "billy"),
	})
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(recorder.views), viewOffset+2, "Billy's EndTurn must drive the remembered step and its immediate follow-up")
	require.Equal(t, len(recorder.views), len(recorder.intents), "each recorded interruption view must have its delegated intent")
	before, after := recorder.views[viewOffset], recorder.views[viewOffset+1]
	require.Empty(t, before.Seen, "the first new driver call must have no current-visible player")
	require.True(t, rememberedAt(before, "billy", carpet), "the first new driver call must remember Billy at the carpet")
	firstMove, ok := recorder.intents[intentOffset].(session.Move)
	require.True(t, ok, "the first no-visible-player decision must pursue remembered Billy")
	var billyMemory session.RememberedMember
	for _, member := range before.Remembered {
		if member.ID == "billy" && member.Position == carpet {
			billyMemory = member
			break
		}
	}
	require.NotEmpty(t, billyMemory.Path, "remembered Billy must supply the first delegated path")
	require.Equal(t, billyMemory.Path[:1], firstMove.Path)
	require.False(t, seen(after, "billy"), "concealed Billy must not become current while David interrupts")

	var david session.SeenMember
	for _, member := range after.Seen {
		if member.ID == "david" {
			david = member
			break
		}
	}
	require.NotEmpty(t, david.Path, "newly visible David must have a live path")
	require.NotEqual(t, billyMemory.Path[:1], david.Path[:1], "David's live route must discriminate visible priority from continued ghost pursuit")
	delegatedMove, ok := recorder.intents[intentOffset+1].(session.Move)
	require.True(t, ok, "visible David outside reach must cause a one-cell move")
	require.Equal(t, david.Path[:1], delegatedMove.Path, "current-visible David must interrupt the stale Billy route")

	requireNeverContainsPosition(t, recorder.views, "billy", hiddenRightCell)
	requireNeverContainsIntentPosition(t, recorder.intents, hiddenRightCell)
}
