// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// intimidate_test.go is the first shenanigan across the whole seam
// (rpg-project#454): the price, the derived DC, the refusals, and the deed
// that reaches the goblin's own mind.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type IntimidateSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters

	// authored, when set, is the dungeon author's hand on the spawn — what
	// dungeonspec's MonsterPlacement compiles to and a host forwards. Most
	// scenes leave it nil, which is the ordinary placement that prices
	// nothing and plants nothing.
	authored func(*session.SpawnInput)
}

func TestIntimidateSuite(t *testing.T) {
	suite.Run(t, new(IntimidateSuite))
}

func (s *IntimidateSuite) SetupTest() {
	s.sessions, s.encounters, s.authored = newFakeSessions(), newFakeEncounters(), nil
	// ALICE IS PROFICIENT IN INTIMIDATION, and every scene in this suite
	// depends on it: an untrained checker rolls the verb at disadvantage
	// (rpg-project#457 R2), which costs two faces instead of one and would
	// make every scripted die in this file about the untrained rule rather
	// than about the thing it says it tests. The rule's own scenes are at the
	// bottom of this file, and they script two faces on purpose.
	//
	// CHA 8 is a -1 modifier and proficiency is +2, so a d20 of 10 totals 11
	// — over the goblin's derived DC of 9. A d20 of 5 totals 6 and does not.
	// Both scenes turn on one die.
	s.characters = newFakeCharacters(talkingFighter("alice"))
}

// aYard opens a session with alice alone in an 8x8 hall, then spawns a goblin
// four cells off. Props, when given, block the sightline between them.
func (s *IntimidateSuite) aYard(rolls []int, props ...encounter.PropInput) *session.Manager {
	return s.aYardDriven(session.Pass{}, rolls, props...)
}

// aYardDriven is [IntimidateSuite.aYard] with the monster's turn driven by a
// real brain instead of passed.
func (s *IntimidateSuite) aYardDriven(
	driver session.TurnDriver, rolls []int, props ...encounter.PropInput,
) *session.Manager {
	// Two initiative rolls first: spawning the goblin in the open forms a
	// fight, and initiative is rolled before any check this suite asserts.
	// The scene with a wall forms no fight and never reaches a check, so the
	// unused faces cost it nothing.
	roller := &sequenceDice{rolls: append([]int{10, 10}, rolls...)}
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: roller,
		TurnDriver: driver,
		Sessions:   s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Props:   props,
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	spawn := &session.SpawnInput{
		Session: "sess", ID: "goblin", Ref: refs.Monsters.Goblin().String(),
		Position: spatial.Position{X: 5, Y: 1},
	}
	if s.authored != nil {
		s.authored(spawn)
	}
	_, err = mgr.Spawn(ctx, spawn)
	s.Require().NoError(err)

	// THE CLOCK IS AUTHORED, not rolled for — [turnWorld]'s own reason, and
	// one more: a wall between the two means no fight forms on sight at all,
	// and every scene here needs alice on the turn clock so the refusal under
	// test is the one it says it is.
	stored, err := s.encounters.GetEncounter(ctx, "world")
	s.Require().NoError(err)
	s.Require().NoError(s.encounters.SaveEncounter(ctx, "world",
		turnWorld(stored, []string{"alice", "goblin"}, 0)))

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "alice is on the turn clock")

	return mgr
}

func (s *IntimidateSuite) threaten(mgr *session.Manager) (*session.IntimidateOutput, error) {
	return mgr.Intimidate(context.Background(), &session.IntimidateInput{
		Session: "sess", Member: "alice", Target: "goblin",
	})
}

// held reads whether the goblin holds a deed with this verb from alice.
func (s *IntimidateSuite) held(mgr *session.Manager, verb string) bool {
	sightings, err := mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "goblin"})
	s.Require().NoError(err)
	for _, holding := range sightings {
		if holding.Channel != string(deed.Channel) {
			continue
		}
		saw, err := deed.Decode(holding.Payload)
		s.Require().NoError(err)
		if saw.Verb == verb && string(saw.Actor) == "alice" {
			return true
		}
	}
	return false
}

func (s *IntimidateSuite) row(mgr *session.Manager) session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, d := range out.Declarations {
		if d.Verb == session.VerbIntimidate {
			return d
		}
	}
	s.Require().Fail("no Intimidate row")
	return session.Declaration{}
}

// A goblin's derived DC is 9: 10 + its WIS 8 modifier, and nothing about that
// number is stored anywhere (rpg-project#454).
func (s *IntimidateSuite) TestTheDerivedDifficultyIsTheGoblinsPassiveInsight() {
	out, err := s.threaten(s.aYard([]int{10}))
	s.Require().NoError(err)

	s.Equal(9, out.DC, "10 + WIS 8's -1")
	s.Equal("intimidation", out.Applied.Ability, "the one derived route")
	s.Equal(11, out.Total, "the d20's 10 with CHA 8's -1 and proficiency's +2")
	s.True(out.Beaten, "a total that meets the DC beats it")
	s.Equal("goblin", out.Target)
}

// A beaten threat lands the deed on the goblin's own mind. That is the whole
// point of the verb: what happens next is the preset's, and the preset reads
// this.
func (s *IntimidateSuite) TestABeatenThreatLandsTheDeed() {
	mgr := s.aYard([]int{10})
	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.True(s.held(mgr, encounter.DeedIntimidate), "the goblin remembers who threatened it")
}

// A missed threat lands nothing. It is not an error — the die was rolled, the
// table saw it, and nothing reached the goblin.
func (s *IntimidateSuite) TestAMissedThreatLandsNothing() {
	mgr := s.aYard([]int{5})
	out, err := s.threaten(mgr)
	s.Require().NoError(err)

	s.False(out.Beaten)
	s.Equal(6, out.Total, "5 with CHA 8's -1 and proficiency's +2")
	s.Equal(9, out.DC)
	s.False(s.held(mgr, encounter.DeedIntimidate), "nothing for the mind to read")
}

// The placement's authored list wins whole, and the derived approach is not
// appended beside it.
func (s *IntimidateSuite) TestAnAuthoredCheckOverridesTheDerivedOne() {
	mgr := s.aYard([]int{10})

	stored, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	for i := range stored.Members {
		if stored.Members[i].ID == "goblin" {
			stored.Members[i].Intimidate = []encounter.CheckApproachData{{Ability: "intimidation", DC: 18}}
		}
	}
	s.Require().NoError(s.encounters.SaveEncounter(context.Background(), "world", stored))

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Equal(18, out.DC, "the author's number, not the stat block's 9")
	s.False(out.Beaten, "a 9 does not reach 18")
}

// The action is spent. A second threat in the same turn is refused, and the
// row Afford shows says so before the caller tries.
func (s *IntimidateSuite) TestTheActionIsSpent() {
	mgr := s.aYard([]int{10, 10})

	s.Require().True(s.row(mgr).Available, "the action is there to spend")
	s.Require().Equal(session.SlotAction, s.row(mgr).Slot)

	_, err := s.threaten(mgr)
	s.Require().NoError(err)

	_, err = s.threaten(mgr)
	s.Require().ErrorIs(err, session.ErrCannotAfford)

	row := s.row(mgr)
	s.False(row.Available, "and Afford said so without anybody trying")
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallNoBudget, row.Why.Reason)
}

// Refused outside the actor's turn, exactly as a swing is: this costs an
// action, and an action belongs to a turn.
func (s *IntimidateSuite) TestRefusedOffTurn() {
	mgr := s.aYard([]int{10})
	_, err := mgr.Intimidate(context.Background(), &session.IntimidateInput{
		Session: "sess", Member: "goblin", Target: "alice",
	})
	s.Require().ErrorIs(err, session.ErrNotYourTurn)
}

// Refused when the target cannot see the actor. A wall between them means the
// goblin never heard a threat, so there is nothing to be frightened of — and
// nothing is written.
func (s *IntimidateSuite) TestRefusedThroughAWall() {
	mgr := s.aYard([]int{10}, occludingProps(
		spatial.Position{X: 3, Y: 0}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 3, Y: 2},
	)...)

	_, err := s.threaten(mgr)
	// THE SEAM'S OWN SENTINEL. A raw encounter error reaching a host is the
	// S2 leak, and one the host cannot match is answered as Internal — an
	// ordinary refusal reported as a server fault.
	s.Require().ErrorIs(err, session.ErrUnwitnessed)
	s.Require().NotErrorIs(err, encounter.ErrUnwitnessed, "the composition's error stays inside")
	s.False(s.held(mgr, encounter.DeedIntimidate))

	// NOTHING WAS CHARGED, and the sheet proves it structurally: lighting
	// the turn's economy is the first thing spendOnIntimidate does, so a
	// character who never had one never reached the charge.
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Nil(stored.ActionEconomy, "a refusal must not cost the actor their turn")
}

// A threat needs somebody to threaten, and a member the encounter does not
// have is refused rather than rolled for.
func (s *IntimidateSuite) TestRefusals() {
	mgr := s.aYard([]int{10})

	_, err := mgr.Intimidate(context.Background(), nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = mgr.Intimidate(context.Background(), &session.IntimidateInput{Session: "sess", Member: "alice"})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = mgr.Intimidate(context.Background(), &session.IntimidateInput{
		Session: "sess", Member: "alice", Target: "nobody",
	})
	s.ErrorIs(err, session.ErrNoMember)
}

// Threatening another PLAYER is refused rather than given a made-up DC: a
// character has no stat block to derive passive Insight from, and nobody has
// brought the use case that would price one.
func (s *IntimidateSuite) TestAPlayerHasNoDerivedDifficulty() {
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr := s.aYard([]int{10})

	_, err := mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "bob", Position: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)

	_, err = mgr.Intimidate(context.Background(), &session.IntimidateInput{
		Session: "sess", Member: "alice", Target: "bob",
	})
	s.Require().ErrorIs(err, session.ErrNoSheet)
}

// beats is every beat kind the member's own stream carries, in order.
func (s *IntimidateSuite) beats(mgr *session.Manager, member string) []string {
	story, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	out := make([]string, 0, len(story))
	for _, event := range story {
		var beat struct {
			Beat string `json:"beat"`
		}
		s.Require().NoError(json.Unmarshal(event.Payload, &beat))
		out = append(out, beat.Beat)
	}
	return out
}

func (s *IntimidateSuite) endTurn(mgr *session.Manager, member string) {
	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: member,
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", member),
	})
	s.Require().NoError(err)
}

// events is the typed events the member's own stream carries.
func (s *IntimidateSuite) events(mgr *session.Manager, member string) []session.Event {
	story, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return story
}

// THE BEAT IS THE ONLY ACCOUNT OF THE ROLL, so it has to cross the seam
// TYPED. A threat writes no outcome beat, and a missed one writes nothing
// else at all — no deed, no fact, nothing about the monster changes — so an
// undecoded beat means the outcome reaches nobody. It shipped that way once:
// kindFor had no case and rpg-api saw EventUnknown.
//
// Driven through the real Manager rather than decodeBeat, because the unit
// pin passed the whole time the wire was broken: what was missing was the
// case, not the decoder.
func (s *IntimidateSuite) TestABeatenThreatSurfacesAsATypedEvent() {
	mgr := s.aYard([]int{10})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	var found *session.Event
	for i, event := range s.events(mgr, "alice") {
		if event.Kind == session.EventIntimidated {
			found = &s.events(mgr, "alice")[i]
		}
	}
	s.Require().NotNil(found, "the threat reached alice's stream as its own kind, not EventUnknown")
	body, ok := found.Body.(session.IntimidatedBody)
	s.Require().True(ok)
	s.Equal("alice", body.Actor)
	s.Equal("goblin", body.Target)
	s.Equal(9, body.DC)
	s.Equal(11, body.Total, "the numbers the response reported, on the wire")
	s.True(body.Beaten)

	// AND THE ROLL BEHIND THE NUMBER. This beat used to be {dc, total,
	// beaten} wide, so the faces that produced the total never left the
	// server (rpg-project#462).
	die := d20Of(s.T(), body.Calculation)
	s.Equal(20, die.DieSize)
	s.Equal(11, body.Calculation.Total)
	s.Equal(out.Seq, found.Seq, "IntimidateOutput.Seq references this event")

	// The goblin heard it too — the audience is the witnesses.
	for _, event := range s.events(mgr, "goblin") {
		if event.Kind == session.EventIntimidated {
			return
		}
	}
	s.Fail("the goblin was threatened and its own stream does not say so")
}

// The missed threat is the case that matters most: nothing else in the run
// records it, so a body carrying beaten:false IS the outcome.
func (s *IntimidateSuite) TestAMissedThreatSurfacesToo() {
	mgr := s.aYard([]int{5})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().False(out.Beaten)

	for _, event := range s.events(mgr, "alice") {
		if event.Kind != session.EventIntimidated {
			continue
		}
		body, ok := event.Body.(session.IntimidatedBody)
		s.Require().True(ok)
		s.Equal("alice", body.Actor)
		s.Equal("goblin", body.Target)
		s.Equal(9, body.DC)
		s.Equal(6, body.Total, "the roll the table saw, and the only record of it")
		s.False(body.Beaten)
		s.Equal(20, d20Of(s.T(), body.Calculation).DieSize)
		s.Equal(6, body.Calculation.Total,
			"a missed threat carries its roll too — full data until v1.0")
		return
	}
	s.Fail("a missed threat left no typed event, which is the whole outcome lost")
}

// where the goblin stands now.
func (s *IntimidateSuite) goblinAt(mgr *session.Manager) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: "goblin"})
	s.Require().NoError(err)
	return out.Position
}

// The acceptance case, end to end: the fighter threatens the archer goblin,
// beats its DC, and the goblin spends its whole next turn running away from
// her instead of opening with the bow it is holding.
//
// WHAT DECIDES IT IS NOW TWO AUTHORED LINES, and that is the slice's whole
// argument (rpg-project#465). The author writes `flee` under `intimidated`, so
// the beaten threat lands a `fled` deed on the goblin; the rulebook's own
// default table answers `away: actor` while that deed is fresh, so the running
// happens on the goblin's OWN time. The preset that used to read the deed and
// decide to run is deleted, and nothing was lost: a streamer can read both of
// those lines and change either.
func (s *IntimidateSuite) TestACowedGoblinRunsInsteadOfShooting() {
	s.authored = func(in *session.SpawnInput) {
		in.Table = encounter.Table{encounter.AnswerIntimidated: {{Weight: 1, Flee: true}}}
	}
	mgr := s.aYardDriven(session.Driver(), []int{10, 1, 1})

	s.Require().Equal(spatial.Position{X: 5, Y: 1}, s.goblinAt(mgr))

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	s.endTurn(mgr, "alice")
	// THE FARTHEST CELL IT COULD REACH, not the first step that helped. The
	// one-shot `fleeFrom` walk this replaced stepped away a cell at a time;
	// `away` hands the composition a POLICY and the composition routes it, so
	// the goblin spends its whole movement getting as far from alice at (1,1)
	// as the floor allows. The board is what knows where the corners are.
	s.Equal(spatial.Position{X: 4, Y: 7}, s.goblinAt(mgr),
		"its whole movement spent putting the hall between it and her")
}

// And the control, one die different: a missed threat lands nothing, so the
// same goblin on the same board stands exactly where it was and shoots.
func (s *IntimidateSuite) TestAnUncowedGoblinStandsAndShoots() {
	// The d20 that misses, then the goblin's own `time` roll on its turn:
	// `attack: enemy` and `hold` are the two eligible entries of the
	// rulebook's default table, a d200 with no temperament loading it, and a
	// face of 1 lands in the attack's share. Then the swing's own two faces.
	//
	// NOTHING IS AUTHORED HERE, which is the control's point twice over: the
	// threat missed, so no `fled` deed was landed, AND this goblin has no
	// `intimidate_failed` line for the miss to roll on — so the only die
	// between the check and the bow is the one that chose the bow.
	mgr := s.aYardDriven(session.Driver(), []int{5, 1, 8, 3, 200})
	// A turn is asked for intents until it is spent, so the goblin rolls its
	// table a second time with its bow already fired; 200 of 200 lands on
	// `hold` and the turn closes.

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().False(out.Beaten)

	s.endTurn(mgr, "alice")
	s.Equal(spatial.Position{X: 5, Y: 1}, s.goblinAt(mgr), "it never moved")

	shot := false
	for _, beat := range s.beats(mgr, "alice") {
		if beat == "struck" || beat == "missed" {
			shot = true
		}
	}
	s.True(shot, "nothing frightened it, so it used its bow")
}

// A CORNERED COWARD SPENDS ITS TURN RUNNING AND DOES NOT SHOOT, and this test
// is here because the answer used to be the opposite one.
//
// The mind ladder had a law about it — "keeping range is a preference; an
// archer that cannot step away stands and shoots" (rung 0) — and that law is
// deleted with the ladder (rpg-project#465 §7). The table has no rung beneath
// a word: `away` is a whole turn spent going somewhere, terminal by
// construction, and a creature that rolled it and then found the wall has
// still spent its turn on it. It rolls again NEXT turn, on a table that offers
// `attack: enemy` beside the running, so an archer with its back to the wall
// is not frozen — it is one round slower to start shooting than it used to be.
//
// THAT IS A CHANGE A WALK WILL SEE, which is the whole reason this is pinned
// rather than deleted: the difference between "the fear is not working" and
// "the fear costs a round here" is exactly the kind of thing a streamer would
// report as a bug. It is also what a table lets an author fix without us —
// an `intimidated` entry that says `attack: actor` instead of `flee`, or a
// weight that leaves room for the bow.
func (s *IntimidateSuite) TestACorneredCowardSpendsItsTurnRunning() {
	s.authored = func(in *session.SpawnInput) {
		in.Table = encounter.Table{encounter.AnswerIntimidated: {{Weight: 1, Flee: true}}}
	}
	mgr := s.aYardDriven(session.Driver(), []int{10, 1, 1})

	_, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.endTurn(mgr, "alice")

	s.Require().Equal(spatial.Position{X: 4, Y: 7}, s.goblinAt(mgr),
		"it ran as far from her as the hall allows")
	for _, beat := range s.beats(mgr, "alice") {
		s.NotContains([]string{"struck", "missed"}, beat,
			"the run was the whole turn — `away` is terminal, and the ladder's rung 0 is gone")
	}
}

// guide hands alice a Guidance die on her stored sheet — GuidedUnlockSuite's
// own pattern, for a threat instead of a lock.
func (s *IntimidateSuite) guide() {
	ctx := context.Background()
	stored, err := s.characters.GetCharacter(ctx, "alice")
	s.Require().NoError(err)
	condition, err := conditions.NewGuidedCondition(conditions.NewGuidedConditionInput{
		MemberID: "alice", SourceID: "cleric-1", SourceRef: refs.Spells.Guidance(),
	})
	s.Require().NoError(err)
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, raw)
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))
}

func (s *IntimidateSuite) answer(mgr *session.Manager, choice session.ReactChoice) {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, row := range out.Declarations {
		if row.Verb != session.VerbReact {
			continue
		}
		_, err := mgr.React(context.Background(), &session.ReactInput{
			Session: "sess", Member: "alice", DeclarationID: row.ID, Choice: choice,
		})
		s.Require().NoError(err)
		return
	}
	s.Require().Fail("no open window for alice")
}

// The pose window, end to end: a threat by somebody holding a Guidance die
// stops after the d20 and asks them, and the answer finishes the same
// attempt. Nothing is re-rolled.
//
// The die is what decides it: 6 on the d20 with CHA 8's -1 and proficiency's
// +2 is a 7, which misses the goblin's 9; the d4's own 4 makes it 11, which
// beats it.
func (s *IntimidateSuite) TestAGuidedThreatStopsAndAsks() {
	mgr := s.aYard([]int{6, 4})
	s.guide()

	out, err := s.threaten(mgr)
	s.Require().NoError(err)

	// ROLL AND THE PRE-OFFER TOTAL, and nothing else — Unlock's paused shape,
	// shared rather than restated. Asserted as the WHOLE output instead of
	// field by field, because the claim is that nothing else is carried and
	// a three-field check would pass on a fourth nobody meant to send.
	s.True(out.Paused, "the attempt is waiting on an answer")
	s.Require().NotNil(out.Roll)
	s.Equal(6, *out.Roll, "the d20 the player is deciding about")
	s.Equal(session.IntimidateOutput{
		Paused: true, Total: 7, Roll: out.Roll, Target: "goblin",
		Seq: out.Seq, Saved: out.Saved, Delivery: out.Delivery,
	}, *out, "the pre-offer total, and no DC, no applied route and no verdict")
	s.False(s.held(mgr, encounter.DeedIntimidate), "and nothing has reached the goblin")

	// THE ACTION IS ALREADY GONE, read off the stored sheet rather than off
	// Afford: while a window is open every row on the panel is blocked with
	// ShortfallWindowOpen, so the panel cannot tell a spent action from a
	// frozen one. The economy can. A member who could answer the question
	// and then threaten somebody else with the same action would be getting
	// two for one.
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Require().NotNil(stored.ActionEconomy)
	s.Equal(0, stored.ActionEconomy.ActionsRemaining, "charged before it rolled")

	s.answer(mgr, session.ReactStrike)
	s.True(s.held(mgr, encounter.DeedIntimidate),
		"5 + a d4's own 4 meets DC 9, and the goblin has something to remember")
}

// The other branch: keeping the die finishes the same attempt without it,
// and the threat falls short.
func (s *IntimidateSuite) TestAGuidedThreatKeptFallsShort() {
	mgr := s.aYard([]int{6, 4})
	s.guide()

	_, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.answer(mgr, session.ReactHold)

	s.False(s.held(mgr, encounter.DeedIntimidate), "a 5 does not reach 9")
}

// AN AUTHORED DC HAS TO REACH A LIVE RUN, and SpawnInput is the only road it
// can travel: a host that resolves monster content at runtime builds its
// world empty of members and brings every monster in through Spawn. Without
// the field the sergeant priced at 12 is talked down on its stat block's 9
// and nobody can tell (rpg-project#454).
func (s *IntimidateSuite) TestAnAuthoredCheckSurvivesTheSpawn() {
	s.authored = func(in *session.SpawnInput) {
		in.Intimidate = []session.DoorApproach{{Ability: "intimidation", DC: 12}}
	}
	mgr := s.aYard([]int{10})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Equal(12, out.DC, "the author's number, not the goblin's passive Insight of 9")
	s.Equal(session.DoorApproach{Ability: "intimidation", DC: 12}, out.Applied)
	s.False(out.Beaten, "a 9 does not reach 12 — which the derived DC would have")
}

// And the ordinary placement, so the field's ABSENCE still means derived
// rather than ungated: the same scene with nothing authored is DC 9.
func (s *IntimidateSuite) TestAnUnauthoredSpawnStillDerives() {
	mgr := s.aYard([]int{10})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Equal(9, out.DC)
	s.True(out.Beaten)
}

// The world half has the same road to travel. An authored
// `on: { intimidated: { fact: … } }` reaches the run through Spawn, and a
// beaten threat teaches it to the witnesses — proved through the stance it
// flips, which is the only thing the fact is FOR.
func (s *IntimidateSuite) TestAnAuthoredFactSurvivesTheSpawnAndIsTaught() {
	const fact = "sergeant-cowed"
	s.authored = func(in *session.SpawnInput) {
		in.Table = encounter.Table{
			encounter.AnswerIntimidated: {{Weight: 1, Fact: encounter.FactID(fact)}},
		}
		in.Faction = "raiders"
	}
	// TWO FACES: the d20 for the check, then the world's own die for the
	// reaction table — one entry, so a d1, and the face is the only one there
	// is (rpg-project#458).
	mgr := s.aCamp(fact, []int{10, 1})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	// The stance turning IS the fact arriving: the disposition waits on it
	// and nothing else can flip it. Read off the seam's own event, because
	// this seam publishes no stance read.
	for _, event := range s.events(mgr, "alice") {
		if event.Kind != session.EventStanceChanged {
			continue
		}
		// The pair is unordered and written in its one normalized order,
		// which is why this reads party-first and the authored disposition
		// does not.
		s.Equal(session.StanceChangedBody{
			Between: []string{"party", "raiders"}, Stance: "neutral",
		}, event.Body, "cowing it in front of the camp turned the camp")
		return
	}
	s.Fail("the fact never reached the run, so the camp never learned anything")
}

// aCamp is the yard with a faction that is hostile until it learns a fact,
// and the goblin as that faction's mind — the smallest world in which
// teaching a fact is observable at this seam.
func (s *IntimidateSuite) aCamp(fact string, rolls []int) *session.Manager {
	roller := &sequenceDice{rolls: append([]int{10, 10}, rolls...)}
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: roller, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Factions: []encounter.FactionInput{{ID: "raiders", Mind: "goblin"}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"raiders", encounter.FactionParty},
				Stance:  encounter.StanceHostile,
				Until:   encounter.TriggerFact{Fact: fact},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	spawn := &session.SpawnInput{
		Session: "sess", ID: "goblin", Ref: refs.Monsters.Goblin().String(),
		Position: spatial.Position{X: 5, Y: 1},
	}
	s.authored(spawn)
	_, err = mgr.Spawn(ctx, spawn)
	s.Require().NoError(err)

	stored, err := s.encounters.GetEncounter(ctx, "world")
	s.Require().NoError(err)
	s.Require().NoError(s.encounters.SaveEncounter(ctx, "world",
		turnWorld(stored, []string{"alice", "goblin"}, 0)))

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock)

	return mgr
}

// talkingFighter is [armedFighter] who took Intimidation and Persuasion at
// creation — the fixture every scene above needs, so that the untrained rule
// is exercised by the scenes that are about it and by no others.
func talkingFighter(id string) *character.Data {
	sheet := armedFighter(id)
	sheet.Skills = map[skills.Skill]shared.ProficiencyLevel{
		skills.Intimidation: shared.Proficient,
		skills.Persuasion:   shared.Proficient,
	}

	return sheet
}

// d20Of reads the roll behind an attempt beat: component 0 is the operation's
// own d20 pool by the shared calculation contract, which is what makes its
// keep record the place a reader looks for advantage and disadvantage.
func d20Of(t require.TestingT, calculation *session.RollCalculation) *session.DiceTrace {
	require.NotNil(t, calculation, "the beat carries the roll behind its total (rpg-project#462)")
	require.NotEmpty(t, calculation.Components)
	trace := calculation.Components[0].Dice
	require.NotNil(t, trace)
	return trace
}
