// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// killingblow_test.go is the death lane's coda (rpg-toolkit#1083): the swing
// notices its own kill.
//
// D1 taught the world to notice who is down and D2 taught a fight to end when a
// side runs out — both from the consult that hangs off a sight refresh. Record
// does not refresh sight, so the one verb that WRITES the blow was the one verb
// that could not see what it did: a party cleared the room, stood still, and was
// in a fight with a corpse until somebody walked.
//
// The consult now runs in Record too, at the same one place noticing happens
// (noticeDown). Nothing below asks for that, and nothing below mentions a hit
// point — the rulebook is a fake driven by hand, exactly as D1 and D2 drive it.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type KillingBlowSuite struct {
	deathScene
}

func TestKillingBlowSuite(t *testing.T) {
	suite.Run(t, new(KillingBlowSuite))
}

// blow records one landed strike from alice at the goblin.
//
// The values are the ordinary ones a rulebook hands over. What they are does not
// matter to a single assertion here — the composition never reads them, and the
// death being noticed comes from the capability rather than from the amount.
func (s *KillingBlowSuite) blow(enc *encounter.Encounter, target encounter.MemberID) (uint64, error) {
	s.T().Helper()

	out, err := enc.Record(&encounter.RecordInput{
		Kind:    encounter.OutcomeStruck,
		Actor:   alice,
		Targets: []encounter.MemberID{target},
		Values:  map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	if err != nil {
		return 0, err
	}

	return out.Seq, nil
}

// apart is one room split by a solid wall, with alice and the goblin on opposite
// sides of it and no fight anywhere.
//
// Free roam is the state the open scenes cannot be in: any co-located pair forms
// a bubble at first light, so a wall is the only way to record an outcome
// between two members who are not fighting.
func (s *KillingBlowSuite) apart(standing encounter.Standing) *encounter.Encounter {
	s.T().Helper()

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Sight: everyoneSeesTheWholeMap{}, Standing: standing,
		Retention: encounter.RetentionUnbounded,
		Field:     encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{rectRegion(cryptID, 0, 0, 12, 12)}, Props: wallRow(6, 0, 11)},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, alice), "the wall keeps this scene quiet")

	return enc
}

// countBeats reports how many of an audience's beats are of the given kind.
func (s *KillingBlowSuite) countBeats(enc *encounter.Encounter, audience encounter.MemberID, kind string) int {
	s.T().Helper()

	n := 0
	for _, k := range s.beatKindsOf(enc, audience) {
		if k == kind {
			n++
		}
	}

	return n
}

// --- the swing notices ------------------------------------------------------

// TestTheKillingBlowNoticesItsOwnKill is the coda, whole.
//
// The blow that drops the last monster is recorded, and the fight is over when
// Record returns. Nobody walked, nobody ticked, nobody called Dissolve — the
// party can stand perfectly still and the world still knows what they just did.
func (s *KillingBlowSuite) TestTheKillingBlowNoticesItsOwnKill() {
	down := &downList{}
	enc := s.pair(down)
	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, alice), "control: a fight is running")

	down.down = []encounter.MemberID{goblin}
	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice),
		"the fight ended inside the verb that ended it")
	s.Empty(enc.ToData().Bubbles, "a bubble exists only while a fight does")
}

// TestTheBeatOrderIsCauseThenEffect is the law this slice had to hold to be
// allowed to exist: A VERB'S OWN BEAT PRECEDES ANY BEAT ITS CONSEQUENCES APPEND.
//
// The strike is the cause, the body is what it caused, and the ending is what
// the body caused. All three land in one Record, in that order — which is the
// whole gain over waiting for the next walk, where the strike and the death it
// caused were separated by however long the party stood around.
func (s *KillingBlowSuite) TestTheBeatOrderIsCauseThenEffect() {
	down := &downList{}
	enc := s.pair(down)

	down.down = []encounter.MemberID{goblin}
	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	s.Equal([]string{"scene-opened", "bubble-formed", "struck", "down", "bubble-dissolved"},
		s.beatKindsOf(enc, alice),
		"the blow, then the body, then the ending the body explains")
}

// TestTheRecordedSeqIsTheOutcomeBeatNotTheLastOne pins which beat the caller is
// told about, and the distinction now has teeth.
//
// Record appends up to three beats in a pass, and RecordOutput.Seq names ONE of
// them: the outcome the caller handed over. A seam that echoed the last sequence
// written would hand a client the ending's number and call it the strike's.
func (s *KillingBlowSuite) TestTheRecordedSeqIsTheOutcomeBeatNotTheLastOne() {
	down := &downList{}
	enc := s.pair(down)

	down.down = []encounter.MemberID{goblin}
	seq, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)

	var named map[string]any
	for i, entry := range story {
		if entry.Seq == seq {
			named = s.beatsOf(enc, alice)[i]
		}
	}
	s.Require().NotNil(named, "the reported sequence is in the story")
	s.Equal("struck", named["beat"], "and it is the beat the caller recorded")
	s.Less(seq, story[len(story)-1].Seq, "with the consequences after it")
}

// TestASideStillStandingKeepsFighting is the rule's other half, and the one a
// naive implementation gets wrong: the trigger is a SIDE being gone, not a body
// being noticed. Drop one of two monsters and the fight goes on without it.
func (s *KillingBlowSuite) TestASideStillStandingKeepsFighting() {
	down := &downList{}
	enc := s.trio(down)
	s.Require().Equal([]encounter.MemberID{alice, goblin, wolf}, s.orderOf(enc, alice))

	down.down = []encounter.MemberID{goblin}
	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice), "the wolf is still standing")
	s.Equal([]encounter.MemberID{alice, wolf}, s.orderOf(enc, alice),
		"and the order closed over the body, mid-round")
	s.Equal([]string{"scene-opened", "bubble-formed", "struck", "down", "transferred"},
		s.beatKindsOf(enc, alice))
}

// --- what happens to the clock the recording swing was on -------------------

// TestTheSpliceLeavesTheRoundWhereItWas pins the turn state a mid-round removal
// leaves behind, DERIVED from the clock rather than assumed.
//
// A body leaving the order is the straggler's own machinery, and the round must
// not restart because somebody fell — a fight where killing a monster rewound
// the round would hand the party a free turn every time they landed a blow.
func (s *KillingBlowSuite) TestTheSpliceLeavesTheRoundWhereItWas() {
	down := &downList{}
	enc := s.trio(down)

	before, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(alice, before.Active, "control: it is her turn")

	down.down = []encounter.MemberID{goblin}
	_, err = s.blow(enc, goblin)
	s.Require().NoError(err)

	after, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal([]encounter.MemberID{alice, wolf}, after.Order,
		"control: the splice really happened, so what follows is about the splice")
	s.Equal(before.Round, after.Round, "the round is where it was")
	s.Equal(alice, after.Active, "and the turn is still hers")
}

// TestABodyRemovedOnItsOwnTurnLeavesSomebodyActive is the same removal at the
// one moment it USED TO be able to strand a fight: the member spliced out is
// the member whose turn it is.
//
// rpg-toolkit#1162 changed what "the member whose turn it is" can even BE.
// Before it, a monster could hold the active slot indefinitely — nothing
// drove its turn forward — so a body dying while active for a monster was a
// real, reachable state, and this test used to reach it with the trio
// fixture's single player. It no longer can: EndTurn now drives any unplayed
// member through before returning (ADR-0043), so Active is never a monster
// at rest between calls. What CAN still happen — and is the case this test
// now proves — is the splice itself handing the active slot to an unplayed
// member: a PLAYER dies while active, a monster is next in the (now
// shortened) order, and driveIfStillRunning has to catch that or the fight
// stalls one call later than it used to.
func (s *KillingBlowSuite) TestABodyRemovedOnItsOwnTurnLeavesSomebodyActive() {
	down := &downList{}
	// Two players either side of a monster apiece, so removing the SECOND
	// player while active hands the slot to a monster rather than wrapping
	// straight back to a player — the case driveIfStillRunning exists for.
	enc := s.scene(down,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 0}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 0}},
		encounter.MemberInput{ID: wolf, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 0}},
	)
	// Trigger detection sorts the roster for determinism rather than keeping
	// authoring order, so the actual order is read back rather than assumed.
	order := s.orderOf(enc, alice)
	s.Require().Equal([]encounter.MemberID{alice, bob, goblin, wolf}, order,
		"pinning the order this test's reasoning below depends on")

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	before, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(bob, before.Active, "control: bob is up, no monster between alice and him")

	down.down = []encounter.MemberID{bob}
	_, err = s.blow(enc, bob)
	s.Require().NoError(err)

	after, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal([]encounter.MemberID{alice, goblin, wolf}, after.Order)
	s.Equal(alice, after.Active,
		"the wolf inherited the slot bob left, has no player, and was driven straight through — "+
			"wrapping the round back to alice rather than stranding the fight on it")
	s.Equal(before.Round+1, after.Round, "the wolf's driven-through pass closed the round")

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: after.Active})
	s.Require().NoError(err, "and the fight can still be advanced")
}

// TestTheEndingReHomesEverybodyTheFightHeld is the mid-turn dissolve seen from
// the members' side. R6 says every member is on exactly one clock, and a fight
// that ended inside a Record must not leave anybody between two.
func (s *KillingBlowSuite) TestTheEndingReHomesEverybodyTheFightHeld() {
	down := &downList{}
	enc := s.pair(down)

	down.down = []encounter.MemberID{goblin}
	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	for _, id := range []encounter.MemberID{alice, goblin} {
		s.Equal(encounter.ClockWorld, s.clockOf(enc, id), "%q is home, not on no clock at all", id)
	}

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().ErrorIs(err, encounter.ErrNoBubble, "there is no turn left to end")
}

// --- said once ---------------------------------------------------------------

// TestTheSameDeathIsNotNarratedTwice is D1's story-ledger dedup holding across
// the new call site, which is the thing that makes adding one SAFE.
//
// The consult now runs from two families of verb rather than one, so the same
// body is asked about by the swing that made it and again by the next step
// anybody takes. A client that heard about the death twice would render it
// twice.
func (s *KillingBlowSuite) TestTheSameDeathIsNotNarratedTwice() {
	down := &downList{}
	enc := s.pair(down)

	down.down = []encounter.MemberID{goblin}
	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	// The fight is over, so the walk is hers — and the walk consults again.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: spatial.Position{X: 1, Y: 2}})
	s.Require().NoError(err)

	s.Equal(1, s.countBeats(enc, alice, "down"), "one body, told once")
	s.Equal(1, s.countBeats(enc, alice, "bubble-dissolved"), "and one ending")
}

// TestRecordingAgainAfterTheDeathAddsOnlyTheOutcome is the same guard from the
// caller's side: the killing blow stays recordable (ruled fork (a)), and a
// second beat about a body already known does not re-narrate the death.
func (s *KillingBlowSuite) TestRecordingAgainAfterTheDeathAddsOnlyTheOutcome() {
	down := &downList{}
	enc := s.pair(down)

	down.down = []encounter.MemberID{goblin}
	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	_, err = s.blow(enc, goblin)
	s.Require().NoError(err, "the killing blow is recordable against a body")

	s.Equal([]string{"scene-opened", "bubble-formed", "struck", "down", "bubble-dissolved", "struck"},
		s.beatKindsOf(enc, alice))
}

// --- and it invents nothing --------------------------------------------------

// TestARecordWithNobodyDownAppendsOneBeat is the negative control every
// assertion above leans on. The consult runs on EVERY Record, in a fight or out
// of one, and a Record in a quiet world must look exactly as it did before this
// slice.
func (s *KillingBlowSuite) TestARecordWithNobodyDownAppendsOneBeat() {
	enc := s.apart(&downList{})

	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	s.Equal([]string{"scene-opened", "struck"}, s.beatKindsOf(enc, alice),
		"one outcome in, one beat out")
}

// TestARecordOutsideAFightNoticesWithoutInventingAFight is the free-roam case,
// which the new call site reaches for the first time: Record is legal against a
// member who is in no bubble at all.
//
// The news is still news — a body is a body wherever it falls — but there is no
// fight to end, no order to splice, and nothing here may invent either.
func (s *KillingBlowSuite) TestARecordOutsideAFightNoticesWithoutInventingAFight() {
	down := &downList{}
	enc := s.apart(down)

	down.down = []encounter.MemberID{goblin}
	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	s.Equal([]string{"scene-opened", "struck", "down"}, s.beatKindsOf(enc, alice),
		"the body is news; the fight it was not in is not")
	s.Equal(encounter.ClockWorld, s.clockOf(enc, goblin), "it was already home")
}

// TestRecordStillMovesNoClock is the half of "records and does nothing else"
// that did NOT change, pinned because the doc around it did.
//
// A notice is not a tick. Every beat a Record writes carries the same clock
// reading as the outcome that caused it, so a client reading the story sees one
// moment rather than a fight aging by a beat every time somebody swings.
func (s *KillingBlowSuite) TestRecordStillMovesNoClock() {
	down := &downList{}
	enc := s.pair(down)

	down.down = []encounter.MemberID{goblin}
	_, err := s.blow(enc, goblin)
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	s.Require().Len(story, 5)

	at := story[2].At // the struck beat
	for _, entry := range story[2:] {
		s.Equal(at, entry.At, "beat at seq %d moved the clock", entry.Seq)
	}
}

// --- the refusals both survive ----------------------------------------------

// TestACallerStillCannotPushADownBeat is the refusal this slice was most likely
// to erode, so it is re-pinned where the change is.
//
// Record now NOTICES a death, which is the composition asking the rulebook. That
// is not the same thing as a caller declaring one, and the difference is the
// whole reason OutcomeDown is refused: a caller who could push the beat would be
// a second system deciding a fact the world already owns, and it would always
// win, because it reaches the fact first.
func (s *KillingBlowSuite) TestACallerStillCannotPushADownBeat() {
	enc := s.pair(&downList{down: []encounter.MemberID{goblin}})

	_, err := enc.Record(&encounter.RecordInput{Kind: encounter.OutcomeDown, Actor: alice})

	s.Require().ErrorIs(err, encounter.ErrInvalidData,
		"noticing a death is the composition asking; pushing one is still refused")
}

// TestTheRefusalsRunBeforeTheConsult keeps R5's promise where it is cheapest to
// lose it: a verb rejected for its INPUT must not have asked the rulebook
// anything, or a mis-typed kind would cost a repository read per member.
func (s *KillingBlowSuite) TestTheRefusalsRunBeforeTheConsult() {
	counted := &countingStanding{}
	enc := s.pair(counted)
	asked := counted.calls

	_, err := enc.Record(&encounter.RecordInput{Kind: encounter.OutcomeStruck, Actor: "nobody"})
	s.Require().ErrorIs(err, encounter.ErrNoMember)

	s.Equal(asked, counted.calls, "a rejected verb asked nothing")
}

// --- the consult's own failures ---------------------------------------------

// TestAStandingErrorAbortsTheRecording is the capability's contract reaching the
// one verb that used never to consult it.
//
// A world that cannot find out who is standing does not half-act on a guess. The
// error comes back and the caller's obligation is doc.go's — drop the encounter
// unsaved — so nothing the consult would have done is in the world that gets
// persisted.
func (s *KillingBlowSuite) TestAStandingErrorAbortsTheRecording() {
	rulebook := &brokenWhenTold{}
	enc := s.pair(rulebook)

	rulebook.broken = true
	_, err := s.blow(enc, goblin)

	s.Require().ErrorIs(err, errRulebookUnreachable)
	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice), "the fight is untouched")
	s.Zero(s.countBeats(enc, alice, "down"), "and nothing was noticed on the way to the refusal")
	s.Zero(s.countBeats(enc, alice, "bubble-dissolved"))
}

// TestAStrangerInTheAnswerAbortsTheRecording is the other half of the
// capability's contract: an answer naming somebody who is not a member is a
// rulebook defect, refused rather than ignored, and now refusable from Record.
func (s *KillingBlowSuite) TestAStrangerInTheAnswerAbortsTheRecording() {
	rulebook := &strangerWhenTold{}
	enc := s.pair(rulebook)

	rulebook.lying = true
	_, err := s.blow(enc, goblin)

	s.Require().ErrorIs(err, encounter.ErrNotMember)
	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice), "the fight is untouched")
}
