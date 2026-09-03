// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// FightStartsTestSuite covers what a walk does when the world changes under it:
// a fight starting, an ending firing, or neither.
//
// It was the interrupt spine's suite — a resolution that stopped mid-way,
// persisted as data, and resumed. The thing that stopped those walks was a rule
// this package no longer holds (rpg-toolkit#964 slice 2), and the spine retired
// with its only producer. What survives here is everything that was never about
// suspension: where a walk stops, what it reports, and which aggregates a verb
// writes.
type FightStartsTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func (s *FightStartsTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	s.mgr = managerOver(s.T(), s.sessions, s.encounters)
}

// SetupSubTest gives every s.Run() its own stores. Without it a table would
// share a session across rows, and the second row would fail to start one
// rather than testing what it claims to test.
func (s *FightStartsTestSuite) SetupSubTest() {
	s.SetupTest()
}

func managerOver(t fataler, sessions *fakeSessions, encounters *fakeEncounters) *session.Manager {
	return managerOverRepos(t, sessions, encounters)
}

// managerOverRepos builds a manager over any repositories, so a test can swap
// in one that fails on demand without rebuilding the world it already set up.
func managerOverRepos(
	t fataler, sessions session.SessionRepository, encounters session.EncounterRepository,
) *session.Manager {
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	if err != nil {
		t.Fatalf("building manager: %v", err)
	}
	return mgr
}

// ambushWorld is a hall split by a wall of occluders with a single gap at y=3.
//
// Alice starts at (1,0) with no line of sight to anything; the ogre waits at
// (5,3) behind the wall. Walking north brings her close enough to the gap that
// they see each other — first contact, mid-path, with cells still to go.
//
// She used to start at (1,1), one row further in. Sight became a LANE rather
// than a line (rpg-toolkit#1022, spatial v0.9.1): a viewer leans around
// obstacles now, so from (1,1) she already found the gap at first light and
// the scene opened mid-fight. One row back and the wall does its job again —
// the geometry moved because the rule did, not because the scene changed.
//
// The scene outlived the rule it was built for. It was written when THIS
// package decided that a sighting stops a walk; the composition owns that now
// (rpg-toolkit#964) and the same geometry produces a started fight instead of
// an opened window. A fixture that survives the rule it was built to test is
// a fixture that was describing the world rather than the code.
// ambushWorldWithAliceAt is ambushWorld with alice moved, for tests that ask
// what the wall does from a particular spot rather than what a walk produces.
func ambushWorldWithAliceAt(t fataler, at spatial.Position) *encounter.EncounterData {
	return buildAmbush(t, at)
}

// ambushPath is the four-cell walk north along authored column 1 that meets
// the ogre on cell two — two cells still to go, which is what makes "the
// fight stopped the walk" a claim rather than a coincidence. Authored cells,
// spoken to the verb as the map's own.
func ambushPath() []spatial.Position {
	return []spatial.Position{hexCell(1, 1), hexCell(1, 2), hexCell(1, 3), hexCell(1, 4)}
}

func ambushWorld(t fataler, extra ...encounter.MemberInput) *encounter.EncounterData {
	return buildAmbush(t, spatial.Position{X: 1, Y: 0}, extra...)
}

func buildAmbush(t fataler, alice spatial.Position, extra ...encounter.MemberInput) *encounter.EncounterData {
	occluders := make([]spatial.Position, 0, 7)
	for y := 0; y < 8; y++ {
		if y == 3 {
			continue // the gap
		}
		occluders = append(occluders, spatial.Position{X: 3, Y: float64(y)})
	}

	members := []encounter.MemberInput{
		{ID: "alice", Kind: encounter.KindPlayer, Position: alice},
		{ID: "ogre", Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 3}},
	}
	members = append(members, extra...)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Props:   occludingProps(occluders...),
		},
		Members: members,
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 7, Y: 7},
			}},
		},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building ambush world: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *FightStartsTestSuite) startAmbush(extra ...encounter.MemberInput) {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T(), extra...),
	})
	s.Require().NoError(err)
}

// walkIntoTheAmbush walks the four-cell path that meets the ogre on cell two,
// and returns the output — two cells still to go, which is what makes "the
// fight stopped the walk" a claim rather than a coincidence.
func (s *FightStartsTestSuite) walkIntoTheAmbush() *session.MoveOutput {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: ambushPath(),
	})
	s.Require().NoError(err)
	return out
}

// TestTheFightStopsTheWalk is scene 2 as it actually works now.
//
// Alice asks for three cells and gets two, because the second lined her up with
// the gap and the ogre. What changed with rpg-toolkit#964 is not the scene but
// WHO DECIDED: this package used to read the perception delta and rule that a
// sighting stops a walk. It holds no such rule now. The composition detects the
// contact, starts the fight itself, and the walk stops because the walker is in
// one — reported as news, not as a question.
//
// The walk did not fail: no error, a short Steps, and the fight's order.
func (s *FightStartsTestSuite) TestTheFightStopsTheWalk() {
	s.startAmbush()
	out := s.walkIntoTheAmbush()

	s.Require().Len(out.Steps, 2, "the walk stopped where the fight started")
	s.Equal(hexCell(1, 2), out.Steps[1].Position,
		"where she stopped, not where she was headed")

	s.Require().NotNil(out.Formed, "lining up with the gap started a fight")
	s.Equal([]string{"alice", "ogre"}, out.Formed.Order, "and it has an order")
	s.Empty(out.Formed.Surprised, "through the gap they see each other, so neither is caught unaware")
	s.NotZero(out.Formed.Seq, "the fight starting is a beat of the story")

	// The discovery is still reported — what changed is who acted on it. The
	// SDK reports what the walker saw; it no longer decides what that means.
	s.Require().Len(out.Discovered["alice"].FirstContact, 1, "the sighting is still news")
	s.Equal("ogre", out.Discovered["alice"].FirstContact[0].Subject)
}

// TestTheDiceDecideTheOrder pins that the host's randomness is consulted, that
// it is what decides, and that the same dice always decide the same way.
//
// Every other fixture wires a constant roll, and a constant makes the dice
// invisible: an implementation that ignored the host's Roller entirely and
// sorted by ID would satisfy every order assertion in this file. Here three
// members roll differently and the expected order is the inverse of the
// alphabet, so a passing run proves the roll drives the order AND that each
// roll landed on the member the seam meant it to.
//
// IT RUNS THE SAME FIGHT EIGHT TIMES, and that is the half Copilot's review
// earned. The rulebook's RollForOrder iterates a map, so asking it for the
// whole fight at once assigns the scripted rolls to whoever comes up first.
// A single run catches that only sometimes — measured at 4 kills in 20 against
// the mutant, because Go's iteration of a small map is a rotation rather than a
// fresh permutation and lands on insertion order more often than not. Eight
// independent fights take the same mutant to 20 of 20, and they assert the
// property the fix actually delivers rather than a symptom of it: identical
// dice, identical fight, every time.
func (s *FightStartsTestSuite) TestTheDiceDecideTheOrder() {
	const runs = 8

	// Members are asked alphabetically — aardvark, alice, ogre — so the
	// scripted rolls land 5, 18, 11 in that order and the fight comes back
	// 18, 11, 5.
	for i := 0; i < runs; i++ {
		sessions, encounters := newFakeSessions(), newFakeEncounters()
		dice := &sequenceDice{rolls: []int{5, 18, 11}}
		mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
			Dice: dice, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
			Characters: testCharacters(), Events: session.DiscardEvents{},
		})
		s.Require().NoError(err)
		_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
			Session: "sess", Encounter: "world", World: ambushWorld(s.T(),
				encounter.MemberInput{
					ID: "aardvark", Kind: encounter.KindMonster,
					Position: spatial.Position{X: 5, Y: 3},
				}),
		})
		s.Require().NoError(err)
		s.Require().Zero(dice.next, "run %d: nothing has met anything yet", i)

		out, err := mgr.Move(context.Background(), &session.MoveInput{
			Session: "sess", Member: "alice",
			Path: ambushPath()[:2],
		})
		s.Require().NoError(err)
		s.Require().NotNil(out.Formed, "run %d", i)

		s.Equal(3, dice.next, "run %d: one d20 per member, from the host's dice", i)
		s.Equal([]string{"alice", "ogre", "aardvark"}, out.Formed.Order,
			"run %d: 18, 11, 5 — the dice decide, not the alphabet, and they decide the same way twice", i)
	}
}

// TestAnUnplayedMemberFirstInInitiativeIsAlreadyDrivenPastFightStart is
// rpg-toolkit#1162's other headline case, at the session seam: if the roll
// puts the ogre first, nobody has reached the fight's clock yet to end its
// turn for them — the caller only just learned the fight exists — so the
// drive has to have already happened by the time this Move call returns.
//
// The ogre wins initiative here on the dice alone (1 vs 20), the same
// mechanism TestTheDiceDecideTheOrder pins, so this is not a different fight-
// forming path: it is the ordinary one, with the roll that used to be able to
// strand it.
func (s *FightStartsTestSuite) TestAnUnplayedMemberFirstInInitiativeIsAlreadyDrivenPastFightStart() {
	dice := &sequenceDice{rolls: []int{1, 20}} // alice, then ogre — alphabetical asking order
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: dice, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: ambushPath(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed)
	s.Equal([]string{"ogre", "alice"}, out.Formed.Order, "control: the ogre really did win initiative")

	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal("alice", turn.Active,
		"the ogre's turn was driven through at formation; the caller sees a PLAYED member active "+
			"in the very first read after Formed, with no EndTurn of its own required to reach it")
	s.Equal(1, turn.Round, "driving the ogre through at formation does not itself wrap a round")
}

// TestADiceFailureAbortsTheFight pins that a host whose randomness is down gets
// a refused verb rather than a wrong fight.
//
// It is not automatic. The rulebook's RollForOrder discards its roller's error
// (`roll, _ := roller.Roll(...)`) and would hand back a member who rolled zero
// — an order that looks fine and is not. The seam keeps the error the rulebook
// threw away, and a fight that cannot be ordered does not half-start.
func (s *FightStartsTestSuite) TestADiceFailureAbortsTheFight() {
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: brokenDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)

	_, err = mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: ambushPath()[:2],
	})
	s.Require().Error(err, "the fight could not be ordered, so the verb fails")
	s.ErrorIs(err, errNoRandomness, "and the host's own failure is still matchable")

	// The whole verb, not just the step that met the ogre. Nothing was saved,
	// so the persisted world still has her where she started — the walk did not
	// half-happen with an unordered fight left behind it.
	view, err := mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "ogre"})
	s.Require().NoError(err)
	s.Empty(view, "the ogre never saw her: not one step was persisted")
}

// TestAWalkThatStartsNoFightRunsToTheEnd is the negative control.
//
// Same world, same ogre, a path that never lines up with the gap. Without this,
// every assertion above is equally consistent with "any walk near a monster
// stops", which is not the rule and would be a much worse game.
//
// It is also the control the old rule could not state. Under perception-stops-
// the-walk the natural negative was "she already holds the ogre, so a second
// walk cannot reveal it" — and that scene no longer exists, because a walker
// who has seen the ogre is in a fight with it and does not free-roam at all.
// The property that survives is the one that was always the point: nothing
// stops a walk except something actually happening.
func (s *FightStartsTestSuite) TestAWalkThatStartsNoFightRunsToTheEnd() {
	s.startAmbush()

	// She walks along the near side of the wall, [2,0] then [2,1], staying
	// off the file the gap opens onto. Neither ever holds the other —
	// verified below rather than assumed, because "no fight" would also be
	// the answer if the walk had simply failed.
	//
	// This route used to run to the corner at [0,0]. Lane sight reaches
	// through the gap at a much shallower angle than a single ray did, and the
	// whole column 0 turned out to be in view of it — so the old "quiet"
	// corner is now the loudest cell in the room.
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{hexCell(2, 0), hexCell(2, 1)},
	})
	s.Require().NoError(err)
	s.Nil(out.Formed, "nobody saw anybody, so no fight started")
	s.Len(out.Steps, 2, "so the whole path is walked")
	s.Empty(out.Discovered["alice"].FirstContact, "and there was nothing to see")

	// Both directions on purpose, even though TestSightIsSymmetric proves they
	// must agree. A negative control that leans on another test's conclusion
	// inherits its failure: if symmetry ever broke, this test would go on
	// reporting a quiet walk while only checking the quiet half.
	for _, who := range []string{"alice", "ogre"} {
		seen, verr := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: who})
		s.Require().NoError(verr)
		s.Empty(seen, "%s holds nothing", who)
	}
}

// TestSightIsSymmetric is what TestTheOgreCanSeeWhatAliceCannot became.
//
// That test recorded a defect as evidence and deliberately did not bless it:
// alice stepped to (1,2), a diagonal across the wall from the ogre at (5,3),
// and the ogre held her while she held nothing — a one-sided contact with
// Surprised populated and NO STEALTH ANYWHERE IN THE MODULE. It was not a
// rule, it was the ray: occluders blocked by rasterizing the line between two
// cells, and that rasterization was direction-dependent on a square grid, so
// A→B and B→A were different cells and one of them clipped the wall.
//
// Filed as rpg-toolkit#1022, ruled a BUG by Kirk — line of sight should be
// symmetric by definition, and asymmetry should arrive deliberately with
// #1020's stealth and perception, never by rounding. Fixed in spatial v0.9.1,
// which now pins symmetry as a LAW over fuzzed rooms in every grid family.
//
// So the test flips: same scene, opposite claim. What it proved for this
// module has not changed — the consumer was always right. Surprised populated,
// crossed the boundary and reported correctly the whole time; what was wrong
// was the PRODUCTION of percepts, exactly where the design said a fix belongs.
// holds reports whether these holdings include a live sighting of a named
// subject.
//
// The symmetry claim is about WHO each side can see, not how many things they
// hold: equal counts would still pass if the two were seeing different things,
// and a symmetry test that cannot tell those apart is not one.
func holds(holdings []session.Sighting, subject string) bool {
	for _, h := range holdings {
		if h.Subject == subject {
			return true
		}
	}
	return false
}

func (s *FightStartsTestSuite) TestSightIsSymmetric() {
	ctx := context.Background()

	// Walked one cell at a time, each in its own session, because a fight
	// forming would stop the walk — the question here is what the WALL does at
	// each spot, not what happens after contact.
	contacts := 0
	for _, at := range []spatial.Position{
		{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 2, Y: 3},
	} {
		sessions, encounters := newFakeSessions(), newFakeEncounters()
		mgr := managerOverRepos(s.T(), sessions, encounters)
		_, err := mgr.StartSession(ctx, &session.StartSessionInput{
			Session: "sess", Encounter: "world",
			World: ambushWorldWithAliceAt(s.T(), at),
		})
		s.Require().NoError(err)

		aliceSees, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "alice"})
		s.Require().NoError(err)
		ogreSees, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "ogre"})
		s.Require().NoError(err)

		// Subjects rather than counts. Equal lengths would still pass if the
		// two were seeing DIFFERENT things, and a symmetry claim that cannot
		// tell those apart is not a symmetry claim.
		aliceHoldsOgre, ogreHoldsAlice := holds(aliceSees, "ogre"), holds(ogreSees, "alice")
		s.Equal(aliceHoldsOgre, ogreHoldsAlice,
			"at %v the wall must do the same thing to both of them", at)
		if aliceHoldsOgre {
			contacts++
		}
	}

	s.Require().Positive(contacts,
		"the fixture must actually put them in sight somewhere, or this proves nothing")
	s.Require().Less(contacts, 5,
		"and must block them somewhere, or the wall is not in the way")
}

// TestAFightOnTheFinalCellIsStillReported is the inverted twin of a rule that
// retired with the walk's own.
//
// The old machine deliberately said NOTHING when the last cell of a path
// revealed something: a window offering "continue or stop" with no cells left
// was a choice between two identical outcomes, so it was suppressed. Reporting
// news has no such problem — a fight that started on the final cell is exactly
// as real as one that started on the second, and a caller who is told nothing
// discovers it from the next verb's refusal instead.
//
// So the suppression is gone, and this pins its absence.
func (s *FightStartsTestSuite) TestAFightOnTheFinalCellIsStillReported() {
	s.startAmbush()

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: ambushPath()[:2], // contact lands on the last cell
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 2, "the walk finished")
	s.Require().NotNil(out.Formed, "and the fight it ended in is reported anyway")
	s.Equal([]string{"alice", "ogre"}, out.Formed.Order)

	seen, err := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Len(seen, 1, "she did see the ogre")
	s.Equal("ogre", seen[0].Subject)
}

// TestTheFightsOrderIsAFunctionOfPersistedData pins C8 at this seam.
//
// Two monsters met by the same step must reach the roller — and come back to
// the caller — in an order derived from stored data rather than from map
// iteration. A session reloaded and re-run has to produce the same fight; an
// order that varied would break "identical inputs yield identical outputs"
// only under load, only in production.
//
// The sort itself moved down with the rule: this package used to sort the
// sighted subjects it enumerated, and the composition now sorts the sides it
// puts in contact. What is pinned HERE is that the SDK passes that order
// through intact — it neither reorders nor drops a participant on the way out.
func (s *FightStartsTestSuite) TestTheFightsOrderIsAFunctionOfPersistedData() {
	// "aardvark" sorts before "ogre"; declared after it, so insertion order and
	// sorted order disagree.
	s.startAmbush(encounter.MemberInput{
		ID: "aardvark", Kind: encounter.KindMonster,
		Position: spatial.Position{X: 5, Y: 3},
	})

	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Formed)
	s.Equal([]string{"aardvark", "alice", "ogre"}, out.Formed.Order,
		"every participant, in an order that does not depend on iteration")
}

// TestAWalkWritesWhatAWalkChanges pins write proportionality — in the shape
// per-recipient numbering gave it (rpg-toolkit#1375). A walk appends beats,
// appended beats advance every member's delivered-stream cursor, and the
// cursors ride SessionData (stream.go) — so a beat-appending verb now writes
// BOTH aggregates, encounter first (persist's own ordering argument). What
// survives of the old rule is its principle: a verb that changes nothing
// writes nothing extra — a verb that appends no beat leaves the cursors
// untouched and the session blob alone (cursorsEqual's early-out).
func (s *FightStartsTestSuite) TestAWalkWritesWhatAWalkChanges() {
	s.startAmbush()

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 0}},
	})
	s.Require().NoError(err)
	s.Equal([]string{"encounter:world", "session:sess"}, out.Saved.Written,
		"the walk's beats advance the stream cursors, and the cursors are session state")
}

// TestAPartialSaveTellsTheCallerWhichHalfLanded is S6 reaching a caller, which
// is not the same as S6 being computed.
//
// A verb returns no output when it returns an error, so a report that only
// exists inside persist is a report nobody can act on. The caller has to be
// able to tell a half failure from a total one — the first is a repair, the
// second a retry.
//
// It used to be driven by a walk that suspended: the world landed, the window
// it owed did not. Nothing owes a window now, so the pin moved to the verb that
// still writes both aggregates — Spawn puts the NPC's sheet in the session
// record. Same failure, same report, a producer that exists.
func (s *FightStartsTestSuite) TestAPartialSaveTellsTheCallerWhichHalfLanded() {
	s.startAmbush()

	// Arm the session store to fail only now, after the world is in place.
	sessions := &failingSessions{fakeSessions: s.sessions, saveErr: errBroken}
	mgr := managerOverRepos(s.T(), sessions, s.encounters)

	_, err := mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: hexCell(6, 6),
	})
	s.Require().Error(err, "the spawned sheet had to be written to the session")
	s.Require().ErrorIs(err, session.ErrSaveFailed, "the condition is matchable")
	s.ErrorIs(err, errBroken, "and so is the store's own failure")

	var saved *session.SaveError
	s.Require().ErrorAs(err, &saved, "the report must survive the error, not die in persist")
	s.Equal([]string{"encounter:world"}, saved.Report.Written,
		"the world landed — the skeleton is really standing there")
	s.Equal([]string{"session:sess"}, saved.Report.Failed,
		"its sheet did not — this is a repair, not a retry")
}

// TestATotalSaveFailureNamesOnlyWhatWasAttempted is the contrast that gives the
// test above its meaning: a report naming one failure and nothing written is a
// different situation from one naming a success and a failure.
func (s *FightStartsTestSuite) TestATotalSaveFailureNamesOnlyWhatWasAttempted() {
	s.startAmbush()

	encounters := &failingEncounters{fakeEncounters: s.encounters, saveErr: errBroken}
	mgr := managerOverRepos(s.T(), s.sessions, encounters)

	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: ambushPath()[:1],
	})
	s.Require().Error(err)

	var saved *session.SaveError
	s.Require().ErrorAs(err, &saved)
	s.Empty(saved.Report.Written, "nothing landed")
	s.Equal([]string{"encounter:world"}, saved.Report.Failed)
}

func TestFightStartsSuite(t *testing.T) {
	suite.Run(t, new(FightStartsTestSuite))
}
