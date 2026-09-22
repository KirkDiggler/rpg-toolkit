// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// persuade_test.go is the second shenanigan across the whole seam
// (rpg-project#458): the twin verb, the social verbs on the WORLD clock where
// no fight exists, the untrained rule reaching a real check, and the author's
// answer table arriving as a typed beat.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type PersuadeSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters

	// sheet is the checker every scene loads; a scene that is about the
	// untrained rule swaps it before opening the room.
	sheet *character.Data

	// authored is the dungeon author's hand on the spawn, when a scene has
	// one.
	authored func(*session.SpawnInput)

	// turnClock puts alice and the goblin into one authored bubble. False —
	// the default — leaves both in FREE ROAM, which is the front room the
	// whole slice exists for.
	turnClock bool

	// driver is what drives the goblin when the world gives it time. Nil —
	// the default — is session.Pass{}, because most scenes here are about a
	// verb's own roll and a creature taking a turn in the middle of one would
	// be noise. A scene that wants the world to actually think wires
	// session.Driver().
	driver session.TurnDriver
}

func TestPersuadeSuite(t *testing.T) {
	suite.Run(t, new(PersuadeSuite))
}

func (s *PersuadeSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.sheet, s.authored, s.turnClock, s.driver = talkingFighter("alice"), nil, false, nil
	s.characters = newFakeCharacters(s.sheet)
}

// front opens a session with alice and a goblin in one hall, NEUTRAL to each
// other so no fight forms — the front room of the design, where a creature
// stands in a doorway and nobody has rolled initiative.
func (s *PersuadeSuite) front(rolls []int) *session.Manager {
	s.characters = newFakeCharacters(s.sheet)
	roller := &sequenceDice{rolls: rolls}
	driver := s.driver
	if driver == nil {
		driver = session.Pass{}
	}
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: roller, TurnDriver: driver,
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
			Factions: []encounter.FactionInput{{ID: "goblins"}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
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
		Position: spatial.Position{X: 5, Y: 1}, Faction: "goblins",
		// THE AUTHOR WROTE BOTH VERBS ON THIS GOBLIN, because after
		// rpg-project#494 a creature carries a social verb only when its
		// binding priced one — an unpriced goblin is refused before any die
		// is thrown, and every scene here is about what happens after one.
		// DC 9 is the number the retired derived approach used to produce for
		// a goblin, so every scripted die still means what its comment says.
		Intimidate: []session.DoorApproach{{Ability: "intimidation", DC: 9}},
		Persuade:   []session.DoorApproach{{Ability: "persuasion", DC: 9}},
	}
	if s.authored != nil {
		s.authored(spawn)
	}
	_, err = mgr.Spawn(ctx, spawn)
	s.Require().NoError(err)

	if s.turnClock {
		stored, err := s.encounters.GetEncounter(ctx, "world")
		s.Require().NoError(err)
		s.Require().NoError(s.encounters.SaveEncounter(ctx, "world",
			turnWorld(stored, []string{"alice", "goblin"}, 0)))
	}

	return mgr
}

func (s *PersuadeSuite) persuade(mgr *session.Manager) (*session.PersuadeOutput, error) {
	return mgr.Persuade(context.Background(), &session.PersuadeInput{
		Session: "sess", Member: "alice", Target: "goblin",
	})
}

func (s *PersuadeSuite) rows(mgr *session.Manager) []session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	return out.Declarations
}

func (s *PersuadeSuite) rowFor(mgr *session.Manager, verb session.Verb) session.Declaration {
	for _, decl := range s.rows(mgr) {
		if decl.Verb == verb {
			return decl
		}
	}
	s.Require().Fail("no row", string(verb))

	return session.Declaration{}
}

// THE HEADLINE OF THE SLICE. A player walks up to a neutral goblin in a room
// with no fight in it, talks to it, and it works — no initiative, no turn, no
// action spent, because there is no economy to spend from (R3).
func (s *PersuadeSuite) TestASocialVerbLandsOnTheWorldClock() {
	mgr := s.front([]int{10})

	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "precondition: nothing here is fighting")

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Equal(9, out.DC, "the number the goblin's own binding priced")
	s.Equal("persuasion", out.Applied.Ability)
	s.Equal(11, out.Total, "the d20's 10 with CHA 8's -1 and proficiency's +2")
	s.True(out.Beaten)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Nil(stored.ActionEconomy,
		"free roam has no economy: the verb must never ready a sheet, let alone spend from one")
}

// Both social verbs are OFFERED there too, and the panel says they cost
// nothing — the row and the door agree.
func (s *PersuadeSuite) TestBothSocialVerbsAreOfferedFreeInFreeRoam() {
	mgr := s.front([]int{10})

	verbs := make([]session.Verb, 0, 2)
	for _, decl := range s.rows(mgr) {
		verbs = append(verbs, decl.Verb)
		s.True(decl.Available, "%s: the goblin is standing right there", decl.Verb)
		s.Equal(session.SlotNone, decl.Slot, "%s: nothing is spent in free roam", decl.Verb)
		s.Nil(decl.Why, "%s: there is no budget to fall short of", decl.Verb)
		s.Require().Len(decl.Candidates, 1, "%s: one witness in the room", decl.Verb)
		s.Equal("goblin", decl.Candidates[0].Member, "%s: aimed at the witness", decl.Verb)
		s.True(decl.Candidates[0].Available, "%s: speech carries as far as sight", decl.Verb)
	}
	s.Equal([]session.Verb{session.VerbIntimidate, session.VerbPersuade}, verbs)
}

// On the turn clock it is still an action, and still the actor's own turn —
// the world clock changed what free roam offers, not what a fight costs.
func (s *PersuadeSuite) TestOnTheTurnClockItStillCostsAnAction() {
	s.turnClock = true
	mgr := s.front([]int{10})

	row := s.rowFor(mgr, session.VerbPersuade)
	s.Equal(session.SlotAction, row.Slot, "in a fight it draws from the action")
	s.True(row.Available)

	_, err := s.persuade(mgr)
	s.Require().NoError(err)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Require().NotNil(stored.ActionEconomy, "a turn-clock appeal readies and spends")
	s.Zero(stored.ActionEconomy.ActionsRemaining, "the standard action is gone")

	// And a second one has nothing left to spend.
	_, err = s.persuade(mgr)
	s.Require().ErrorIs(err, session.ErrCannotAfford)
}

// A beaten appeal lands its OWN deed, not the threat's. A mind reads the verb,
// and the coward's fear must not key on being talked round.
func (s *PersuadeSuite) TestABeatenAppealLandsItsOwnDeed() {
	mgr := s.front([]int{10})

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	s.True(s.heldBy(mgr, encounter.DeedPersuade), "the goblin remembers being talked round")
	s.False(s.heldBy(mgr, encounter.DeedIntimidate), "and was never threatened")
}

// heldBy reads whether the goblin holds a deed with this verb from alice.
func (s *PersuadeSuite) heldBy(mgr *session.Manager, verb string) bool {
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

// THE UNTRAINED RULE, at the seam that sets it (rpg-project#457 R2). A
// character who never took Persuasion rolls the verb at disadvantage, and the
// difference is visible in the total the beat reports.
//
// TWO SCENES, ONE DIE SCRIPT. The trained checker rolls once and takes 10; the
// untrained one rolls twice and takes the lower of 10 and 3. A single-die
// assertion could not tell the rule from a bad roll.
func (s *PersuadeSuite) TestAnUntrainedCheckerRollsTheVerbAtDisadvantage() {
	s.Run("trained: one die", func() {
		s.SetupTest()
		out, err := s.persuade(s.front([]int{10, 3}))
		s.Require().NoError(err)
		s.Equal(11, out.Total, "one d20 of 10, CHA -1, proficiency +2")
		s.True(out.Beaten)
	})

	s.Run("untrained: two dice, lower kept", func() {
		s.SetupTest()
		s.sheet = armedFighter("alice") // no Persuasion at all
		out, err := s.persuade(s.front([]int{10, 3}))
		s.Require().NoError(err)
		s.Equal(2, out.Total, "two d20s, the lower kept: 3 with CHA 8's -1 and no proficiency")
		s.False(out.Beaten, "the same script that beat the DC trained misses it untrained")
	})
}

// The author's table arrives as a typed beat: the creature, the verb, the die
// it was rolled with, the entry that fired and the line the author wrote.
func (s *PersuadeSuite) TestTheReactionReachesTheStreamAsATypedBeat() {
	s.authored = func(in *session.SpawnInput) {
		in.Table = encounter.Table{
			encounter.AnswerPersuaded: {
				{Weight: 3, Say: "Bandits took the cellar. Go left at the rope."},
				{Weight: 1, Say: "Follow me."},
			},
		}
	}
	// The d20, then the world's own die: 3 + 1 weights make a d4, and a face
	// of 2 lands inside the first entry's share.
	mgr := s.front([]int{10, 2})

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	var found *session.Event
	events := s.events(mgr, "alice")
	for i := range events {
		if events[i].Kind == session.EventAnswered {
			found = &events[i]
		}
	}
	s.Require().NotNil(found, "the answer reached alice's stream as its own kind")
	s.Equal(session.AnsweredBody{
		Creature: "goblin", Verb: encounter.DeedPersuade, Beaten: true,
		// THE DIE IS IN LOADED UNITS, and the candidates say why: a weight is
		// multiplied by its temperament's PERCENT before it reaches the die
		// (rpg-project#465 §3). This goblin has no temperament, which is a
		// soldier, which is 100 across the board — so 3 and 1 become 300 and
		// 100 and the die is a d400. A face of 2 still lands in the first
		// entry's share, which is why the line it picked did not change.
		Roll: 2, Of: 400, Entry: 0, Word: "",
		Say: "Bandits took the cellar. Go left at the rope.", Fact: "",
		Key: string(encounter.AnswerPersuaded),
		Candidates: []session.AnswerCandidate{
			{Entry: 0, Weight: 3, Percent: 100, Loaded: 300},
			{Entry: 1, Weight: 1, Percent: 100, Loaded: 100},
		},
	}, found.Body, "the die, every eligible entry's arithmetic, and the author's line verbatim")
}

// A verdict the author wrote no table for produces NO answer beat at all.
// Absent means absent, which is what makes an entry that fires and does
// nothing distinguishable from nothing being authored.
func (s *PersuadeSuite) TestAnUnauthoredOutcomeRollsNothing() {
	s.authored = func(in *session.SpawnInput) {
		in.Table = encounter.Table{
			encounter.AnswerPersuadeFailed: {{Weight: 1, Say: "Nothing down there, friend."}},
		}
	}
	// ONE face. An answer roll would ask for a second and fail the verb; the
	// appeal succeeding is the assertion that the beaten half never rolled.
	mgr := s.front([]int{10})

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	for _, event := range s.events(mgr, "alice") {
		s.NotEqual(session.EventAnswered, event.Kind, "nothing was authored for a beaten appeal")
	}
}

// The refusals are the threat's, with the threat's sentinels.
func (s *PersuadeSuite) TestRefusals() {
	mgr := s.front([]int{10})

	_, err := mgr.Persuade(context.Background(), nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = mgr.Persuade(context.Background(), &session.PersuadeInput{Session: "sess", Member: "alice"})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = mgr.Persuade(context.Background(), &session.PersuadeInput{
		Session: "sess", Member: "alice", Target: "nobody"})
	s.ErrorIs(err, session.ErrNoMember)
}

// events reads one member's delivered stream.
func (s *PersuadeSuite) events(mgr *session.Manager, member string) []session.Event {
	story, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	return story
}

// INTIMIDATE GOT THE SAME DOOR, and this scene is why the change is a ruling
// and not a convenience: the verb used to refuse a world-clock actor outright
// with ErrNotYourTurn, on the reasoning that it costs an action and an action
// belongs to a turn. R3 (rpg-project#457) ruled the other way — a front room
// has no turn to be out of.
func (s *PersuadeSuite) TestIntimidateAlsoLandsOnTheWorldClock() {
	mgr := s.front([]int{10})

	out, err := mgr.Intimidate(context.Background(), &session.IntimidateInput{
		Session: "sess", Member: "alice", Target: "goblin",
	})
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.Equal(9, out.DC)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Nil(stored.ActionEconomy, "free roam charged nothing, exactly as Move does there")
}

// And the turn clock's own gate is UNCHANGED: somebody else's turn still
// refuses, which is the half of the old rule that was always right.
func (s *PersuadeSuite) TestSomebodyElsesTurnStillRefuses() {
	s.turnClock = true
	mgr := s.front([]int{10})

	stored, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	s.Require().NoError(s.encounters.SaveEncounter(context.Background(), "world",
		turnWorld(stored, []string{"alice", "goblin"}, 1)))

	_, err = s.persuade(mgr)
	s.ErrorIs(err, session.ErrNotYourTurn, "it is the goblin's turn")
}

// While an interrupt window is open, the social rows are BLOCKED on the world
// clock rather than dropped.
//
// Before R3 a frozen free-roam panel was the window alone and that was the
// whole truth — the economy verbs were never rows there, so reporting a
// refusal for them would have been reporting a refusal for nothing. The social
// verbs ARE rows there now, so leaving them out would make two rows simply
// vanish while a window stands, which is the one thing that panel exists to
// prevent.
func (s *PersuadeSuite) TestAFrozenFreeRoamPanelBlocksTheSocialRowsRatherThanDroppingThem() {
	// The d20, then the Guidance die the offer would spend.
	s.sheet = guidedTalker("alice")
	mgr := s.front([]int{6, 4})

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Paused, "the checker holds a die and the machine stopped to ask")

	byVerb := declarationsByVerb(s.rows(mgr))
	for _, verb := range []session.Verb{session.VerbIntimidate, session.VerbPersuade} {
		row, ok := byVerb[verb]
		s.Require().True(ok, "%s: the row is still on the panel", verb)
		s.False(row.Available, "%s: nothing may be declared while a window stands", verb)
		s.Require().NotNil(row.Why)
		s.Equal(session.ShortfallWindowOpen, row.Why.Reason,
			"%s: and the panel says WHY, rather than the row disappearing", verb)
	}
	_, hasReact := byVerb[session.VerbReact]
	s.True(hasReact, "the question itself is on the panel")

	// And no turn-economy verb is reported there: those were never rows in
	// free roam, and a refusal for a row that never existed is noise.
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

// A PAUSED VERB STILL ROLLS THE CREATURE'S ANSWER when it resumes, and it
// resumes as the verb that was PAUSED.
//
// The window carries which social verb it stopped: while Intimidate was the
// only one, "a target and no door" named it unambiguously; with two, a resumed
// Persuade that landed an Intimidate would put the wrong deed on a mind — the
// coward would take fear from a conversation it was talked round by. This
// scene pauses a Persuade, answers it, and reads back the deed, the beat and
// the answer the author wrote.
func (s *PersuadeSuite) TestAResumedAppealFinishesAsAnAppealAndRollsItsReaction() {
	s.sheet = guidedTalker("alice")
	s.authored = func(in *session.SpawnInput) {
		in.Table = encounter.Table{
			encounter.AnswerPersuaded: {{Weight: 1, Say: "Go left at the rope.", Fact: "bandits-in-cellar"}},
		}
	}
	// The d20 (6, which totals 7 and misses DC 9), the Guidance d4 (4, making
	// 11, which beats it), then the world's own die for the one-entry table.
	mgr := s.front([]int{6, 4, 1})

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Paused, "the checker holds a die and the machine stopped")

	rows := s.rows(mgr)
	var react session.Declaration
	for _, row := range rows {
		if row.Verb == session.VerbReact {
			react = row
		}
	}
	s.Require().NotEmpty(react.ID, "the question is on the panel")
	_, err = mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Choice: session.ReactStrike,
	})
	s.Require().NoError(err)

	s.True(s.heldBy(mgr, encounter.DeedPersuade), "it finished as the verb that was paused")
	s.False(s.heldBy(mgr, encounter.DeedIntimidate), "and never as the other one")

	var persuaded, answered *session.Event
	events := s.events(mgr, "alice")
	for i := range events {
		switch events[i].Kind {
		case session.EventPersuaded:
			persuaded = &events[i]
		case session.EventAnswered:
			answered = &events[i]
		}
	}
	s.Require().NotNil(persuaded, "the verdict reached the log")
	body, ok := persuaded.Body.(session.PersuadedBody)
	s.Require().True(ok)
	s.Equal("alice", body.Actor)
	s.Equal("goblin", body.Target)
	s.Equal(9, body.DC)
	s.Equal(11, body.Total, "the offered die joined the total, and nothing was re-rolled")
	s.True(body.Beaten)
	s.Require().NotNil(body.Calculation)
	s.Equal(11, body.Calculation.Total,
		"and the RESUMED arithmetic rides the beat, offered die included")

	s.Require().NotNil(answered, "and so did what the goblin did about it")
	s.Equal(session.AnsweredBody{
		Creature: "goblin", Verb: encounter.DeedPersuade, Beaten: true,
		Roll: 1, Of: 100, Entry: 0, Word: "fact",
		Say: "Go left at the rope.", Fact: "bandits-in-cellar",
		Key:        string(encounter.AnswerPersuaded),
		Candidates: []session.AnswerCandidate{{Entry: 0, Weight: 1, Percent: 100, Loaded: 100}},
	}, answered.Body)
}

// THE RING IS PER VIEWER, and the sighting is what carries it
// (rpg-project#458). Two viewers on different sides of ONE subject get
// different stances for it out of the same read, which is what makes `pretend`
// a later change to one function rather than a rewrite of this projection.
func (s *PersuadeSuite) TestTwoViewersBelieveDifferentStancesAboutOneSubject() {
	// A third faction the party is HOSTILE to and the goblins are merely
	// neutral to, so alice and the goblin looking at the same bandit disagree.
	mgr := s.frontWithBandit()
	ctx := context.Background()

	stanceOf := func(viewer, subject string) string {
		sightings, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: viewer})
		s.Require().NoError(err)
		for _, sighting := range sightings {
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
// "neutral". Nobody is on their side and nobody is against them, and an empty
// string says the run had no pair to answer about instead of inventing one.
func (s *PersuadeSuite) TestASubjectInNoFactionHasNoStance() {
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
	for _, sighting := range sightings {
		if sighting.Subject != "innkeeper" {
			continue
		}
		found = true
		s.Empty(sighting.Stance, "a world NPC is on nobody's side, and empty says so")
	}
	s.Require().True(found, "alice can see the innkeeper")
}

// frontWithBandit is [PersuadeSuite.front] plus a third faction: bandits, whom
// the party declared itself hostile to and the goblins said nothing about. It
// is the smallest world in which two viewers disagree about one subject.
func (s *PersuadeSuite) frontWithBandit() *session.Manager {
	s.characters = newFakeCharacters(s.sheet)
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: &sequenceDice{rolls: []int{10, 10, 10, 10}},
		TurnDriver: session.Pass{},
		Sessions:   s.sessions, Encounters: s.encounters,
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
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "goblin", Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 1},
				Faction: "goblins"},
			{ID: "bandit", Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 1},
				Faction: "bandits"},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	return mgr
}

// TestTheUntrainedRuleReachesTheWireByName is the done-when of
// rpg-project#462, at the seam that used to lose it. The rule shipped applied
// but INVISIBLE: it kept the lower of two faces and the beat said one number,
// so a player could not tell a house rule from a bad roll. Both faces, the
// kept one, and the word "Untrained" now cross typed.
func (s *PersuadeSuite) TestTheUntrainedRuleReachesTheWireByName() {
	s.SetupTest()
	s.sheet = armedFighter("alice") // no Persuasion at all

	mgr := s.front([]int{10, 3})
	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().False(out.Beaten)

	var body session.PersuadedBody
	for _, event := range s.events(mgr, "alice") {
		if persuaded, ok := event.Body.(session.PersuadedBody); ok {
			body = persuaded
		}
	}
	s.Require().NotZero(body.Actor, "the verdict reached alice's stream typed")

	die := d20Of(s.T(), body.Calculation)
	s.Equal("2d20", die.Notation, "the pair the rule actually threw")
	s.Equal([]int{10, 3}, die.FinalRolls, "and the discarded 10 survives to the log")
	s.Equal([]int{1}, die.KeptIndices)
	s.Require().NotNil(die.Keep)
	s.Equal(session.KeepDisadvantage, die.Keep.Rule)
	s.Require().Len(die.Keep.Imposed, 1)
	s.Equal("Untrained", die.Keep.Imposed[0].Name,
		"the word the log prints comes down from the server, never invented by a client")
	s.Equal("alice", die.Keep.Imposed[0].SourceID)
	s.Empty(die.Keep.Granted)
	s.Equal(body.Total, body.Calculation.Total)
}

// TestATrainedCheckerCarriesNoRule is the control: the same verb, the same
// seam, a character who took the skill. One die, and nothing recorded over it.
// Without this, "Keep is set" could just mean "Keep is always set".
func (s *PersuadeSuite) TestATrainedCheckerCarriesNoRule() {
	s.SetupTest()

	mgr := s.front([]int{10, 3})
	_, err := s.persuade(mgr)
	s.Require().NoError(err)

	var body session.PersuadedBody
	for _, event := range s.events(mgr, "alice") {
		if persuaded, ok := event.Body.(session.PersuadedBody); ok {
			body = persuaded
		}
	}
	s.Require().NotZero(body.Actor)

	die := d20Of(s.T(), body.Calculation)
	s.Equal("1d20", die.Notation)
	s.Empty(die.KeptIndices)
	s.Nil(die.Keep, "nobody touched the pool, and the zero value says so")
}

// THE DIE REACHES THE ENCOUNTER, proved by driving a round of the WORLD rather
// than by looking at the wiring (rpg-project#465 §5, §6).
//
// The composition's Roller is optional at its door and refused loudly at the
// roll ([encounter.ErrNoRoller]), which is the right shape — a scene with no
// table and no mix rolls nothing, and requiring a die at every door would make
// every caller declare one it never uses. What it means for this seam is that a
// write verb that forgot to hand one over would not fail at construction: it
// would fail on the first creature given time, in production, in the middle of
// somebody's verb.
//
// So this drives the whole path. Alice speaks to the goblin, which is an action
// and costs the world a round; the world thinks at that raise and gives every
// standing creature on the world clock one turn's worth; the goblin rolls its
// own `time` table. The face on the beat is the one this test seeded, which no
// arrangement of the wiring can produce by accident.
func (s *PersuadeSuite) TestAWorldRoundRollsThroughTheSessionsOwnDice() {
	// An unconditional `hold` is the whole table, deliberately: the point is
	// that a die was thrown and whose faces it landed on, not what the creature
	// decided. One entry at weight 1, loaded by the soldier's 100, is a d100.
	s.driver = session.Driver()
	s.authored = func(in *session.SpawnInput) {
		in.Table = encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}
	}
	// The d20 for the appeal, then the world's own die for the goblin's turn.
	// Nothing is authored for either verdict, so the appeal itself rolls no
	// answer and 42 can only be the `time` pick.
	mgr := s.front([]int{10, 42})

	_, err := s.persuade(mgr)
	s.Require().NoError(err)

	var answered *session.Event
	for _, event := range s.events(mgr, "alice") {
		if event.Kind == session.EventAnswered {
			answered = &event
		}
	}
	s.Require().NotNil(answered, "the world thought, and a creature's pick is a beat")

	body, ok := answered.Body.(session.AnsweredBody)
	s.Require().True(ok)
	s.Equal(string(encounter.AnswerTime), body.Key, "this was time passing, not anybody speaking")
	s.Empty(body.Verb, "nothing spoke, and an empty verb is that answer rather than a gap")
	s.False(body.Beaten, "and there was no verdict to be beaten")
	s.Equal(42, body.Roll, "the face this test seeded, thrown through the session's own dice")
	s.Equal(100, body.Of, "one entry at weight 1, loaded by the soldier's 100")
	s.Equal([]session.AnswerCandidate{{Entry: 0, Weight: 1, Percent: 100, Loaded: 100}}, body.Candidates)
	s.Equal("hold", body.Word)
}

// A FACTION'S MIX IS DEALT AT THE DOOR, and the streamer sees which goblin
// came out the coward (rpg-project#465 §3, R5).
//
// Four goblins off one sheet with one table are four different creatures
// because their dice are loaded differently, not because they were given
// different orders. The deal has to be visible for that to be a story rather
// than an accident, which is what this beat is for — and it is the second
// place the session's shared dice have to have reached the composition, the
// first being the picks themselves.
func (s *PersuadeSuite) TestAFactionsMixDealsATemperamentAndSaysSo() {
	s.authored = func(in *session.SpawnInput) {
		// The design's own example spread. Walked in sorted order by the
		// composition so a seeded roller deals the same word every run:
		// aggressive 1, coward 1, soldier 2 — a d4 whose first face is the
		// aggressive one.
		in.Temper = encounter.Temper{Mix: map[string]int{"coward": 1, "soldier": 2, "aggressive": 1}}
	}
	// The one face the deal needs, spent at Spawn. Nothing else rolls: the
	// appeal below never happens.
	mgr := s.front([]int{1})

	var dealt *session.Event
	for _, event := range s.events(mgr, "alice") {
		if event.Kind == session.EventTempered {
			dealt = &event
		}
	}
	s.Require().NotNil(dealt, "a dealt temperament is a beat, not a private fact")

	body, ok := dealt.Body.(session.TemperedBody)
	s.Require().True(ok)
	s.Equal("goblin", body.Member)
	s.Equal("aggressive", body.Temper, "face 1 of the sorted mix")
	s.Equal(1, body.Roll)
	s.Equal(4, body.Of, "1 + 2 + 1: the author's shares are the die's faces")
	s.Equal("goblins", body.Faction,
		"the die belonged to the FACTION — the spread is the instructions given to the group")
}

// An authored WORD is never dealt for, and writes no beat: the author already
// answered the question the mix exists to ask.
func (s *PersuadeSuite) TestAnAuthoredTemperamentIsNotDealtFor() {
	s.authored = func(in *session.SpawnInput) {
		in.Temper = encounter.Temper{Word: "coward"}
	}
	// NO FACES AT ALL. A deal would ask for one and fail the spawn, which is
	// the assertion: nothing was rolled.
	mgr := s.front(nil)

	for _, event := range s.events(mgr, "alice") {
		s.NotEqual(session.EventTempered, event.Kind,
			"nothing was dealt, so there is no roll to show")
	}
}
