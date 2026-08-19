// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// standing_test.go is death lane slice D1 (rpg-toolkit#1075): the world
// notices who is down.
//
// Three live defects the death census named, one suite. A corpse could start a
// fight, because sides were partitioned by Kind alone. A corpse could keep
// walking, because Pump had no standing filter. A downed character could keep
// swinging, because nothing took them out of the turn order. None of the three
// is a rule this module is allowed to know — the composition cannot import the
// rulebook (law C1) — so every scene below installs a FAKE Standing and drives
// it by hand. Member IDs in, member IDs out; not one of these tests mentions a
// hit point, and neither does the package.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// deathScene is the death lane's shared fixture — D1's standing pins run on it
// here, D2's endings run on it in defeat_test.go. It holds no tests of its own;
// embedding it hands a suite the crypt, the story readers, and the clock and
// roster lookups every death scene asks the same questions with.
type deathScene struct {
	suite.Suite
}

type StandingSuite struct {
	deathScene
}

func TestStandingSuite(t *testing.T) {
	suite.Run(t, new(StandingSuite))
}

const cryptID = "crypt"

// wolf is this suite's third body. alice, bob and goblin come from
// encounter_test.go.
const wolf = encounter.MemberID("wolf")

// scene is the shared set: one open 12x12 room with alice, the goblin, and
// (when asked for) the wolf standing in plain sight of each other.
//
// Plain sight is deliberate. Everywhere a corpse is expected to change nothing,
// the control is a scene that WOULD have fought — so a test that stops seeing a
// bubble is seeing the standing rule and not a sightline.
//
// Retention is unbounded because most of these tests read the story, and a
// window would make "what the story says" a question about trimming.
// TestAForgottenDeathIsToldAgain sets its own, on purpose.
func (s *deathScene) scene(standing encounter.Standing, members ...encounter.MemberInput) *encounter.Encounter {
	s.T().Helper()

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Sight:      everyoneSeesTheWholeMap{}, Standing: standing,
		Retention: encounter.RetentionUnbounded,
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{
			{ID: cryptID, Width: 12, Height: 12},
		}},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// pair is alice and a goblin in plain sight of each other, the goblin pacing
// between two cells so a tick has something to show.
func (s *deathScene) pair(standing encounter.Standing) *encounter.Encounter {
	s.T().Helper()

	return s.scene(standing,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Room: cryptID,
			Position: spatial.Position{X: 0, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Room: cryptID,
			Position: spatial.Position{X: 0, Y: 10},
			Decider: &patrolDecider{positions: []spatial.Position{
				{X: 1, Y: 10}, {X: 0, Y: 10},
			}}},
	)
}

// trio adds the wolf, so a fight has an order long enough for a gap in the
// middle of it to be visible.
func (s *deathScene) trio(standing encounter.Standing) *encounter.Encounter {
	s.T().Helper()

	return s.scene(standing,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Room: cryptID,
			Position: spatial.Position{X: 0, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Room: cryptID,
			Position: spatial.Position{X: 0, Y: 10}},
		encounter.MemberInput{ID: wolf, Kind: encounter.KindMonster, Room: cryptID,
			Position: spatial.Position{X: 2, Y: 10}},
	)
}

func (s *deathScene) clockOf(enc *encounter.Encounter, id encounter.MemberID) encounter.ClockKind {
	s.T().Helper()

	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
	s.Require().NoError(err)

	return out.Kind
}

func (s *deathScene) orderOf(enc *encounter.Encounter, id encounter.MemberID) []encounter.MemberID {
	s.T().Helper()

	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
	s.Require().NoError(err)

	return out.Order
}

func (s *deathScene) positionOf(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	s.T().Helper()

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Fail("no such member", "%q is not in the roster", id)

	return spatial.Position{}
}

// beatKindsOf reads one audience's whole retained story as its list of beat
// kinds — the shape the beat-order law is asserted in.
func (s *deathScene) beatKindsOf(enc *encounter.Encounter, audience encounter.MemberID) []string {
	s.T().Helper()

	kinds := make([]string, 0)
	for _, beat := range s.beatsOf(enc, audience) {
		kinds = append(kinds, beat["beat"].(string))
	}

	return kinds
}

func (s *deathScene) beatsOf(enc *encounter.Encounter, audience encounter.MemberID) []map[string]any {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)

	beats := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		beats = append(beats, beat)
	}

	return beats
}

// --- construction is total ------------------------------------------------

// TestSetupRefusesAnEncounterThatCannotAskWhoIsDown is the #1021 precedent
// applied to hit points: an encounter consults Standing from first light, so
// one that cannot ask is a misconfiguration, and it is refused at the door
// rather than guarded at the use site. There is no nil-means-everyone-standing
// — a default would be this module deciding a rule it is not allowed to know.
func (s *StandingSuite) TestSetupRefusesAnEncounterThatCannotAskWhoIsDown() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{
			{ID: cryptID, Width: 12, Height: 12},
		}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: cryptID, Position: spatial.Position{X: 0, Y: 2}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})

	s.Require().ErrorIs(err, encounter.ErrNoStanding)
}

// TestLoadRefusesAnEncounterThatCannotAskWhoIsDown is the other door. A loaded
// encounter runs the same consult on its first sight refresh, and a blob is
// exactly as capable of arriving without a capability as a Setup is.
func (s *StandingSuite) TestLoadRefusesAnEncounterThatCannotAskWhoIsDown() {
	saved := s.pair(&downList{}).ToData()

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       saved,
		Initiative: orderAsGiven{},
	})

	s.Require().ErrorIs(err, encounter.ErrNoStanding)
}

// TestALoadedEncounterAsksToo proves the capability is actually wired through
// Load rather than merely demanded by it — the blob comes back mid-fight and
// the reloaded encounter drops the body out of the order on its next consult.
func (s *StandingSuite) TestALoadedEncounterAsksToo() {
	saved := s.trio(&downList{}).ToData()

	down := &downList{down: []encounter.MemberID{goblin}}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       saved,
		Initiative: orderAsGiven{},
		Sight:      everyoneSeesTheWholeMap{}, Standing: down,
	})
	s.Require().NoError(err)
	s.Require().Equal([]encounter.MemberID{alice, goblin, wolf}, s.orderOf(enc, alice),
		"the blob was saved mid-fight, with all three in the order")

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal([]encounter.MemberID{alice, wolf}, s.orderOf(enc, alice),
		"the reloaded encounter asked, and the fight closed over the gap")
}

// --- a corpse cannot start or join a fight --------------------------------

// TestACorpseStartsNoFight is the first of the three census defects:
// sidesInContactOrder partitioned by Kind alone, so a dead monster in view was
// still a monster in view, and sight started a fight with it.
func (s *StandingSuite) TestACorpseStartsNoFight() {
	fighting := s.pair(&downList{})
	s.Require().Equal(encounter.ClockTurn, s.clockOf(fighting, alice),
		"control: standing, in plain sight, first light starts the fight")

	quiet := s.pair(&downList{down: []encounter.MemberID{goblin}})

	s.Equal(encounter.ClockWorld, s.clockOf(quiet, alice), "a body in view is not an enemy in view")
	s.Equal(encounter.ClockWorld, s.clockOf(quiet, goblin), "and it is on no turn order of its own")
}

// TestADownPlayerStartsNoFightEither pins the same rule from the other side. A
// filter that only ever ran over monsters would pass every test above and
// still let an unconscious character walk into a wolf and start a fight.
func (s *StandingSuite) TestADownPlayerStartsNoFightEither() {
	quiet := s.pair(&downList{down: []encounter.MemberID{alice}})

	s.Equal(encounter.ClockWorld, s.clockOf(quiet, alice))
	s.Equal(encounter.ClockWorld, s.clockOf(quiet, goblin))
}

// TestACorpseDoesNotJoinAFightAlreadyRunning is the straggler path's other
// half. A monster in contact with a live fight transfers into it at the end of
// the order; a body lying beside one is never swept in, tick after tick.
func (s *StandingSuite) TestACorpseDoesNotJoinAFightAlreadyRunning() {
	down := &downList{down: []encounter.MemberID{wolf}}
	enc := s.trio(down)

	s.Require().Equal([]encounter.MemberID{alice, goblin}, s.orderOf(enc, alice),
		"the fight formed without the body")
	s.Equal(encounter.ClockWorld, s.clockOf(enc, wolf))

	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal([]encounter.MemberID{alice, goblin}, s.orderOf(enc, alice),
		"and a tick beside a running fight does not sweep it in")
	s.Equal(encounter.ClockWorld, s.clockOf(enc, wolf))
}

// --- a downed character stops swinging ------------------------------------

// TestADownMemberLeavesTheTurnOrder is the second census defect: nothing took a
// downed character out of the order, so the fight kept handing them turns and
// the SDK kept letting them swing (#845, reproduced on the new stack).
//
// The gap closes rather than being held open — mid-round order splicing is the
// same machinery a straggler leaving uses — and the body lands on the world
// clock, because R6 says every member is on exactly one clock and a corpse is
// still a member.
func (s *StandingSuite) TestADownMemberLeavesTheTurnOrder() {
	down := &downList{}
	enc := s.trio(down)
	s.Require().Equal([]encounter.MemberID{alice, goblin, wolf}, s.orderOf(enc, alice))

	down.down = []encounter.MemberID{goblin}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal([]encounter.MemberID{alice, wolf}, s.orderOf(enc, alice),
		"the order closes over the body, mid-round")
	s.Equal(encounter.ClockWorld, s.clockOf(enc, goblin),
		"which leaves it on the world clock — present, not gone")
}

// TestTheSpliceIsNotWhatEndsTheFight replaces D1's TestTheFightGoesOnWithoutIt.
//
// That pin held the interim honestly: every monster down, alice still in a
// fight, because self-dissolve was D2's ruling and not D1's. D2 landed
// (rpg-toolkit#1078), so the scene it pinned now ends — and the pin is rewritten
// rather than deleted, because the SPLICE still must not be what ends it. The
// order closing over a body is one thing; a side being gone is another, and
// defeat_test.go owns the second. If the two ever collapse into each other, this
// is the test standing where the seam used to be.
func (s *StandingSuite) TestTheSpliceIsNotWhatEndsTheFight() {
	down := &downList{}
	enc := s.trio(down)

	down.down = []encounter.MemberID{goblin}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice),
		"one body spliced out of a fight the wolf is still standing in")
	s.Equal([]encounter.MemberID{alice, wolf}, s.orderOf(enc, alice))

	// And the fight that pin used to describe — every monster down — is over.
	down.down = []encounter.MemberID{goblin, wolf}
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice),
		"the last one down ends it, with no caller (ByDefeat)")
	s.Empty(enc.ToData().Bubbles)
}

// --- a corpse does not walk -----------------------------------------------

// TestACorpseDoesNotWalk is the third census defect: Pump consulted every
// monster's decider with no standing filter, so a dead goblin kept patrolling.
//
// The second half is the pull itself. Nothing is rebuilt and nothing is told
// that the goblin recovered — the capability's answer simply changes, and the
// next tick moves it again. A composition that cached the first answer passes
// the first half of this test and fails here.
func (s *StandingSuite) TestACorpseDoesNotWalk() {
	down := &downList{down: []encounter.MemberID{goblin}}
	enc := s.pair(down)
	fell := s.positionOf(enc, goblin)

	// It is on the WORLD clock, which is the only place Pump thinks for
	// anybody — so the tick below skipping it is the standing filter and not
	// the bubble filter that was already there.
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, goblin))

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Empty(out.MonsterMoves, "a body has nowhere to be")
	s.Equal(fell, s.positionOf(enc, goblin), "and is still where it fell")

	down.down = nil
	out, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Require().Len(out.MonsterMoves, 1, "the rulebook says it is standing, so it walks")
	s.NotEqual(fell, s.positionOf(enc, goblin))
}

// --- the story says so ----------------------------------------------------

// TestTheStorySaysWhoIsDown is ruled fork (d): a minimal beat, kind and who.
// It is everything a client needs to render a death; what the wire says about
// hit points is a separate decision with its own case (census S3).
func (s *StandingSuite) TestTheStorySaysWhoIsDown() {
	enc := s.pair(&downList{down: []encounter.MemberID{goblin}})

	var found []map[string]any
	for _, beat := range s.beatsOf(enc, alice) {
		if beat["beat"] == "down" {
			found = append(found, beat)
		}
	}

	s.Require().Len(found, 1)
	s.Equal("goblin", found[0]["member"], "kind, and who")
}

// TestTheStorySaysItOnce is what makes the beat a beat rather than a status
// line. The consult runs at every sight refresh — every walk, every tick — and
// a body is news exactly once.
func (s *StandingSuite) TestTheStorySaysItOnce() {
	enc := s.pair(&downList{down: []encounter.MemberID{goblin}})

	for i := 0; i < 3; i++ {
		_, err := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(err)
	}

	downs := 0
	for _, kind := range s.beatKindsOf(enc, alice) {
		if kind == "down" {
			downs++
		}
	}

	s.Equal(1, downs, "three ticks over a body, one down beat")
}

// TestAForgottenDeathIsToldAgain pins the accepted cost of holding NO state.
//
// There is no down flag anywhere in this module and none in the blob: the
// composition asks the rulebook who is down, and asks its own STORY whether it
// has already said so. The story is the only memory involved, and the story has
// a retention window — so when the window forgets the down beat, the next
// consult says it again.
//
// Pinned rather than left to be discovered, because the obvious "fix" is to
// remember it, and remembering it is the dual state this whole shape exists to
// avoid (heal a character and the sheet says four hit points while the
// composition still says down).
func (s *StandingSuite) TestAForgottenDeathIsToldAgain() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Sight:      everyoneSeesTheWholeMap{}, Standing: &downList{down: []encounter.MemberID{goblin}},
		Retention: 4,
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{
			{ID: cryptID, Width: 12, Height: 12},
		}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: cryptID, Position: spatial.Position{X: 0, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Room: cryptID, Position: spatial.Position{X: 0, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// Every down beat ever appended, by sequence — the retained story holds at
	// most one at a time, so it has to be collected as the ticks go by.
	told := map[uint64]bool{}
	collect := func() {
		story, serr := enc.Story(&encounter.StoryInput{Audience: alice})
		s.Require().NoError(serr)
		for _, entry := range story {
			var beat map[string]any
			s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
			if beat["beat"] == "down" {
				told[entry.Seq] = true
			}
		}
	}

	collect()
	for i := 0; i < 8; i++ {
		_, perr := enc.Pump(&encounter.PumpInput{})
		s.Require().NoError(perr)
		collect()
	}

	s.Greater(len(told), 1, "the story forgot the body, so the world noticed it again")
}

// --- beat order -----------------------------------------------------------

// TestTheSceneOpensThenNoticesThenFights holds the beat-order law where the new
// beat meets it: A VERB'S OWN BEAT PRECEDES ANY BEAT ITS CONSEQUENCES APPEND.
//
// The scene opens; the world notices the goblin is down; the fight that the
// remaining wolf starts comes last. Noticing precedes the fight because it
// DECIDES the fight — a bubble-formed beat naming a body would be a story that
// contradicts the beat two lines above it.
func (s *StandingSuite) TestTheSceneOpensThenNoticesThenFights() {
	enc := s.trio(&downList{down: []encounter.MemberID{goblin}})

	s.Equal([]string{"scene-opened", "down", "bubble-formed"}, s.beatKindsOf(enc, alice))
	s.Equal([]encounter.MemberID{alice, wolf}, s.orderOf(enc, alice),
		"and the fight is the two who could have one")
}

// TestThePumpTicksThenNotices is the same law at the verb where a body most
// often appears: the tick is the cause, everything the world notices inside it
// is the effect, and the transfer out of the fight follows the notice that
// caused it.
func (s *StandingSuite) TestThePumpTicksThenNotices() {
	down := &downList{}
	enc := s.trio(down)
	s.Require().Equal([]string{"scene-opened", "bubble-formed"}, s.beatKindsOf(enc, alice))

	down.down = []encounter.MemberID{goblin}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal([]string{"scene-opened", "bubble-formed", "tick", "down", "transferred"},
		s.beatKindsOf(enc, alice))
}

// --- the body stays --------------------------------------------------------

// TestACorpseIsStillOnTheMapAndInTheRoster is ruled fork (a), whole. A member
// the rulebook reports down is never removed: the killing blow stays
// recordable (Record validates targets against the roster), Members still
// reports where the body lies, and Exit carries it forward — a TPK must not
// close a dungeon as "abandoned" because the roster emptied itself.
func (s *StandingSuite) TestACorpseIsStillOnTheMapAndInTheRoster() {
	enc := s.pair(&downList{down: []encounter.MemberID{goblin}})

	s.Equal(spatial.Position{X: 0, Y: 10}, s.positionOf(enc, goblin),
		"still on the map, at the cell it fell on")

	_, err := enc.Record(&encounter.RecordInput{
		Kind:    encounter.OutcomeStruck,
		Actor:   alice,
		Targets: []encounter.MemberID{goblin},
		Values:  map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	s.Require().NoError(err, "the killing blow is recordable against a body")

	out, err := enc.Exit(&encounter.ExitInput{Member: goblin})
	s.Require().NoError(err, "and the body is carried out, not vanished")
	s.Equal(goblin, out.Outcome.ID)
}

// --- the capability is a seam, not a back door -----------------------------

// TestNobodyCanPushADownBeatIn is the census's rejected shape, refused
// structurally. Down arrives by the composition ASKING, at the choke point
// where it already asks about sight; a caller that could push the beat in would
// be a second system deciding the same thing, and the first one would always
// win, because it reaches the fact first.
//
// Record itself now performs that ask (rpg-toolkit#1083), which is why the
// refusal is worth re-reading rather than assuming: a caller gets the down beat
// it would have pushed, in the same call it recorded the blow, and still cannot
// write one. The beat says what the rulebook answered. It has never said what a
// caller claimed.
func (s *StandingSuite) TestNobodyCanPushADownBeatIn() {
	enc := s.pair(&downList{})

	_, err := enc.Record(&encounter.RecordInput{Kind: encounter.OutcomeDown, Actor: alice})

	s.Require().ErrorIs(err, encounter.ErrInvalidData)
}

// TestADownAnswerNamingAStrangerIsRefused holds the capability to its contract.
// The composition asks about ITS roster; an answer naming somebody else is a
// rulebook defect, and a defect this module cannot act on is one it refuses
// rather than ignores — silently dropping the name would make a mis-wired
// Standing look like a rule that simply does not fire.
//
// The refusal is atomic (R5): the tick that raised it moved nothing.
func (s *StandingSuite) TestADownAnswerNamingAStrangerIsRefused() {
	rulebook := &strangerWhenTold{}
	enc := s.pair(rulebook)
	before := s.positionOf(enc, goblin)

	rulebook.lying = true
	_, err := enc.Pump(&encounter.PumpInput{})

	s.Require().ErrorIs(err, encounter.ErrNotMember)
	s.Equal(before, s.positionOf(enc, goblin), "and nothing moved on the way to the refusal")
}
