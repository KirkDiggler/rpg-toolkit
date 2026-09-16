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

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type IntimidateSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestIntimidateSuite(t *testing.T) {
	suite.Run(t, new(IntimidateSuite))
}

func (s *IntimidateSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	// CHA 8 is a -1 modifier, so a d20 of 10 totals 9 — exactly the goblin's
	// derived DC, which a total that MEETS beats. A d20 of 5 totals 4 and
	// does not. Both scenes turn on one die.
	s.characters = newFakeCharacters(armedFighter("alice"))
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

	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "goblin", Ref: refs.Monsters.Goblin().String(),
		Position: spatial.Position{X: 5, Y: 1},
	})
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
	s.Equal(9, out.Total, "the d20's 10 with CHA 8's -1")
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
	s.Equal(4, out.Total, "5 with CHA 8's -1")
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
	s.Require().ErrorIs(err, encounter.ErrUnwitnessed)
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
	s.Equal(session.IntimidatedBody{
		Actor: "alice", Target: "goblin", DC: 9, Total: 9, Beaten: true,
	}, found.Body, "the numbers the response reported, on the wire")
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
		s.Equal(session.IntimidatedBody{
			Actor: "alice", Target: "goblin", DC: 9, Total: 4, Beaten: false,
		}, event.Body, "the roll the table saw, and the only record of it")
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
// her instead of opening with the bow it is holding. Nothing decided that but
// the deed this verb landed.
func (s *IntimidateSuite) TestACowedGoblinRunsInsteadOfShooting() {
	driver, err := session.Minded(nil)
	s.Require().NoError(err)
	mgr := s.aYardDriven(driver, []int{10, 8, 3})

	s.Require().Equal(spatial.Position{X: 5, Y: 1}, s.goblinAt(mgr))

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	s.endTurn(mgr, "alice")
	s.Equal(spatial.Position{X: 5, Y: 5}, s.goblinAt(mgr),
		"six cells of movement spent putting distance between it and her")
}

// And the control, one die different: a missed threat lands nothing, so the
// same goblin on the same board stands exactly where it was and shoots.
func (s *IntimidateSuite) TestAnUncowedGoblinStandsAndShoots() {
	driver, err := session.Minded(nil)
	s.Require().NoError(err)
	mgr := s.aYardDriven(driver, []int{5, 8, 3})

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

// A CORNERED COWARD STILL SHOOTS, and that is the ladder's own law rather
// than a hole in the fear: "keeping range is a preference — an archer that
// cannot step away stands and shoots" (mind/behavior's ladder, rung 0). This
// eight-by-eight hall is small enough to reach the wall in one turn, so the
// frightened goblin runs out of floor and then uses the bow.
//
// Worth pinning because it is the thing a walk will see and could mistake
// for the fear not working. The sibling finding is rpg-toolkit#1758, where a
// fleeing coward orbits its pursuer instead of leaving the room.
func (s *IntimidateSuite) TestACorneredCowardShootsAnyway() {
	driver, err := session.Minded(nil)
	s.Require().NoError(err)
	mgr := s.aYardDriven(driver, []int{10, 8, 3})

	_, err = s.threaten(mgr)
	s.Require().NoError(err)
	s.endTurn(mgr, "alice")

	s.Require().Equal(spatial.Position{X: 5, Y: 5}, s.goblinAt(mgr), "it ran to the wall")
	shot := false
	for _, beat := range s.beats(mgr, "alice") {
		if beat == "struck" || beat == "missed" {
			shot = true
		}
	}
	s.True(shot, "and with nowhere left to go it shot")
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
// The die is what decides it: 6 on the d20 with CHA 8's -1 is a 5, which
// misses the goblin's 9; the d4's own face makes it 9, which meets it.
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
		Paused: true, Total: 5, Roll: out.Roll, Target: "goblin",
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
