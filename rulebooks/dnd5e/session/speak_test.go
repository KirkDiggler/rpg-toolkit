// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// speak_test.go is the one social verb across the whole seam (rpg-project#569):
// an authored line on a creature, the check it rolls, the price on each clock,
// the refusals, the deed its skill lands on the creature's own mind, and the
// offer that lists only what THIS speaker may say.
//
// The scenes are the ones Intimidate and Persuade proved (rpg-project#454,
// rpg-project#458), carried onto the options that replaced them: a threat is
// now a line whose check rolls Intimidation, and an appeal one whose check
// rolls Persuasion.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// threatLine is the line every threat scene authors: Intimidation against DC
// 9, the number the retired derived approach used to produce for a goblin —
// 10 plus its WIS 8 — so every scripted die still means what its comment says.
func threatLine() encounter.Option {
	return encounter.Option{
		ID: "threaten", Say: "Drop the bow or I drop you.",
		Check: []encounter.CheckApproach{{Ability: "intimidation", DC: 9}},
	}
}

// appealLine is [threatLine]'s twin on Persuasion.
func appealLine() encounter.Option {
	return encounter.Option{
		ID: "appeal", Say: "We are not here for you. Where are the bandits?",
		Check: []encounter.CheckApproach{{Ability: "persuasion", DC: 9}},
	}
}

type SpeakSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters

	// authored, when set, is the dungeon author's hand on the spawn — what
	// dungeonspec's MonsterPlacement compiles to and a host forwards. Every
	// scene starts from the goblin carrying [threatLine]; a scene that wants
	// something else rewrites it here.
	authored func(*dungeonspec.MonsterPlacement)
}

func TestSpeakSuite(t *testing.T) {
	suite.Run(t, new(SpeakSuite))
}

func (s *SpeakSuite) SetupTest() {
	s.sessions, s.encounters, s.authored = newFakeSessions(), newFakeEncounters(), nil
	// ALICE IS PROFICIENT IN INTIMIDATION, and every scene in this suite
	// depends on its +2.
	//
	// CHA 8 is a -1 modifier and proficiency is +2, so a d20 of 10 totals 11
	// — over the goblin's DC of 9. A d20 of 5 totals 6 and does not.
	s.characters = newFakeCharacters(talkingFighter("alice"))
}

// aYard opens a session with alice alone in an 8x8 hall, then spawns a goblin
// four cells off. Props, when given, stand in the hall.
func (s *SpeakSuite) aYard(rolls []int, props ...encounter.PropInput) *session.Manager {
	return s.aYardDriven(session.Pass{}, rolls, props...)
}

// aYardDriven is [SpeakSuite.aYard] with the monster's turn driven by a real
// brain instead of passed.
func (s *SpeakSuite) aYardDriven(
	driver session.TurnDriver, rolls []int, props ...encounter.PropInput,
) *session.Manager {
	// Two initiative rolls first: spawning the goblin in the open forms a
	// fight, and initiative is rolled before any check this suite asserts.
	roller := &sequenceDice{rolls: append([]int{10, 10}, rolls...)}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: roller,
		TurnDriver: driver,
		Sessions:   s.sessions, Encounters: s.encounters,
		Characters: s.characters,
	})
	s.Require().NoError(err)

	sc := scene{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Props:   props,
		},
		Party: []sceneSeat{seatAt("alice", 1, 1)},
	}

	ctx := context.Background()
	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 5, 1)
	goblin.Options = []encounter.Option{threatLine()}
	if s.authored != nil {
		s.authored(&goblin)
	}
	sc.Monsters = []dungeonspec.MonsterPlacement{goblin}
	launchScene(s.T(), mgr, sc)

	// THE CLOCK IS AUTHORED, not rolled for: a wall between the two means no
	// fight forms on sight at all, and every scene here needs alice on the
	// turn clock so the refusal under test is the one it says it is.
	authorTurnClock(s.T(), s.encounters, testSession, []string{"alice", "goblin"}, 0)

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "alice is on the turn clock")

	return mgr
}

// say speaks one of the goblin's lines as alice.
func (s *SpeakSuite) say(mgr *session.Manager, option string) (*session.SpeakOutput, error) {
	return mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "goblin", OptionID: option,
	})
}

func (s *SpeakSuite) threaten(mgr *session.Manager) (*session.SpeakOutput, error) {
	return s.say(mgr, "threaten")
}

// held reads whether the goblin holds a deed with this verb from alice.
func (s *SpeakSuite) held(mgr *session.Manager, verb string) bool {
	return deedHeld(s.T(), mgr, "goblin", "alice", verb)
}

func (s *SpeakSuite) row(mgr *session.Manager) session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, d := range out.Declarations {
		if d.Verb == session.VerbSpeak {
			return d
		}
	}
	s.Require().Fail("no Speak row")
	return session.Declaration{}
}

// The DC is the author's and only ever the author's: this goblin's line priced
// DC 9, and that is the number the check beats.
func (s *SpeakSuite) TestTheDifficultyIsTheAuthoredOne() {
	out, err := s.threaten(s.aYard([]int{10}))
	s.Require().NoError(err)

	s.Equal(9, out.DC, "the number the line priced")
	s.Equal("intimidation", out.Applied.Ability, "the one authored route")
	s.Equal(11, out.Total, "the d20's 10 with CHA 8's -1 and proficiency's +2")
	s.True(out.Beaten, "a total that meets the DC beats it")
	s.Equal("goblin", out.Target)
	s.Equal("threaten", out.OptionID)
	s.Empty(out.Prop)
}

// A beaten Intimidation line lands the threat on the goblin's own mind: the
// deed is the SKILL's (R3), and what happens next is the creature's.
func (s *SpeakSuite) TestABeatenThreatLandsTheDeed() {
	mgr := s.aYard([]int{10})
	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.True(s.held(mgr, encounter.DeedIntimidate), "the goblin remembers who threatened it")
	s.False(s.held(mgr, encounter.DeedPersuade), "and the line rolled no Persuasion")
}

// A missed line lands nothing. It is not an error — the die was rolled, the
// table saw it, and nothing reached the goblin.
func (s *SpeakSuite) TestAMissedThreatLandsNothing() {
	mgr := s.aYard([]int{5})
	out, err := s.threaten(mgr)
	s.Require().NoError(err)

	s.False(out.Beaten)
	s.Equal(6, out.Total, "5 with CHA 8's -1 and proficiency's +2")
	s.Equal(9, out.DC)
	s.False(s.held(mgr, encounter.DeedIntimidate), "nothing for the mind to read")
}

// The line the world currently carries is the one that is beaten — an author
// who repriced it repriced the check, and nothing else contributes a route
// beside theirs.
func (s *SpeakSuite) TestTheCurrentAuthoredLineIsWhatIsBeaten() {
	mgr := s.aYard([]int{10})

	stored, err := s.encounters.GetEncounter(context.Background(), testSession)
	s.Require().NoError(err)
	repriced := false
	for i := range stored.Members {
		if stored.Members[i].ID != "goblin" {
			continue
		}
		for j := range stored.Members[i].Options {
			if stored.Members[i].Options[j].ID == "threaten" {
				stored.Members[i].Options[j].Check = []encounter.CheckApproachData{{Ability: "intimidation", DC: 18}}
				repriced = true
			}
		}
	}
	s.Require().True(repriced, "precondition: the stored goblin carries the line")
	s.Require().NoError(s.encounters.SaveEncounter(context.Background(), testSession, stored))

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Equal(18, out.DC, "the author's new number, not the one the spawn carried")
	s.False(out.Beaten, "an 11 does not reach 18")
}

// On the turn clock the action is spent. A second line in the same turn is
// refused, and the row Afford shows says so before the caller tries.
func (s *SpeakSuite) TestOnTheTurnClockTheActionIsSpent() {
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
	s.NotEmpty(row.SpeakOptions, "the menu is the line's; the budget is the row's")
}

// Refused outside the actor's turn, exactly as a swing is: this costs an
// action, and an action belongs to a turn.
func (s *SpeakSuite) TestRefusedOffTurn() {
	mgr := s.aYard([]int{10})
	_, err := mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "goblin", Target: "alice", OptionID: "threaten",
	})
	s.Require().ErrorIs(err, session.ErrNotYourTurn)
}

// Refused when the target cannot see the actor. A wall between them means the
// goblin never heard the line — and nothing is written or charged.
func (s *SpeakSuite) TestRefusedThroughAWall() {
	mgr := s.aYard([]int{10}, occludingProps(
		spatial.Position{X: 3, Y: 0}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 3, Y: 2},
	)...)

	_, err := s.threaten(mgr)
	// THE SEAM'S OWN SENTINEL. A raw encounter error reaching a host is the
	// S2 leak.
	s.Require().ErrorIs(err, session.ErrUnwitnessed)
	s.Require().NotErrorIs(err, encounter.ErrUnwitnessed, "the composition's error stays inside")
	s.False(s.held(mgr, encounter.DeedIntimidate))

	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Nil(stored.ActionEconomy, "a refusal must not cost the actor their turn")
}

// A line needs a speaker, exactly one thing to say it to, and a name; a member
// the encounter does not have is refused rather than rolled for.
func (s *SpeakSuite) TestRefusals() {
	mgr := s.aYard([]int{10})
	ctx := context.Background()

	_, err := mgr.Speak(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Target: "goblin", OptionID: "threaten"})
	s.ErrorIs(err, session.ErrNoMemberID, "nobody speaking")

	_, err = mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", OptionID: "threaten"})
	s.ErrorIs(err, session.ErrNoMemberID, "spoken to nothing")

	_, err = mgr.Speak(ctx, &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "goblin", Prop: "altar", OptionID: "threaten",
	})
	s.ErrorIs(err, session.ErrNoMemberID, "spoken to two things at once")

	_, err = mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "goblin"})
	s.ErrorIs(err, session.ErrNoOption, "no line named")

	_, err = mgr.Speak(ctx, &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "nobody", OptionID: "threaten",
	})
	s.ErrorIs(err, session.ErrNoMember)

	_, err = mgr.Speak(ctx, &session.SpeakInput{
		Session: "sess", Member: "alice", Prop: "nothing-here", OptionID: "threaten",
	})
	s.ErrorIs(err, session.ErrNoProp, "a prop id that names nothing is the probe law's answer")
}

// Speaking to another PLAYER is refused by the same law every creature is
// judged by: nobody authored a line on a party member, so there is nothing to
// say.
func (s *SpeakSuite) TestAPlayerOffersNoLines() {
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr := s.aYard([]int{10})

	_, err := mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "bob", Position: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)

	_, err = mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "bob", OptionID: "threaten",
	})
	s.Require().ErrorIs(err, session.ErrNoOption)
}

// beats is every beat word the member's own story carries, in order.
func (s *SpeakSuite) beats(mgr *session.Manager, member string) []string {
	return beatNames(s.events(mgr, member))
}

func (s *SpeakSuite) endTurn(mgr *session.Manager, member string) {
	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: member,
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", member),
	})
	s.Require().NoError(err)
}

// events is the beats the member's own story carries, in order.
func (s *SpeakSuite) events(mgr *session.Manager, member string) []witnessed {
	return storyBeats(s.T(), mgr, "sess", member)
}

// spokePayload is the spoke beat's payload, as the encounter wrote it.
type spokePayload struct {
	Actor       string                   `json:"actor"`
	Target      string                   `json:"target"`
	Prop        string                   `json:"prop"`
	OptionID    string                   `json:"option"`
	Line        string                   `json:"line"`
	Skill       string                   `json:"skill"`
	DC          int                      `json:"dc"`
	Total       int                      `json:"total"`
	Beaten      bool                     `json:"beaten"`
	Calculation *session.RollCalculation `json:"calculation"`
}

// answeredPayload is the answered beat's payload, as the encounter wrote it.
type answeredPayload struct {
	Creature string `json:"creature"`
	Prop     string `json:"prop"`
	OptionID string `json:"option"`
	Say      string `json:"say"`
}

// lastSpoke is the last spoke beat in the member's story, decoded, or nil.
func (s *SpeakSuite) lastSpoke(mgr *session.Manager, member string) *spokePayload {
	beats := beatsNamed(s.events(mgr, member), encounter.BeatSpoke)
	if len(beats) == 0 {
		return nil
	}
	var body spokePayload
	beats[len(beats)-1].decode(s.T(), &body)
	return &body
}

// THE BEAT IS THE ONLY ACCOUNT OF THE ROLL: a beaten line's spoke beat
// carries the line, the skill, the numbers and the roll behind them, and the
// creature spoken to witnesses it too.
func (s *SpeakSuite) TestABeatenLineIsRecorded() {
	mgr := s.aYard([]int{10})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	body := s.lastSpoke(mgr, "alice")
	s.Require().NotNil(body, "the line reached alice's story")
	s.Equal("alice", body.Actor)
	s.Equal("goblin", body.Target)
	s.Empty(body.Prop)
	s.Equal("threaten", body.OptionID)
	s.Equal("Drop the bow or I drop you.", body.Line, "the author's line, verbatim")
	s.Equal("intimidation", body.Skill, "the skill the deed was decided by")
	s.Equal(9, body.DC)
	s.Equal(11, body.Total, "the numbers the response reported, on the wire")
	s.True(body.Beaten)

	// AND THE ROLL BEHIND THE NUMBER (rpg-project#462).
	die := d20Of(s.T(), body.Calculation)
	s.Equal(20, die.DieSize)
	s.Equal(11, body.Calculation.Total)

	// The goblin heard it too — the audience is the witnesses.
	s.NotEmpty(beatsNamed(s.events(mgr, "goblin"), encounter.BeatSpoke),
		"the goblin was spoken to and its own story says so")
}

// The missed line is the case that matters most: with no failure rows nothing
// else in the run records it, so a body carrying beaten:false IS the outcome.
func (s *SpeakSuite) TestAMissedLineSurfacesToo() {
	mgr := s.aYard([]int{5})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().False(out.Beaten)

	body := s.lastSpoke(mgr, "alice")
	s.Require().NotNil(body, "a missed line left no spoke beat, which is the whole outcome lost")
	s.Equal("goblin", body.Target)
	s.Equal(9, body.DC)
	s.Equal(6, body.Total, "the roll the table saw, and the only record of it")
	s.False(body.Beaten)
	s.Require().NotNil(body.Calculation)
	s.Equal(6, body.Calculation.Total, "a missed line carries its roll too — full data until v1.0")
}

// where the goblin stands now.
func (s *SpeakSuite) goblinAt(mgr *session.Manager) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: "goblin"})
	s.Require().NoError(err)
	return out.Position
}

// cowingLine is [threatLine] with the author's answer on success: run.
func cowingLine() encounter.Option {
	line := threatLine()
	line.Success = []encounter.Answer{{Weight: 1, Flee: true}}
	return line
}

// The acceptance case, end to end: the fighter threatens the archer goblin,
// beats its DC, and the goblin spends its whole next turn running away from
// her instead of opening with the bow it is holding.
//
// WHAT DECIDES IT IS TWO AUTHORED LINES: the option's success row says `flee`,
// so the beaten line lands a `fled` deed; the rulebook's own default table
// answers `away: actor` while that deed is fresh, so the running happens on
// the goblin's OWN time.
func (s *SpeakSuite) TestACowedGoblinRunsInsteadOfShooting() {
	s.authored = func(in *dungeonspec.MonsterPlacement) { in.Options = []encounter.Option{cowingLine()} }
	mgr := s.aYardDriven(session.Driver(), []int{10, 1, 1})

	s.Require().Equal(spatial.Position{X: 5, Y: 1}, s.goblinAt(mgr))

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	s.endTurn(mgr, "alice")
	s.Equal(spatial.Position{X: 4, Y: 7}, s.goblinAt(mgr),
		"its whole movement spent putting the hall between it and her")
}

// And the control, one die different: a missed line lands nothing, so the same
// goblin on the same board stands exactly where it was and shoots.
func (s *SpeakSuite) TestAnUncowedGoblinStandsAndShoots() {
	// The d20 that misses, then the goblin's own `time` roll on its turn (a
	// face of 1 lands in `attack: enemy`'s share), then the swing's own two
	// faces, then 200 of 200 lands on `hold` and the turn closes. NO FAILURE
	// ROWS are authored, so the only die between the check and the bow is
	// the one that chose the bow.
	s.authored = func(in *dungeonspec.MonsterPlacement) { in.Options = []encounter.Option{cowingLine()} }
	mgr := s.aYardDriven(session.Driver(), []int{5, 1, 8, 3, 200})

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

// A CORNERED COWARD SPENDS ITS TURN RUNNING AND DOES NOT SHOOT: `away` is a
// whole turn spent going somewhere, terminal by construction (rpg-project#465
// §7). Pinned because a walk will see it.
func (s *SpeakSuite) TestACorneredCowardSpendsItsTurnRunning() {
	s.authored = func(in *dungeonspec.MonsterPlacement) { in.Options = []encounter.Option{cowingLine()} }
	mgr := s.aYardDriven(session.Driver(), []int{10, 1, 1})

	_, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.endTurn(mgr, "alice")

	s.Require().Equal(spatial.Position{X: 4, Y: 7}, s.goblinAt(mgr),
		"it ran as far from her as the hall allows")
	for _, beat := range s.beats(mgr, "alice") {
		s.NotContains([]string{"struck", "missed"}, beat,
			"the run was the whole turn — `away` is terminal")
	}
}

// guide hands alice a Guidance die on her stored sheet — GuidedUnlockSuite's
// own pattern, for a line instead of a lock.
func (s *SpeakSuite) guide() {
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

func (s *SpeakSuite) answer(mgr *session.Manager, choice session.Answer) {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, row := range out.Declarations {
		if row.Verb != session.VerbReact {
			continue
		}
		_, err := mgr.React(context.Background(), &session.ReactInput{
			Session: "sess", Member: "alice", DeclarationID: row.ID, Answer: choice,
		})
		s.Require().NoError(err)
		return
	}
	s.Require().Fail("no open window for alice")
}

// THE POSE WINDOW STILL FIRES ON A LINE, end to end: a line spoken by somebody
// holding a die that can join a check stops after the d20 and asks them, and
// the answer finishes the same attempt — keyed by the option the window
// carries. Nothing is re-rolled.
//
// 6 on the d20 with CHA 8's -1 and proficiency's +2 is a 7, which misses the
// goblin's 9; the d4's own 4 makes it 11, which beats it.
func (s *SpeakSuite) TestAGuidedLineStopsAndAsks() {
	mgr := s.aYard([]int{6, 4})
	s.guide()

	out, err := s.threaten(mgr)
	s.Require().NoError(err)

	// ROLL AND THE PRE-OFFER TOTAL, and nothing else — Unlock's paused shape.
	// Asserted as the WHOLE output, because the claim is that nothing else
	// is carried.
	s.True(out.Paused, "the attempt is waiting on an answer")
	s.Require().NotNil(out.Roll)
	s.Equal(6, *out.Roll, "the d20 the player is deciding about")
	s.Equal(session.SpeakOutput{
		Paused: true, Total: 7, Roll: out.Roll, Target: "goblin", OptionID: "threaten",
		Saved: out.Saved, Delivery: out.Delivery,
	}, *out, "the pre-offer total, and no DC, no applied route and no verdict")
	s.False(s.held(mgr, encounter.DeedIntimidate), "and nothing has reached the goblin")

	// THE ACTION IS ALREADY GONE, read off the stored sheet: while a window
	// is open every row is blocked with ShortfallWindowOpen, so the panel
	// cannot tell a spent action from a frozen one. The economy can.
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Require().NotNil(stored.ActionEconomy)
	s.Equal(0, stored.ActionEconomy.ActionsRemaining, "charged before it rolled")

	s.answer(mgr, session.Take(""))
	s.True(s.held(mgr, encounter.DeedIntimidate),
		"7 + a d4's own 4 beats DC 9, and the resumed line landed its skill's deed")

	spoke := s.lastSpoke(mgr, "alice")
	s.Require().NotNil(spoke, "the resumed verdict reached the log")
	s.Equal("threaten", spoke.OptionID, "the window carried the line it stopped")
	s.Equal("intimidation", spoke.Skill)
	s.Equal(11, spoke.Total, "the offered die joined the total, and nothing was re-rolled")
}

// The other branch: keeping the die finishes the same attempt without it,
// and the line falls short.
func (s *SpeakSuite) TestAGuidedLineKeptFallsShort() {
	mgr := s.aYard([]int{6, 4})
	s.guide()

	_, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.answer(mgr, session.Decline())

	s.False(s.held(mgr, encounter.DeedIntimidate), "a 7 does not reach 9")
}

// AN AUTHORED DC HAS TO REACH A LIVE RUN, and the placement is the only road
// it can travel: a host that resolves monster content at runtime brings every
// monster in through Launch.
func (s *SpeakSuite) TestAnAuthoredLineSurvivesTheSpawn() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := threatLine()
		line.Check = []encounter.CheckApproach{{Ability: "intimidation", DC: 12}}
		in.Options = []encounter.Option{line}
	}
	mgr := s.aYard([]int{10})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Equal(12, out.DC, "the sergeant's own price, carried through the spawn")
	s.Equal(session.DoorApproach{Ability: "intimidation", DC: 12}, out.Applied)
	s.False(out.Beaten, "an 11 does not reach 12")
}

// And the lines' ABSENCE is the whole ruling: the same scene with nothing
// authored cannot be spoken to at all, and the panel said so before anybody
// tried.
func (s *SpeakSuite) TestAnUnauthoredSpawnCannotBeSpokenTo() {
	s.authored = func(in *dungeonspec.MonsterPlacement) { in.Options = nil }
	mgr := s.aYard([]int{10})

	_, err := s.threaten(mgr)
	s.Require().ErrorIs(err, session.ErrNoOption, "there is no derived line to fall back on")

	row := s.row(mgr)
	s.False(row.Available, "and the offer withheld it first")
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallNoOption, row.Why.Reason)
	s.Equal("nothing here has a line for you", row.Why.Text)
	s.Empty(row.Candidates, "the goblin is not a candidate, not an unavailable one")
	s.Empty(row.SpeakOptions)

	// NOTHING WAS CHARGED on the way to the refusal: the lines are read
	// before the action is spent, exactly as the sight check is.
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Require().NotNil(stored.ActionEconomy)
	s.Equal(1, stored.ActionEconomy.ActionsRemaining)
}

// The world half has the same road to travel: a fact on the line's success
// rows reaches the run through the spawn, and a beaten line teaches it to the
// witnesses — proved through the stance it flips, the only thing the fact is
// FOR.
func (s *SpeakSuite) TestAnAuthoredFactSurvivesTheSpawnAndIsTaught() {
	const fact = "sergeant-cowed"
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := threatLine()
		line.Success = []encounter.Answer{{Weight: 1, Fact: encounter.FactID(fact)}}
		in.Options = []encounter.Option{line}
		in.Faction = "raiders"
	}
	// TWO FACES: the d20 for the check, then the world's own die for the
	// success rows — one entry, so the face is the only one there is.
	mgr := s.aCamp(fact, []int{10, 1})

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	stances := beatsNamed(s.events(mgr, "alice"), encounter.BeatStance)
	s.Require().NotEmpty(stances, "the fact never reached the run, so the camp never learned anything")
	var stance struct {
		Between []string `json:"between"`
		Stance  string   `json:"stance"`
	}
	stances[0].decode(s.T(), &stance)
	s.Equal([]string{"party", "raiders"}, stance.Between, "cowing it in front of the camp turned the camp")
	s.Equal("neutral", stance.Stance)
}

// aCamp is the yard with a faction that is hostile until it learns a fact,
// and the goblin as that faction's mind — the smallest world in which
// teaching a fact is observable at this seam.
func (s *SpeakSuite) aCamp(fact string, rolls []int) *session.Manager {
	roller := &sequenceDice{rolls: append([]int{10, 10}, rolls...)}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: roller, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters,
	})
	s.Require().NoError(err)

	sc := scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Factions: []encounter.FactionInput{{ID: "raiders", Mind: "goblin"}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"raiders", encounter.FactionParty},
				Stance:  encounter.StanceHostile,
				Until:   []encounter.UntilStep{{When: encounter.TriggerFact{Fact: encounter.FactID(fact)}}},
			}},
		},
		Party: []sceneSeat{seatAt("alice", 1, 1)},
	}

	ctx := context.Background()
	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 5, 1)
	goblin.Options = []encounter.Option{threatLine()}
	s.authored(&goblin)
	sc.Monsters = []dungeonspec.MonsterPlacement{goblin}
	launchScene(s.T(), mgr, sc)

	authorTurnClock(s.T(), s.encounters, testSession, []string{"alice", "goblin"}, 0)

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock)

	return mgr
}

// menu is the Speak row's lines as (target, option) pairs, in the row's own
// order.
func menu(row session.Declaration) [][2]string {
	out := make([][2]string, 0, len(row.SpeakOptions))
	for _, line := range row.SpeakOptions {
		who := line.Target
		if who == "" {
			who = "prop:" + line.Prop
		}
		out = append(out, [2]string{who, line.OptionID})
	}
	return out
}

// THE OFFER IS JUDGED ON THE SPEAKER'S OWN HOLDINGS (R1, R9). The goblin
// carries two lines: one anybody may say, and one that waits on a fact — its
// name, which the first line teaches. Until alice knows the name, the second
// line is not on her menu at all (not listed unavailable: listing it would
// confirm a line the author hid), and a stale client naming it is refused at
// the door BEFORE anything is charged or appended. Once she has learned it,
// it is offered.
func (s *SpeakSuite) TestALineWaitingOnAFactIsOfferedOnlyOnceTheSpeakerKnowsIt() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := threatLine()
		line.Success = []encounter.Answer{{Weight: 1, Fact: "goblin-name"}}
		byName := encounter.Option{
			ID: "by-name", Say: "Snikt. It is Snikt, isn't it?",
			Check:    []encounter.CheckApproach{{Ability: "persuasion", DC: 5}},
			Requires: encounter.TriggerFact{Fact: "goblin-name"},
		}
		in.Options = []encounter.Option{line, byName}
	}
	// The d20 for the threat, then the world's die for its one success row.
	mgr := s.aYard([]int{10, 1})

	row := s.row(mgr)
	s.Equal([][2]string{{"goblin", "threaten"}}, menu(row), "the line waiting on a fact is not listed")
	s.Require().Len(row.SpeakOptions, 1)
	s.Equal(session.SpeakOptionRow{
		Target: "goblin", OptionID: "threaten", Line: "Drop the bow or I drop you.", Available: true,
	}, row.SpeakOptions[0], "the line, verbatim, and who offers it")

	// THE DOOR AGREES WITH THE OFFER, and refuses before anything happens.
	before := len(s.beats(mgr, "alice"))
	_, err := s.say(mgr, "by-name")
	s.Require().ErrorIs(err, session.ErrNoOption, "a stale client naming a hidden line")
	s.Len(s.beats(mgr, "alice"), before, "nothing was appended")
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Require().NotNil(stored.ActionEconomy)
	s.Equal(1, stored.ActionEconomy.ActionsRemaining, "and nothing was charged")

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten, "the threat landed and its success row taught the name to every witness")

	s.Equal([][2]string{{"goblin", "threaten"}, {"goblin", "by-name"}}, menu(s.row(mgr)),
		"alice saw herself say it, so she knows the name now, and the line is hers to say")
}

// A PROP CAN BE SPOKEN TO (rpg-project#569): an altar with a line on it is on
// the menu beside the creatures, the line is said to the prop by id, no deed
// lands on anybody, and the beat names the prop.
func (s *SpeakSuite) TestALineSaidToAProp() {
	open := false
	altar := encounter.PropInput{
		ID: "altar", Ref: "altar", At: spatial.Position{X: 2, Y: 3},
		BlocksMovement: &open, BlocksLineOfSight: &open,
		Options: []encounter.Option{{
			ID: "read", Say: "I read the inscription aloud.",
			Check: []encounter.CheckApproach{{Ability: "intimidation", DC: 5}},
		}},
	}
	mgr := s.aYard([]int{10}, altar)

	s.Equal([][2]string{{"goblin", "threaten"}, {"prop:altar", "read"}}, menu(s.row(mgr)),
		"the creatures first, then the props the speaker can address")

	out, err := mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Prop: "altar", OptionID: "read",
	})
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.Equal("altar", out.Prop)
	s.Empty(out.Target)
	s.False(s.held(mgr, encounter.DeedIntimidate),
		"an Intimidation line to a prop lands no deed — a prop is cowed by nothing")

	spoke := s.lastSpoke(mgr, "alice")
	s.Require().NotNil(spoke)
	s.Equal("altar", spoke.Prop)
	s.Empty(spoke.Target)
	s.Equal("read", spoke.OptionID)

	// And a line one prop does not carry is refused like any other.
	_, err = mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Prop: "altar", OptionID: "threaten",
	})
	s.Require().ErrorIs(err, session.ErrNoOption)
}

// A PAUSED LINE SAID TO A PROP RESUMES AS THAT SAME LINE. The check window
// persists the prop and the option, and the resumed check lands through the
// same composition op with the resumed approach's skill: the beat names the
// prop and no creature, and the prop's own answer rows are rolled.
//
// 6 on the d20 with CHA 8's -1 and proficiency's +2 is a 7, which misses DC 9;
// the Guidance d4's 4 makes it 11. Then the world's die for the one answer row.
func (s *SpeakSuite) TestAGuidedLineToAPropResumesAsTheSameLine() {
	open := false
	altar := encounter.PropInput{
		ID: "altar", Ref: "altar", At: spatial.Position{X: 2, Y: 3},
		BlocksMovement: &open, BlocksLineOfSight: &open,
		Options: []encounter.Option{{
			ID: "read", Say: "I read the inscription aloud.",
			Check:   []encounter.CheckApproach{{Ability: "intimidation", DC: 9}},
			Success: []encounter.Answer{{Weight: 1, Say: "The letters glow."}},
		}},
	}
	mgr := s.aYard([]int{6, 4, 1}, altar)
	s.guide()

	out, err := mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Prop: "altar", OptionID: "read",
	})
	s.Require().NoError(err)
	s.Require().True(out.Paused, "the checker holds a die and the machine stopped")
	s.Equal("altar", out.Prop)
	s.Equal("read", out.OptionID)

	s.answer(mgr, session.Take(""))

	spoke := s.lastSpoke(mgr, "alice")
	var answered *answeredPayload
	if beats := beatsNamed(s.events(mgr, "alice"), encounter.BeatAnswered); len(beats) > 0 {
		answered = &answeredPayload{}
		beats[len(beats)-1].decode(s.T(), answered)
	}
	s.Require().NotNil(spoke, "the resumed verdict reached the log")
	s.Equal("altar", spoke.Prop, "the window carried the prop it stopped on")
	s.Empty(spoke.Target)
	s.Equal("read", spoke.OptionID)
	s.Equal("intimidation", spoke.Skill, "the resumed approach's skill")
	s.Equal(11, spoke.Total, "the offered die joined the total, and nothing was re-rolled")
	s.True(spoke.Beaten)

	s.Require().NotNil(answered, "the prop's own success row was rolled")
	s.Equal("altar", answered.Prop)
	s.Empty(answered.Creature)
	s.Equal("read", answered.OptionID)
	s.Equal("The letters glow.", answered.Say)
	s.False(s.held(mgr, encounter.DeedIntimidate), "a prop is cowed by nothing")
}

// talkingFighter is [armedFighter] who took Intimidation and Persuasion at
// creation — the fixture every scene above needs, so that an untrained
// checker appears only in the scenes that are about one.
func talkingFighter(id string) *character.Data {
	sheet := armedFighter(id)
	sheet.Skills = map[skills.Skill]shared.ProficiencyLevel{
		skills.Intimidation: shared.Proficient,
		skills.Persuasion:   shared.Proficient,
	}

	return sheet
}

// deedHeld reads whether one member holds a deed with this verb from another.
func deedHeld(t require.TestingT, mgr *session.Manager, holder, actor, verb string) bool {
	sightings, err := mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: holder})
	require.NoError(t, err)
	for _, holding := range sightings.Sightings {
		if holding.Channel != string(deed.Channel) {
			continue
		}
		saw, err := deed.Decode(holding.Payload)
		require.NoError(t, err)
		if saw.Verb == verb && string(saw.Actor) == actor {
			return true
		}
	}
	return false
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
