// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// speak_world_test.go is Speak in the FRONT ROOM (rpg-project#458's scenes,
// carried onto options by rpg-project#569): the verb on the WORLD clock where
// no fight exists, the deed each skill lands, an untrained checker rolling
// plain, and the option's own answer rows arriving as a typed beat.

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type SpeakWorldSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters

	// sheet is the checker every scene loads; a scene that is about an
	// untrained checker swaps it before opening the room.
	sheet *character.Data

	// authored is the dungeon author's hand on the spawn, when a scene has
	// one. Every scene starts from the goblin carrying [threatLine] and
	// [appealLine].
	authored func(*dungeonspec.MonsterPlacement)

	// turnClock puts alice and the goblin into one authored bubble. False —
	// the default — leaves both in FREE ROAM, which is the front room.
	turnClock bool

	// driver is what drives the goblin when the world gives it time. Nil is
	// session.Pass{}.
	driver session.TurnDriver
}

func TestSpeakWorldSuite(t *testing.T) {
	suite.Run(t, new(SpeakWorldSuite))
}

func (s *SpeakWorldSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.sheet, s.authored, s.turnClock, s.driver = talkingFighter("alice"), nil, false, nil
	s.characters = newFakeCharacters(s.sheet)
}

// front opens a session with alice and a goblin in one hall, NEUTRAL to each
// other so no fight forms — a creature standing in a doorway and nobody has
// rolled initiative.
func (s *SpeakWorldSuite) front(rolls []int) *session.Manager {
	s.characters = newFakeCharacters(s.sheet)
	roller := &sequenceDice{rolls: rolls}
	driver := s.driver
	if driver == nil {
		driver = session.Pass{}
	}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: roller, TurnDriver: driver,
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters,
	})
	s.Require().NoError(err)

	sc := scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Factions: []encounter.FactionInput{{ID: "goblins"}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Party: []sceneSeat{seatAt("alice", 2, 1)},
	}

	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 5, 1)
	goblin.Faction = "goblins"
	goblin.Options = []encounter.Option{threatLine(), appealLine()}
	if s.authored != nil {
		s.authored(&goblin)
	}
	sc.Monsters = []dungeonspec.MonsterPlacement{goblin}
	// The scene's own conditions (a Guidance die already held) are written
	// AFTER the launch: the launch's first-admission long rest ends them.
	conditionsBefore := append([]json.RawMessage(nil), s.sheet.Conditions...)
	launchScene(s.T(), mgr, sc)
	if len(conditionsBefore) > 0 {
		stored, err := s.characters.GetCharacter(context.Background(), s.sheet.ID)
		s.Require().NoError(err)
		for _, scene := range conditionsBefore {
			if !slices.ContainsFunc(stored.Conditions, func(held json.RawMessage) bool { return bytes.Equal(held, scene) }) {
				stored.Conditions = append(stored.Conditions, scene)
			}
		}
		s.Require().NoError(s.characters.SaveCharacter(context.Background(), stored))
	}

	if s.turnClock {
		authorTurnClock(s.T(), s.encounters, testSession, []string{"alice", "goblin"}, 0)
	}

	return mgr
}

func (s *SpeakWorldSuite) say(mgr *session.Manager, option string) (*session.SpeakOutput, error) {
	return mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "goblin", OptionID: option,
	})
}

func (s *SpeakWorldSuite) appeal(mgr *session.Manager) (*session.SpeakOutput, error) {
	return s.say(mgr, "appeal")
}

func (s *SpeakWorldSuite) rows(mgr *session.Manager) []session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	return out.Declarations
}

func (s *SpeakWorldSuite) rowFor(mgr *session.Manager, verb session.Verb) session.Declaration {
	for _, decl := range s.rows(mgr) {
		if decl.Verb == verb {
			return decl
		}
	}
	s.Require().Fail("no row", string(verb))

	return session.Declaration{}
}

// beats reads one member's whole story.
func (s *SpeakWorldSuite) beats(mgr *session.Manager, member string) []witnessed {
	return storyBeats(s.T(), mgr, "sess", member)
}

// lastBeat decodes the last beat named name in alice's story into v, and
// reports whether there was one.
func (s *SpeakWorldSuite) lastBeat(mgr *session.Manager, name string, v any) bool {
	s.T().Helper()
	found := beatsNamed(s.beats(mgr, "alice"), name)
	if len(found) == 0 {
		return false
	}
	found[len(found)-1].decode(s.T(), v)
	return true
}

// answerCandidate is one eligible entry of an answered beat's table roll.
type answerCandidate struct {
	Entry   int `json:"entry"`
	Weight  int `json:"weight"`
	Percent int `json:"percent"`
	Loaded  int `json:"loaded"`
}

// answeredBeat is an answered beat's payload.
type answeredBeat struct {
	Creature   string            `json:"creature"`
	Prop       string            `json:"prop"`
	Verb       string            `json:"verb"`
	Option     string            `json:"option"`
	Beaten     bool              `json:"beaten"`
	Roll       int               `json:"roll"`
	Of         int               `json:"of"`
	Entry      int               `json:"entry"`
	Word       string            `json:"word"`
	Say        string            `json:"say"`
	Fact       string            `json:"fact"`
	Key        string            `json:"key"`
	Temper     string            `json:"temper"`
	Candidates []answerCandidate `json:"candidates"`
}

// spokeBeat is a spoke beat's payload.
type spokeBeat struct {
	Actor       string                   `json:"actor"`
	Target      string                   `json:"target"`
	Prop        string                   `json:"prop"`
	Option      string                   `json:"option"`
	Line        string                   `json:"line"`
	Skill       string                   `json:"skill"`
	DC          int                      `json:"dc"`
	Total       int                      `json:"total"`
	Beaten      bool                     `json:"beaten"`
	Calculation *session.RollCalculation `json:"calculation"`
}

// temperedBeat is a tempered beat's payload.
type temperedBeat struct {
	Member  string `json:"member"`
	Temper  string `json:"temper"`
	Roll    int    `json:"roll"`
	Of      int    `json:"of"`
	Faction string `json:"faction"`
}

// heldBy reads whether the goblin holds a deed with this verb from alice.
func (s *SpeakWorldSuite) heldBy(mgr *session.Manager, verb string) bool {
	return deedHeld(s.T(), mgr, "goblin", "alice", verb)
}

// THE HEADLINE. A player walks up to a neutral goblin in a room with no fight
// in it, talks to it, and it works — no initiative, no turn, no action spent,
// because there is no economy to spend from (R3).
func (s *SpeakWorldSuite) TestALineLandsOnTheWorldClock() {
	mgr := s.front([]int{10})

	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "precondition: nothing here is fighting")

	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Equal(9, out.DC, "the number the goblin's own line priced")
	s.Equal("persuasion", out.Applied.Ability)
	s.Equal(11, out.Total, "the d20's 10 with CHA 8's -1 and proficiency's +2")
	s.True(out.Beaten)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Nil(stored.ActionEconomy,
		"free roam has no economy: the verb must never ready a sheet, let alone spend from one")
}

// A threat lands there too — the clock gate is "not somebody else's turn",
// and a front room has no turn to be out of (R3, rpg-project#457).
func (s *SpeakWorldSuite) TestAThreatAlsoLandsOnTheWorldClock() {
	mgr := s.front([]int{10})

	out, err := s.say(mgr, "threaten")
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.Equal(9, out.DC)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Nil(stored.ActionEconomy, "free roam charged nothing, exactly as Move does there")
}

// Speak is OFFERED there too, as ONE row whose menu is every line the goblin
// offers alice, and the panel says it costs nothing — the row and the door
// agree.
func (s *SpeakWorldSuite) TestSpeakIsOfferedFreeInFreeRoam() {
	mgr := s.front([]int{10})

	rows := s.rows(mgr)
	s.Require().Len(rows, 1, "free roam offers one row: the line")
	decl := rows[0]
	s.Equal(session.VerbSpeak, decl.Verb)
	s.True(decl.Available, "the goblin is standing right there")
	s.Equal(session.SlotNone, decl.Slot, "nothing is spent in free roam")
	s.Nil(decl.Why, "there is no budget to fall short of")
	s.NotEmpty(decl.ID, "a compiled row carries its selector")
	s.Require().Len(decl.Candidates, 1, "one witness in the room")
	s.Equal("goblin", decl.Candidates[0].Member)
	s.True(decl.Candidates[0].Available, "a witness within reach is an available candidate")
	s.Equal([]session.SpeakOptionRow{
		{Target: "goblin", OptionID: "threaten", Line: threatLine().Say, Available: true},
		{Target: "goblin", OptionID: "appeal", Line: appealLine().Say, Available: true},
	}, decl.SpeakOptions, "every line, in the author's order, with the words the character says")
	s.Require().NotNil(decl.Information)
	s.NotEmpty(decl.Information.Description)
}

// On the turn clock it is still an action, and still the actor's own turn —
// the world clock changed what free roam offers, not what a fight costs.
func (s *SpeakWorldSuite) TestOnTheTurnClockItStillCostsAnAction() {
	s.turnClock = true
	mgr := s.front([]int{10})

	row := s.rowFor(mgr, session.VerbSpeak)
	s.Equal(session.SlotAction, row.Slot, "in a fight it draws from the action")
	s.True(row.Available)

	_, err := s.appeal(mgr)
	s.Require().NoError(err)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Require().NotNil(stored.ActionEconomy, "a turn-clock line readies and spends")
	s.Zero(stored.ActionEconomy.ActionsRemaining, "the standard action is gone")

	// And a second one has nothing left to spend.
	_, err = s.appeal(mgr)
	s.Require().ErrorIs(err, session.ErrCannotAfford)
}

// THE DEED IS THE SKILL (R3). A beaten Persuasion line lands the appeal, not
// the threat: a mind reads the deed, and the coward's fear must not key on
// being talked round.
func (s *SpeakWorldSuite) TestABeatenAppealLandsItsOwnDeed() {
	mgr := s.front([]int{10})

	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	s.True(s.heldBy(mgr, encounter.DeedPersuade), "the goblin remembers being talked round")
	s.False(s.heldBy(mgr, encounter.DeedIntimidate), "and was never threatened")
}

// NO SKILL JUST MEANS NO PLUS (rpg-project#569). A character who never took
// Persuasion rolls the line on one d20 like anyone else and simply adds no
// proficiency bonus — there is no untrained disadvantage.
//
// TWO SCENES, ONE DIE SCRIPT. Both checkers roll once and take 10; only the
// proficiency bonus differs.
func (s *SpeakWorldSuite) TestAnUntrainedCheckerRollsTheLinePlain() {
	s.Run("trained: one die", func() {
		s.SetupTest()
		out, err := s.appeal(s.front([]int{10, 3}))
		s.Require().NoError(err)
		s.Equal(11, out.Total, "one d20 of 10, CHA -1, proficiency +2")
		s.True(out.Beaten)
	})

	s.Run("untrained: one die, no plus", func() {
		s.SetupTest()
		s.sheet = armedFighter("alice") // no Persuasion at all
		out, err := s.appeal(s.front([]int{10, 3}))
		s.Require().NoError(err)
		s.Equal(9, out.Total, "one d20 of 10, CHA -1, no proficiency — the 3 is never thrown")
		s.True(out.Beaten, "9 meets DC 9: no skill costs the +2, not the better face")
	})
}

// The option's success rows arrive as their own beat in the story: the creature, the verb,
// the option they belong to, the die, the entry that fired and the line the
// author wrote.
func (s *SpeakWorldSuite) TestTheAnswerReachesTheStreamAsATypedBeat() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := appealLine()
		line.Success = []encounter.Answer{
			{Weight: 3, Say: "Bandits took the cellar. Go left at the rope."},
			{Weight: 1, Say: "Follow me."},
		}
		in.Options = []encounter.Option{line}
	}
	// The d20, then the world's own die: 3 + 1 weights, loaded by the
	// soldier's 100, make a d400, and a face of 2 lands in the first entry.
	mgr := s.front([]int{10, 2})

	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	var found answeredBeat
	s.Require().True(s.lastBeat(mgr, encounter.BeatAnswered, &found), "the answer reached alice's story as its own beat")
	s.Equal(answeredBeat{
		Creature: "goblin", Verb: "speak", Option: "appeal", Beaten: true,
		Roll: 2, Of: 400, Entry: 0, Word: "",
		Say: "Bandits took the cellar. Go left at the rope.", Fact: "",
		Key: string(encounter.AnswerSuccess),
		Candidates: []answerCandidate{
			{Entry: 0, Weight: 3, Percent: 100, Loaded: 300},
			{Entry: 1, Weight: 1, Percent: 100, Loaded: 100},
		},
	}, found, "the die, every eligible entry's arithmetic, and the author's line verbatim")
}

// A verdict the author wrote no rows for produces NO answer beat at all.
// Absent means absent.
func (s *SpeakWorldSuite) TestAnUnauthoredVerdictRollsNothing() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := appealLine()
		line.Failure = []encounter.Answer{{Weight: 1, Say: "Nothing down there, friend."}}
		in.Options = []encounter.Option{line}
	}
	// ONE face. An answer roll would ask for a second and fail the verb; the
	// line succeeding is the assertion that the beaten half never rolled.
	mgr := s.front([]int{10})

	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	s.Empty(beatsNamed(s.beats(mgr, "alice"), encounter.BeatAnswered), "nothing was authored for a beaten appeal")
}

// The world clock's own refusals are the turn clock's, with its sentinels.
func (s *SpeakWorldSuite) TestRefusals() {
	mgr := s.front([]int{10})

	_, err := mgr.Speak(context.Background(), nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", OptionID: "appeal"})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "nobody", OptionID: "appeal"})
	s.ErrorIs(err, session.ErrNoMember)

	_, err = s.say(mgr, "sing")
	s.ErrorIs(err, session.ErrNoOption, "a line the goblin does not carry")
}

// And the turn clock's own gate is UNCHANGED: somebody else's turn still
// refuses.
func (s *SpeakWorldSuite) TestSomebodyElsesTurnStillRefuses() {
	s.turnClock = true
	mgr := s.front([]int{10})

	authorTurnClock(s.T(), s.encounters, testSession, []string{"alice", "goblin"}, 1)

	_, err := s.appeal(mgr)
	s.ErrorIs(err, session.ErrNotYourTurn, "it is the goblin's turn")
}

// While an interrupt window is open, the Speak row is BLOCKED on the world
// clock rather than dropped: it is a row there, so leaving it out would make
// it simply vanish while a window stands.
func (s *SpeakWorldSuite) TestAFrozenFreeRoamPanelBlocksTheSpeakRowRatherThanDroppingIt() {
	// The d20, then the Guidance die the offer would spend.
	s.sheet = guidedTalker("alice")
	mgr := s.front([]int{6, 4})

	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Paused, "the checker holds a die and the machine stopped to ask")

	byVerb := declarationsByVerb(s.rows(mgr))
	row, ok := byVerb[session.VerbSpeak]
	s.Require().True(ok, "the row is still on the panel")
	s.False(row.Available, "nothing may be declared while a window stands")
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallWindowOpen, row.Why.Reason, "and the panel says WHY")
	_, hasReact := byVerb[session.VerbReact]
	s.True(hasReact, "the question itself is on the panel")

	for _, verb := range []session.Verb{
		session.VerbAttack, session.VerbMove, session.VerbActivate, session.VerbCast, session.VerbEndTurn,
	} {
		s.NotContains(byVerb, verb, "%s: never a free-roam row, so never a free-roam refusal", verb)
	}
}

// guidedTalker is [talkingFighter] already holding a Guidance die, so the
// check machine poses instead of settling.
func guidedTalker(id string) *character.Data {
	sheet := talkingFighter(id)
	condition, err := conditions.NewGuidedCondition(conditions.NewGuidedConditionInput{
		MemberID: id, SourceID: "cleric-1", SourceRef: refs.Spells.Guidance(),
	})
	if err != nil {
		panic(err)
	}
	raw, err := condition.ToJSON()
	if err != nil {
		panic(err)
	}
	sheet.Conditions = append(sheet.Conditions, raw)

	return sheet
}

// A PAUSED LINE STILL ROLLS ITS ANSWER when it resumes, and it resumes as the
// LINE that was paused: the window carries the option, and the resumed check's
// applied skill decides the deed. A resumed appeal that landed a threat would
// put the wrong deed on a mind.
func (s *SpeakWorldSuite) TestAResumedAppealFinishesAsAnAppealAndRollsItsAnswer() {
	s.sheet = guidedTalker("alice")
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := appealLine()
		line.Success = []encounter.Answer{{Weight: 1, Say: "Go left at the rope.", Fact: "bandits-in-cellar"}}
		in.Options = []encounter.Option{threatLine(), line}
	}
	// The d20 (6, which totals 7 and misses DC 9), the Guidance d4 (4, making
	// 11, which beats it), then the world's own die for the one-row answer.
	mgr := s.front([]int{6, 4, 1})

	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Paused, "the checker holds a die and the machine stopped")

	var react session.Declaration
	for _, row := range s.rows(mgr) {
		if row.Verb == session.VerbReact {
			react = row
		}
	}
	s.Require().NotEmpty(react.ID, "the question is on the panel")
	_, err = mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take(""),
	})
	s.Require().NoError(err)

	s.True(s.heldBy(mgr, encounter.DeedPersuade), "it finished as the line that was paused")
	s.False(s.heldBy(mgr, encounter.DeedIntimidate), "and never as the other one")

	var body spokeBeat
	s.Require().True(s.lastBeat(mgr, encounter.BeatSpoke, &body), "the verdict reached the log")
	s.Equal("alice", body.Actor)
	s.Equal("goblin", body.Target)
	s.Equal("appeal", body.Option)
	s.Equal("persuasion", body.Skill)
	s.Equal(9, body.DC)
	s.Equal(11, body.Total, "the offered die joined the total, and nothing was re-rolled")
	s.True(body.Beaten)
	s.Require().NotNil(body.Calculation)
	s.Equal(11, body.Calculation.Total, "and the RESUMED arithmetic rides the beat, offered die included")

	var answered answeredBeat
	s.Require().True(s.lastBeat(mgr, encounter.BeatAnswered, &answered), "and so did what the goblin did about it")
	s.Equal(answeredBeat{
		Creature: "goblin", Verb: "speak", Option: "appeal", Beaten: true,
		Roll: 1, Of: 100, Entry: 0, Word: "fact",
		Say: "Go left at the rope.", Fact: "bandits-in-cellar",
		Key:        string(encounter.AnswerSuccess),
		Candidates: []answerCandidate{{Entry: 0, Weight: 1, Percent: 100, Loaded: 100}},
	}, answered)
}

// THE RING IS PER VIEWER, and the sighting is what carries it
// (rpg-project#458). Two viewers on different sides of ONE subject get
// different stances for it out of the same read.
func (s *SpeakWorldSuite) TestTwoViewersBelieveDifferentStancesAboutOneSubject() {
	mgr := s.frontWithBandit()
	ctx := context.Background()

	stanceOf := func(viewer, subject string) string {
		sightings, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: viewer})
		s.Require().NoError(err)
		for _, sighting := range sightings.Sightings {
			if sighting.Subject == subject {
				return sighting.Stance
			}
		}
		s.Require().Fail("no sighting", "%s does not see %s", viewer, subject)

		return ""
	}

	forAlice := stanceOf("alice", "bandit")
	forGoblin := stanceOf("goblin", "bandit")

	s.Equal(string(encounter.StanceHostile), forAlice, "the party declared itself hostile to bandits")
	s.Equal(string(encounter.StanceNeutral), forGoblin, "goblins declared nothing about bandits")
	s.NotEqual(forAlice, forGoblin,
		"one subject, one read, two beliefs — the fact is about the PAIR, not the subject")
}

// A subject in NO faction — a world NPC — yields an empty stance rather than
// "neutral".
func (s *SpeakWorldSuite) TestASubjectInNoFactionHasNoStance() {
	mgr := s.front(nil)
	ctx := context.Background()

	_, err := mgr.PlaceNPC(ctx, &session.PlaceNPCInput{
		Session: "sess", Member: "innkeeper",
		NPC: merchantData(), Position: spatial.Position{X: 6, Y: 1},
	})
	s.Require().NoError(err)

	sightings, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	var found bool
	for _, sighting := range sightings.Sightings {
		if sighting.Subject != "innkeeper" {
			continue
		}
		found = true
		s.Empty(sighting.Stance, "a world NPC is on nobody's side, and empty says so")
	}
	s.Require().True(found, "alice can see the innkeeper")
}

// frontWithBandit is [SpeakWorldSuite.front] plus a third faction: bandits,
// whom the party declared itself hostile to and the goblins said nothing about.
func (s *SpeakWorldSuite) frontWithBandit() *session.Manager {
	s.characters = newFakeCharacters(s.sheet)
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: &sequenceDice{rolls: []int{10, 10, 10, 10}},
		TurnDriver: session.Pass{},
		Sessions:   s.sessions, Encounters: s.encounters,
		Characters: s.characters,
	})
	s.Require().NoError(err)

	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 5, 1)
	goblin.Faction = "goblins"
	bandit := monsterAt("bandit", refs.Monsters.Bandit().String(), 6, 1)
	bandit.Faction = "bandits"
	sc := scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Factions: []encounter.FactionInput{{ID: "goblins"}, {ID: "bandits"}},
			Dispositions: []encounter.DispositionInput{
				{
					Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
					Stance:  encounter.StanceNeutral,
				},
				{
					Between: [2]encounter.FactionID{"bandits", encounter.FactionParty},
					Stance:  encounter.StanceHostile,
				},
			},
		},
		Party:    []sceneSeat{seatAt("alice", 2, 1)},
		Monsters: []dungeonspec.MonsterPlacement{goblin, bandit},
	}
	launchScene(s.T(), mgr, sc)

	return mgr
}

// spokeBody is the last `spoke` beat in alice's story.
func (s *SpeakWorldSuite) spokeBody(mgr *session.Manager) spokeBeat {
	var body spokeBeat
	s.Require().True(s.lastBeat(mgr, encounter.BeatSpoke, &body), "the verdict reached alice's story")
	s.Require().NotZero(body.Actor)
	return body
}

// TestAnUntrainedCheckerCarriesNoRule: the line's own beat for a checker who
// never took the skill shows one d20 and no keep record — nothing named
// "Untrained" reaches the wire, because no such rule exists (rpg-project#569).
func (s *SpeakWorldSuite) TestAnUntrainedCheckerCarriesNoRule() {
	s.SetupTest()
	s.sheet = armedFighter("alice") // no Persuasion at all

	mgr := s.front([]int{10, 3})
	_, err := s.appeal(mgr)
	s.Require().NoError(err)

	body := s.spokeBody(mgr)
	die := d20Of(s.T(), body.Calculation)
	s.Equal("1d20", die.Notation, "one die, however untrained the checker is")
	s.Equal([]int{10}, die.FinalRolls)
	s.Empty(die.KeptIndices)
	s.Nil(die.Keep, "nothing touched the pool")
	s.Equal(body.Total, body.Calculation.Total)
}

// TestATrainedCheckerCarriesNoRule is the control: the same line, a character
// who took the skill. One die, and nothing recorded over it.
func (s *SpeakWorldSuite) TestATrainedCheckerCarriesNoRule() {
	s.SetupTest()

	mgr := s.front([]int{10, 3})
	_, err := s.appeal(mgr)
	s.Require().NoError(err)

	die := d20Of(s.T(), s.spokeBody(mgr).Calculation)
	s.Equal("1d20", die.Notation)
	s.Empty(die.KeptIndices)
	s.Nil(die.Keep, "nobody touched the pool, and the zero value says so")
}

// THE DIE REACHES THE ENCOUNTER, proved by driving a round of the WORLD rather
// than by looking at the wiring (rpg-project#465 §5, §6): alice speaks, which
// costs the world a round; the goblin rolls its own `time` table on the
// session's own dice.
func (s *SpeakWorldSuite) TestAWorldRoundRollsThroughTheSessionsOwnDice() {
	s.driver = session.Driver()
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		in.Table = encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}
	}
	// The d20 for the line, then the world's own die for the goblin's turn.
	// The line has no answer rows, so 42 can only be the `time` pick.
	mgr := s.front([]int{10, 42})

	_, err := s.appeal(mgr)
	s.Require().NoError(err)

	var body answeredBeat
	s.Require().True(s.lastBeat(mgr, encounter.BeatAnswered, &body), "the world thought, and a creature's pick is a beat")
	s.Equal(string(encounter.AnswerTime), body.Key, "this was time passing, not anybody speaking")
	s.Empty(body.Verb, "nothing spoke, and an empty verb is that answer rather than a gap")
	s.Empty(body.Option, "a time pick names no option")
	s.False(body.Beaten, "and there was no verdict to be beaten")
	s.Equal(42, body.Roll, "the face this test seeded, thrown through the session's own dice")
	s.Equal(100, body.Of, "one entry at weight 1, loaded by the soldier's 100")
	s.Equal([]answerCandidate{{Entry: 0, Weight: 1, Percent: 100, Loaded: 100}}, body.Candidates)
	s.Equal("hold", body.Word)
}

// A FACTION'S MIX IS DEALT AT THE DOOR, and the streamer sees which goblin
// came out the coward (rpg-project#465 §3, R5).
func (s *SpeakWorldSuite) TestAFactionsMixDealsATemperamentAndSaysSo() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		in.Temper = encounter.Temper{Mix: map[string]int{"coward": 1, "soldier": 2, "aggressive": 1}}
	}
	mgr := s.front([]int{1})

	var body temperedBeat
	s.Require().True(s.lastBeat(mgr, encounter.BeatTempered, &body), "a dealt temperament is a beat, not a private fact")
	s.Equal("goblin", body.Member)
	s.Equal("aggressive", body.Temper, "face 1 of the sorted mix")
	s.Equal(1, body.Roll)
	s.Equal(4, body.Of, "1 + 2 + 1: the author's shares are the die's faces")
	s.Equal("goblins", body.Faction)
}

// An authored WORD is never dealt for, and writes no beat.
func (s *SpeakWorldSuite) TestAnAuthoredTemperamentIsNotDealtFor() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		in.Temper = encounter.Temper{Word: "coward"}
	}
	mgr := s.front(nil)

	s.Empty(beatsNamed(s.beats(mgr, "alice"), encounter.BeatTempered), "nothing was dealt, so there is no roll to show")
}
