// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RetentionTestSuite covers the story-log retention window (#937): the log is
// bounded so an encounter's blob stops growing without limit, and a Story
// resume point below the retained floor is REJECTED rather than partially
// answered.
//
// The distinction those two halves draw is the whole feature. Trimming alone
// would be a silent data-loss bug for any caller resuming from a sequence;
// rejecting alone would be an outage for the most common call in the system
// (AfterSeq == 0, "I have nothing, send what you have"). Both, together, are a
// reconnect protocol.
//
// WHEN the trim runs is part of the contract (#1381): retention enforces at
// the storage boundary — ToData — never per append, so a verb's own beats
// all survive the verb that minted them and the live Story serves a whole
// verb's delta regardless of size. Tests that exercise trimming therefore
// cross the boundary explicitly with ToData; a test that generates beats and
// never saves is asserting on the UNTRIMMED live log, deliberately.
type RetentionTestSuite struct {
	suite.Suite
}

// walkingEncounter builds a one-room encounter whose only inhabitant can be
// moved back and forth to generate story beats on demand.
//
// The room is 5x5 square with p1 at (1,1); beatsFor below walks between (1,1)
// and (2,1), both permanently in bounds, so beat generation never collides
// with the ending at (4,4).
func (s *RetentionTestSuite) walkingEncounter(retention int) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{rectRegion("r1", 0, 0, 5, 5)}},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "done", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 4, Y: 4},
			}},
		},
		Retention: retention,
	})
	s.Require().NoError(err)
	return enc
}

// generateBeats moves p1 back and forth n times, producing exactly n movement
// beats. Setup has already produced one scene-opened beat, so the log holds
// n+1 beats' worth of sequence numbers afterwards.
func (s *RetentionTestSuite) generateBeats(enc *encounter.Encounter, n int) {
	for i := 0; i < n; i++ {
		to := spatial.Position{X: 2, Y: 1}
		if i%2 == 1 {
			to = spatial.Position{X: 1, Y: 1}
		}
		_, err := enc.Step(&encounter.StepInput{Member: "p1", To: to})
		s.Require().NoError(err, "beat %d", i)
	}
}

// storyLen returns how many entries p1's story currently holds from the
// beginning — the caller-visible size of the live log, which matches the
// retained window only after a save has crossed the storage boundary.
func (s *RetentionTestSuite) storyLen(enc *encounter.Encounter) int {
	entries, err := enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 0})
	s.Require().NoError(err)
	return len(entries)
}

// TestTrimsToExactlyTheWindow is the retention pin. It asserts the retained
// count NUMERICALLY rather than "fewer than we appended", because a check that
// only proves the log shrank cannot distinguish a correct window from an
// off-by-one or from a policy that keeps a wildly different amount.
//
// It also pins WHEN (#1381): before the save, every beat ever appended is
// still readable — the live log does not trim — and it is ToData, the
// storage boundary, that cuts both the emitted blob and the live log to
// exactly the window.
//
// Mutation (#937 pin discipline): change enforceRetention's floor to
// `nextSeq - window - 1` (retaining one extra) and this test fails on the
// count while every other test in the package still passes.
func (s *RetentionTestSuite) TestTrimsToExactlyTheWindow() {
	const window = 8
	enc := s.walkingEncounter(window)

	// 40 movement beats plus setup's scene-opened beat = 41 sequences
	// assigned, comfortably past the window.
	s.generateBeats(enc, 40)

	s.Equal(41, s.storyLen(enc),
		"the live log holds everything appended since load; nothing trims per append")

	data := enc.ToData()
	s.Len(data.Log.Entries, window,
		"the blob holds exactly the window: retention is a fact about what is written")
	s.Equal(window, s.storyLen(enc),
		"retention window must be enforced exactly, not approximately")
}

// TestLogAtExactlyTheWindowIsUntouched pins the boundary below which nothing is
// dropped: a log holding exactly the window is full, not over, and every
// sequence it ever issued must still be servable.
//
// This is the must-accept row for the trim guard itself. An off-by-one that
// began trimming one beat early would still satisfy every "did it shrink"
// assertion in this suite while quietly losing the oldest beat of every log
// that merely reached its limit.
func (s *RetentionTestSuite) TestLogAtExactlyTheWindowIsUntouched() {
	const window = 8
	enc := s.walkingEncounter(window)

	// Setup contributes one scene-opened beat, so seven moves bring the log to
	// exactly the window. The save is what would trim (#1381), so the guard
	// must be proven AT the boundary, not on a log that never crossed it.
	s.generateBeats(enc, window-1)
	enc.ToData()

	s.Equal(window, s.storyLen(enc), "a full log is not an over-full one")

	entries, err := enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 1})
	s.Require().NoError(err, "the very first beat must still be servable at the boundary")
	s.Len(entries, window, "and nothing may have been dropped")
	s.Equal(uint64(1), entries[0].Seq)
}

// TestAfterSeqIsInclusive pins the semantics the field's name gets wrong.
//
// AfterSeq is passed straight through as record.SliceFor's FromSeq, which is an
// inclusive lower bound: asking for 3 yields the entry AT 3. The name says
// otherwise, and a well-meaning future change that "fixed" the behaviour to
// match the name would silently drop one entry from every resume — so the
// behaviour is pinned here and the name is documented as the misnomer it is
// (Copilot, PR #939).
func (s *RetentionTestSuite) TestAfterSeqIsInclusive() {
	enc := s.walkingEncounter(encounter.RetentionUnbounded)
	s.generateBeats(enc, 5)

	entries, err := enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 3})
	s.Require().NoError(err)
	s.Require().NotEmpty(entries)
	s.Equal(uint64(3), entries[0].Seq,
		"AfterSeq is an inclusive lower bound despite its name: to resume after N, pass N+1")
}

// TestSeqSurvivesTrimming is the reconnect contract. Trimming drops entries; it
// must never renumber the survivors, or every client's stored resume point
// silently becomes wrong.
func (s *RetentionTestSuite) TestSeqSurvivesTrimming() {
	const window = 8
	enc := s.walkingEncounter(window)
	s.generateBeats(enc, 40)
	enc.ToData() // the storage boundary is where the trim happens (#1381)

	entries, err := enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 0})
	s.Require().NoError(err)
	s.Require().Len(entries, window)

	// 41 sequences assigned (1..41), retaining the last 8 => 34..41.
	s.Equal(uint64(34), entries[0].Seq, "oldest retained Seq is the floor, unrenumbered")
	s.Equal(uint64(41), entries[len(entries)-1].Seq, "newest retained Seq is unchanged")
	for i := 1; i < len(entries); i++ {
		s.Equal(entries[i-1].Seq+1, entries[i].Seq, "retained sequences stay gapless")
	}
}

// TestZeroIsAlwaysAnswerable pins the exemption that makes trimming safe. A
// caller passing zero holds nothing and is asking for whatever survives; that
// is the first-load path and it must never fail, however much has been trimmed.
//
// Without this exemption the feature would break the single most common call
// in the system the moment the first beat aged out.
func (s *RetentionTestSuite) TestZeroIsAlwaysAnswerable() {
	enc := s.walkingEncounter(4)
	s.generateBeats(enc, 40)
	enc.ToData() // the storage boundary is where the trim happens (#1381)

	entries, err := enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 0})
	s.Require().NoError(err, "AfterSeq 0 must remain answerable after heavy trimming")
	s.Len(entries, 4)
}

// TestResumeBelowFloorIsRejected is the other half of the protocol: a caller
// asserting "I already hold everything below N" must be told when that claim
// can no longer be honoured, rather than handed a short answer that looks
// complete.
func (s *RetentionTestSuite) TestResumeBelowFloorIsRejected() {
	const window = 8
	enc := s.walkingEncounter(window)
	s.generateBeats(enc, 40)
	enc.ToData() // the storage boundary is where the trim happens (#1381)

	// Floor is 34; anything below it is gone.
	_, err := enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 33})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrTrimmed)

	_, err = enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 1})
	s.ErrorIs(err, encounter.ErrTrimmed, "the original first beat is long gone")
}

// TestResumeAtFloorSucceeds is the must-accept row guarding against
// over-tightening. A rejection table proves the check rejects; only a positive
// control at the exact boundary proves it does not over-reach and reject a
// resume point the log can still serve.
//
// Mutation: weaken the guard to `AfterSeq <= e.logFloor` and this test fails
// while every rejection above still passes — which is precisely the failure
// mode a rejection-only suite cannot see.
func (s *RetentionTestSuite) TestResumeAtFloorSucceeds() {
	const window = 8
	enc := s.walkingEncounter(window)
	s.generateBeats(enc, 40)
	enc.ToData() // the storage boundary is where the trim happens (#1381)

	entries, err := enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 34})
	s.Require().NoError(err, "the floor itself is retained and must be servable")
	s.Len(entries, window)
}

// TestUnboundedKeepsEverything pins the opt-out that verified-transcript scenes
// rely on: they assert on the story, not on the policy.
func (s *RetentionTestSuite) TestUnboundedKeepsEverything() {
	enc := s.walkingEncounter(encounter.RetentionUnbounded)
	s.generateBeats(enc, 40)
	enc.ToData() // even the storage boundary trims nothing when unbounded

	s.Equal(41, s.storyLen(enc), "unbounded retains every beat ever appended")

	_, err := enc.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 1})
	s.NoError(err, "nothing was trimmed, so no resume point can be below the floor")
}

// TestDefaultAppliesWhenUnset pins that the zero value selects the package
// default rather than accidentally meaning "unbounded" — the failure mode that
// would silently restore the unbounded growth this issue exists to remove.
func (s *RetentionTestSuite) TestDefaultAppliesWhenUnset() {
	enc := s.walkingEncounter(0)
	s.generateBeats(enc, encounter.DefaultRetention+20)
	enc.ToData() // the storage boundary is where the trim happens (#1381)

	s.Equal(encounter.DefaultRetention, s.storyLen(enc),
		"an unset retention must take DefaultRetention, not unbounded")
}

// TestRetentionSurvivesReload pins that the policy is construction data that
// persists. A reloaded encounter that silently reverted to the default would
// change behaviour on a restart — the class of bug that only appears in
// production, after a deploy, on a session that had been running for hours.
func (s *RetentionTestSuite) TestRetentionSurvivesReload() {
	const window = 8
	enc := s.walkingEncounter(window)
	s.generateBeats(enc, 40)

	data := enc.ToData()
	s.Equal(window, data.Retention, "retention is persisted, not inferred")

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{}, Data: data})
	s.Require().NoError(err)

	// The reloaded encounter must still be trimming to the SAME window at its
	// next save: keep appending and the count must return to the window
	// rather than drift toward the default. Between saves the live log holds
	// window + delta — the bound the load-per-verb law guarantees (#1381).
	s.generateBeats(reloaded, 20)
	s.Equal(window+20, s.storyLen(reloaded),
		"between saves the live log is the loaded window plus the new delta")
	reloaded.ToData()
	s.Equal(window, s.storyLen(reloaded),
		"a reloaded encounter keeps the policy it was built with")
}

// TestFloorSurvivesReload pins that the trimmed floor is reconstructed at load.
// It is derived from the log rather than persisted, so this is the test that
// proves the derivation agrees with what the writer actually trimmed — a
// reloaded encounter that forgot its floor would start accepting resume points
// for beats it no longer holds, answering them with a silent short read.
func (s *RetentionTestSuite) TestFloorSurvivesReload() {
	const window = 8
	enc := s.walkingEncounter(window)
	s.generateBeats(enc, 40)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{}, Data: enc.ToData()})
	s.Require().NoError(err)

	_, err = reloaded.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 33})
	s.ErrorIs(err, encounter.ErrTrimmed, "the floor must be known after a reload")

	_, err = reloaded.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: 34})
	s.NoError(err, "and must not over-reach after a reload either")
}

// TestUntrimmedEncounterHasNoFloor is the negative control for the derivation:
// an encounter that has never trimmed must accept every resume point it ever
// issued. A floor derivation that returned something non-zero for an untrimmed
// log would reject perfectly valid reconnects, and no rejection test could see
// it.
func (s *RetentionTestSuite) TestUntrimmedEncounterHasNoFloor() {
	enc := s.walkingEncounter(encounter.RetentionUnbounded)
	s.generateBeats(enc, 5)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{}, Data: enc.ToData()})
	s.Require().NoError(err)

	for seq := uint64(1); seq <= 6; seq++ {
		_, err := reloaded.Story(&encounter.StoryInput{Audience: "p1", AfterSeq: seq})
		s.NoError(err, "seq %d was never trimmed and must be servable", seq)
	}
}

// TestVerbBeatsSurviveTheVerb is the #1381 pin. A single engine call that
// mints more beats than the retention window must leave ALL of them readable
// in the live Story: PR #1377's review reproduced a verb trimming its own
// beats mid-verb, past every cursor, so nothing could ever commit them. The
// window is one and Pump mints two beats in one call — the tick frame and the
// patroller's movement — so per-append enforcement would destroy the tick
// beat before Pump returned.
//
// The second half is the boundary doing its job as before: the next ToData
// emits exactly the window with the floor advanced, and a resume below that
// floor is refused.
func (s *RetentionTestSuite) TestVerbBeatsSurviveTheVerb() {
	patrol := &patrolDecider{positions: []spatial.Position{cellAt(5, 5), cellAt(3, 3)}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10), rectRegion(room2, 10, 0, 10, 10)}, Walls: twoRoomSealedWall(),
		},
		Members: []encounter.MemberInput{
			// The watcher is sealed in the second room: a monster a player can
			// see is a monster in a fight, and Pump does not move fight
			// members — the wall is what lets the patroller patrol.
			{ID: "watcher", Kind: encounter.KindPlayer, Position: spatial.Position{X: 10, Y: 0}},
			{ID: "pacer", Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 1}, Decider: patrol},
		},
		Endings:   []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		Retention: 1,
	})
	s.Require().NoError(err)

	// One engine call, two beats minted — retention+1 for a window of one.
	pumpOut, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Len(pumpOut.MonsterMoves, 1, "the patroller must actually have moved")

	entries, err := enc.Story(&encounter.StoryInput{Audience: "watcher", AfterSeq: 0})
	s.Require().NoError(err)
	s.Len(entries, 3, "scene-opened, tick, movement: the verb's own beats all survive the verb")
	for i, entry := range entries {
		s.Equal(uint64(i+1), entry.Seq, "nothing was trimmed out from under the verb")
	}

	data := enc.ToData()
	s.Require().Len(data.Log.Entries, 1, "the boundary then emits exactly the window")
	s.Equal(uint64(3), data.Log.Entries[0].Seq, "and keeps the newest beat")

	_, err = enc.Story(&encounter.StoryInput{Audience: "watcher", AfterSeq: 2})
	s.ErrorIs(err, encounter.ErrTrimmed, "the floor advanced with the save, exactly as before")
}

// TestBlobStaysBoundedAcrossSaveLoadCycles is the memory pin for contract
// point 2: under the load-per-verb law the blob is bounded by construction at
// window + one verb's delta — a load, an arbitrarily large verb delta, and a
// save must land the blob back at exactly the window, cycle after cycle.
func (s *RetentionTestSuite) TestBlobStaysBoundedAcrossSaveLoadCycles() {
	const window = 8
	enc := s.walkingEncounter(window)
	s.generateBeats(enc, 40)

	data := enc.ToData()
	s.Require().Len(data.Log.Entries, window, "first save: the blob holds the window, not the 41-beat history")

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{}, Data: data})
	s.Require().NoError(err)

	// A big verb delta on the reloaded encounter: the live log grows to
	// window + delta and no further.
	s.generateBeats(reloaded, 40)
	s.Equal(window+40, s.storyLen(reloaded),
		"the live log is bounded by the loaded window plus this load's own delta")

	data2 := reloaded.ToData()
	s.Len(data2.Log.Entries, window, "the next save lands the blob back at the window")

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{}, Data: data2})
	s.Require().NoError(err, "and that bounded blob round-trips")
}

func TestRetentionSuite(t *testing.T) {
	suite.Run(t, new(RetentionTestSuite))
}
