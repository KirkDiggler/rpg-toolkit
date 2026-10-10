// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ReactWindowSuite is rung 3 of rpg-project#316: a monster walking out of a
// player's reach ASKS that player whether they swing, instead of swinging for
// them.
//
// Every scene here drives the whole stack — the session's Mover poses, the
// composition checkpoints the turn, the ledger persists, React answers, the
// turn resumes — because the thing being proven is that those five agree. A
// unit test of any one of them would pass with the other four wired wrong,
// which is exactly how the last spine came to have a freeze no test could
// enter.
type ReactWindowSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestReactWindowSuite(t *testing.T) {
	suite.Run(t, new(ReactWindowSuite))
}

func (s *ReactWindowSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("fighter"), armedFighter("second"))
}

// pathWalker walks each named monster one fixed path on its first turn and
// passes ever after — a monster that deliberately leaves a threatened square,
// which is the one thing session.Driver() will never do on its own.
//
// KEYED BY MEMBER, unlike rung 2's single-shot retreatWalker, because the
// done-when scene needs two skeletons to walk on their OWN turns rather than
// one walking and the rest passing.
type pathWalker struct {
	paths  map[string][]spatial.Position
	walked map[string]bool
}

func (p *pathWalker) Act(view session.MonsterView) (session.TurnIntent, error) {
	path, ok := p.paths[view.Self]
	if !ok || p.walked[view.Self] {
		return session.Pass{}, nil
	}
	p.walked[view.Self] = true
	return session.Move{Path: path}, nil
}

// managerWith builds this suite's manager over one turn driver.
func (s *ReactWindowSuite) managerWith(driver session.TurnDriver) *session.Manager {
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: driver,
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	return mgr
}

// inCombat gives a stored sheet the economy of somebody in a fight, with
// reactions in hand. AFTER Join, never before — Join writes the joined
// member's sheet from a freshly loaded character.
func (s *ReactWindowSuite) inCombat(id string, reactions int) {
	stored, err := s.characters.GetCharacter(context.Background(), id)
	s.Require().NoError(err)
	stored.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
		ReactionsRemaining: reactions, MovementRemaining: 30,
	}
	s.Require().NoError(s.characters.SaveCharacter(context.Background(), stored))
}

// reactionsLeft reads what a member has left on the stored sheet — the only
// place a spend is durable, and the only place a REFUND would have to show up
// if holding cost anything.
func (s *ReactWindowSuite) reactionsLeft(member string) int {
	stored, err := s.characters.GetCharacter(context.Background(), member)
	s.Require().NoError(err)
	s.Require().NotNil(stored.ActionEconomy, "member %q is not in a fight", member)
	return stored.ActionEconomy.ReactionsRemaining
}

// frail drops a spawned monster to one hit point, so the next blow that lands
// on it fells it. Written straight into the session record, because a spawned
// sheet lives there rather than behind a repository.
func (s *ReactWindowSuite) frail(id string) {
	ctx := context.Background()
	data, err := s.sessions.GetSession(ctx, "sess")
	s.Require().NoError(err)
	found := false
	for i := range data.NPCs {
		if data.NPCs[i].ID == id {
			data.NPCs[i].HitPoints = 1
			found = true
		}
	}
	s.Require().True(found, "no spawned sheet for %q", id)
	s.Require().NoError(s.sessions.SaveSession(ctx, data))
}

// start launches the standard room with the given players seated and the
// given monsters placed, then puts each player in a fight's economy with one
// reaction in hand. AFTER the launch, never before — the launch long-rests and
// writes each seated member's sheet.
func (s *ReactWindowSuite) start(mgr *session.Manager, players []sceneSeat, monsters ...dungeonspec.MonsterPlacement) {
	sc := tombRoom(12, 6)
	sc.Party = players
	sc.Monsters = monsters
	launchScene(s.T(), mgr, sc)
	for _, seat := range players {
		s.inCombat(seat.ID, 1)
	}
}

// reactSkeleton is one skeleton placed on an authored cell.
func reactSkeleton(id string, col, row int) dungeonspec.MonsterPlacement {
	return monsterAt(id, refs.Monsters.Skeleton().String(), col, row)
}

// reactRow is the member's own open REACT declaration, or the zero value when
// nothing is being asked of them.
//
// READ THROUGH Afford, never minted, for the same reason currentMoveID is: the
// selector is the server's and a test that built one would be asserting against
// its own arithmetic.
func (s *ReactWindowSuite) reactRow(mgr *session.Manager, member string) session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	for _, declaration := range out.Declarations {
		if declaration.Verb == session.VerbReact {
			return declaration
		}
	}
	return session.Declaration{}
}

// windowBeats reads a member's story and returns every window_opened beat.
type windowBeat struct {
	Beat    string           `json:"beat"`
	Member  string           `json:"member"`
	From    spatial.Position `json:"from"`
	To      spatial.Position `json:"to"`
	Windows []struct {
		Audience string `json:"audience"`
		Reaction struct {
			Ref  string `json:"ref"`
			Name string `json:"name"`
		} `json:"reaction"`
	} `json:"windows"`
}

func (s *ReactWindowSuite) windowBeats(mgr *session.Manager, member string) []windowBeat {
	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	var out []windowBeat
	for _, entry := range entries {
		// PEEKED FIRST. One story carries a dozen beat shapes and several of
		// them spell "to" as something other than a cell — decoding every
		// entry into this shape would fail on the first transfer.
		var peek struct {
			Beat string `json:"beat"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &peek))
		if peek.Beat != "window_opened" {
			continue
		}
		var body windowBeat
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		out = append(out, body)
	}
	return out
}

// reactionBeats is mover_test.go's reader, repeated here so this suite reads
// standalone: every beat in member's story that names a reaction.
func (s *ReactWindowSuite) reactionBeats(mgr *session.Manager, member string) []reactedBeat {
	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	var out []reactedBeat
	for _, entry := range entries {
		var body reactedBeat
		if json.Unmarshal(entry.Payload, &body) != nil || body.Reaction == nil {
			continue
		}
		s.Require().Contains([]string{"struck", "missed"}, body.Beat,
			"a strike is the only thing recorded as a reaction today")
		out = append(out, body)
	}
	return out
}

// where is one member's cell.
func (s *ReactWindowSuite) where(mgr *session.Manager, member string) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Position
}

// twoSkeletons is the done-when scene: a fighter between two skeletons, each
// of which walks out of her reach on its own turn.
func (s *ReactWindowSuite) twoSkeletons() *session.Manager {
	mgr := s.managerWith(&pathWalker{
		paths: map[string][]spatial.Position{
			"skel-1": {hexCell(5, 0), hexCell(6, 0)},
			"skel-2": {hexCell(1, 0), hexCell(0, 0)},
		},
		walked: map[string]bool{},
	})
	s.start(mgr, []sceneSeat{seatAt("fighter", 3, 0)}, reactSkeleton("skel-1", 4, 0), reactSkeleton("skel-2", 2, 0))
	return mgr
}

// endTurn ends member's turn through the current declaration.
func (s *ReactWindowSuite) endTurn(mgr *session.Manager, member string) {
	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: member,
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", member),
	})
	s.Require().NoError(err)
}

// react answers member's own open window.
func (s *ReactWindowSuite) react(mgr *session.Manager, member string, choice session.Answer) {
	row := s.reactRow(mgr, member)
	s.Require().NotEmpty(row.ID, "no open window for %q", member)
	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: member, DeclarationID: row.ID, Answer: choice,
	})
	s.Require().NoError(err)
}

// TestTheFighterIsAskedAndHoldsThenIsAskedAndStrikes is the design's done-when,
// end to end and in one scene, because it is one scene: two skeletons pass the
// fighter on their turns, she declines the first swing and takes the second,
// and the reaction she kept is the one she spends.
//
// THE SECOND PASS IS THE POINT. Kirk's case is exactly this — "I might neglect
// a hit on one and want to target the second" — and it is only a case at all
// because holding is free (ruling R1). Before that, the first trigger spent her
// reaction whether she swung or not, and the second skeleton walked past a
// fighter who had nothing left.
func (s *ReactWindowSuite) TestTheFighterIsAskedAndHoldsThenIsAskedAndStrikes() {
	mgr := s.twoSkeletons()

	s.endTurn(mgr, "fighter")

	// THE FIRST PASS ASKS. The step is announced and not taken: the skeleton
	// still stands where it was when the question was posed.
	opened := s.windowBeats(mgr, "fighter")
	s.Require().Len(opened, 1, "the first skeleton's first cell leaves reach and asks")
	s.Equal("skel-1", opened[0].Member)
	s.Equal(hexCell(4, 0), opened[0].From, "announced from where the skeleton still stands")
	s.Equal(hexCell(5, 0), opened[0].To)
	s.Require().Len(opened[0].Windows, 1)
	s.Equal("fighter", opened[0].Windows[0].Audience)
	s.Equal(oaRef(), opened[0].Windows[0].Reaction.Ref)
	s.Equal(hexCell(4, 0), s.where(mgr, "skel-1"), "the announced step is NOT taken while the question stands")

	// The dock is told, before any click, exactly what it may do.
	row := s.reactRow(mgr, "fighter")
	s.Require().NotEmpty(row.ID)
	s.True(row.Available, "every gate a reaction has was passed before the question was worth asking")
	s.Equal(session.SlotReaction, row.Slot)
	s.Require().NotNil(row.Reaction)
	s.Equal(oaRef(), row.Reaction.Ref)
	s.Equal("Opportunity Attack", row.Reaction.Name)
	s.Equal([]session.TargetCandidate{{Member: "skel-1", Available: true}}, row.Candidates)

	// And the freeze is real, not merely announced.
	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(3, 1)},
		DeclarationID: "anything",
	})
	s.Require().ErrorIs(err, session.ErrWindowOpen, "a change verb is refused while the table is waiting")

	// HOLD. The skeleton finishes the walk it announced, and the fighter's
	// reaction is still in her hand for the next one.
	s.react(mgr, "fighter", session.Decline())
	s.Equal(hexCell(6, 0), s.where(mgr, "skel-1"), "the resumed turn walks the rest of the path")
	s.Empty(s.reactionBeats(mgr, "fighter"), "holding swings at nobody")
	s.Equal(1, s.reactionsLeft("fighter"), "a reaction nobody took costs nothing")

	// THE SECOND PASS ASKS AGAIN — posed during the resume that finished the
	// first skeleton's turn, without anybody calling a verb for it.
	s.Require().Len(s.windowBeats(mgr, "fighter"), 2)
	second := s.reactRow(mgr, "fighter")
	s.Require().NotEmpty(second.ID)
	s.NotEqual(row.ID, second.ID, "a window id is never reused, so neither is its selector")
	s.Equal([]session.TargetCandidate{{Member: "skel-2", Available: true}}, second.Candidates)

	// STRIKE.
	s.react(mgr, "fighter", session.Take(""))

	beats := s.reactionBeats(mgr, "fighter")
	s.Require().Len(beats, 1, "one swing, at the skeleton she chose")
	s.Equal("fighter", beats[0].Actor)
	s.Equal([]string{"skel-2"}, beats[0].Targets)
	s.Equal(oaRef(), beats[0].Reaction.Ref)
	s.Equal("Opportunity Attack", beats[0].Reaction.Name)

	s.Equal(hexCell(0, 0), s.where(mgr, "skel-2"), "the second skeleton's turn finishes from where it stopped")
	s.Equal(1, s.reactionsLeft("fighter"), "the spent reaction refreshes at her next turn start")
	s.Empty(s.reactRow(mgr, "fighter").ID, "nothing is being asked any more")

	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal("fighter", turn.Active, "and the fight is back on the fighter's own turn")
}

// TestASpentReactionAsksNobody is the gate order made visible: the condition's
// own predicate fails before anything is offered, so no window opens at all
// and the fight never pauses.
//
// It stands in for every upstream refusal — a disengaging mover, an ally, a
// caster with no melee weapon — because they all fail in the same place, ahead
// of the pose. The monk who spends a ki point on Step of the Wind is the same
// scene one layer up: the fold marks opportunity attacks prevented and the
// trigger the pose would need never arrives (TestADisengagingMoverProvokesNothing).
func (s *ReactWindowSuite) TestASpentReactionAsksNobody() {
	mgr := s.twoSkeletons()
	s.inCombat("fighter", 0)

	s.endTurn(mgr, "fighter")

	s.Empty(s.windowBeats(mgr, "fighter"), "a fighter with no reaction is never asked")
	s.Empty(s.reactRow(mgr, "fighter").ID)
	s.Equal(hexCell(6, 0), s.where(mgr, "skel-1"), "and both skeletons walk unimpeded")
	s.Equal(hexCell(0, 0), s.where(mgr, "skel-2"))
}

// TestAPlayersOwnWalkNeverAsksAnybody is ruling R4 from the side that produces
// it: a window opens inside a MONSTER's turn and nowhere else. A player walking
// past monsters is rung 2 — automatic, both ways — and freezing the table on
// one player's step would be a question with no producer behind it.
func (s *ReactWindowSuite) TestAPlayersOwnWalkNeverAsksAnybody() {
	ctx := context.Background()
	mgr := s.managerWith(session.Pass{})
	s.start(mgr, []sceneSeat{seatAt("fighter", 2, 0)}, reactSkeleton("skel-1", 3, 0))

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(1, 0)},
		DeclarationID: currentMoveID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 1, "the walk completes; nothing pauses it")

	s.Empty(s.windowBeats(mgr, "fighter"), "no window opens on a player's own step")
	beats := s.reactionBeats(mgr, "fighter")
	s.Require().Len(beats, 1, "and the skeleton's own bite still fires automatically")
	s.Equal("skel-1", beats[0].Actor)
}

// TestSomebodyElsesWindowIsRefused: two fighters can be asked about one step
// (ruling R3), and each may answer only their own. The refusal is
// ErrNotAudience rather than ErrNoWindow, because the window is open and real
// — the caller is simply not the one being asked.
func (s *ReactWindowSuite) TestSomebodyElsesWindowIsRefused() {
	mgr := s.twoFightersOneSkeleton()

	mine := s.reactRow(mgr, "fighter")
	s.Require().NotEmpty(mine.ID)

	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "second", DeclarationID: mine.ID, Answer: session.Take(""),
	})
	s.Require().ErrorIs(err, session.ErrNotAudience)
}

// TestAnUnknownChoiceIsRefused. Two checks, two refusals with one sentinel: a
// word this build has never posed, and — were a window ever to offer fewer
// options — a word this window did not offer.
func (s *ReactWindowSuite) TestAnUnknownChoiceIsRefused() {
	mgr := s.twoSkeletons()
	s.endTurn(mgr, "fighter")

	row := s.reactRow(mgr, "fighter")
	s.Require().NotEmpty(row.ID)

	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter", DeclarationID: row.ID, Answer: session.Take("parry"),
	})
	s.Require().ErrorIs(err, session.ErrNotOffered)

	s.Require().NotEmpty(s.reactRow(mgr, "fighter").ID, "and the window is still open to be answered properly")
}

// TestAStaleDeclarationIsRefused: an id that names no open window is stale, not
// malformed — a second click on a REACT button after the answer already landed.
func (s *ReactWindowSuite) TestAStaleDeclarationIsRefused() {
	mgr := s.twoSkeletons()
	s.endTurn(mgr, "fighter")

	row := s.reactRow(mgr, "fighter")
	s.Require().NotEmpty(row.ID)
	s.react(mgr, "fighter", session.Decline())

	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter", DeclarationID: row.ID, Answer: session.Take(""),
	})
	s.Require().ErrorIs(err, session.ErrNoWindow, "a window id is never reused, so an answered one is gone for good")
}

// TestARestartBetweenTheQuestionAndTheAnswerChangesNothing is the design's own
// done-when clause. A manager built fresh over the same repositories holds no
// memory of the pose at all: both halves — the questions in the session record,
// the interrupted turn in the encounter's — are read back off storage.
func (s *ReactWindowSuite) TestARestartBetweenTheQuestionAndTheAnswerChangesNothing() {
	mgr := s.twoSkeletons()
	s.endTurn(mgr, "fighter")
	s.Require().NotEmpty(s.reactRow(mgr, "fighter").ID)

	// A NEW MANAGER OVER THE SAME STORES, with a driver that would refuse to
	// walk anything: the rest of the interrupted walk is the ENCOUNTER's, not
	// the driver's, so the resume must finish it without asking anybody.
	restarted := s.managerWith(session.Pass{})

	row := s.reactRow(restarted, "fighter")
	s.Require().NotEmpty(row.ID, "the question survives the restart")
	s.Equal(oaRef(), row.Reaction.Ref)

	_, err := restarted.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter", DeclarationID: row.ID, Answer: session.Take(""),
	})
	s.Require().NoError(err)

	beats := s.reactionBeats(restarted, "fighter")
	s.Require().Len(beats, 1, "the swing the question described")
	s.Equal([]string{"skel-1"}, beats[0].Targets)
	s.Equal(hexCell(6, 0), s.where(restarted, "skel-1"), "and the walk it interrupted finishes")
}

// twoFightersOneSkeleton is the R3 scene: one step, two people asked.
func (s *ReactWindowSuite) twoFightersOneSkeleton(bystanders ...string) *session.Manager {
	mgr := s.managerWith(&pathWalker{
		paths:  map[string][]spatial.Position{"skel-1": {hexCell(4, 0), hexCell(5, 0)}},
		walked: map[string]bool{},
	})
	monsters := []dungeonspec.MonsterPlacement{reactSkeleton("skel-1", 3, 0)}
	// On the board from the start, before the first turn ends: the
	// fight-survives scene below needs them standing when the window opens.
	for i, id := range bystanders {
		monsters = append(monsters, reactSkeleton(id, 9, 4+i))
	}
	s.start(mgr, []sceneSeat{seatAt("fighter", 2, 0), seatAt("second", 2, 1)}, monsters...)
	s.endTurn(mgr, "fighter")
	s.endTurn(mgr, "second")
	return mgr
}

// TestOneStepAsksEveryPlayerReactorAtOnce is ruling R3. The design said one
// window at a time and the plan overruled it: the ledger already holds several,
// serial asking would need a memory of who had already held, and the freeze
// lifts on the LAST answer either way.
func (s *ReactWindowSuite) TestOneStepAsksEveryPlayerReactorAtOnce() {
	mgr := s.twoFightersOneSkeleton()

	opened := s.windowBeats(mgr, "fighter")
	s.Require().Len(opened, 1, "one step")
	s.Require().Len(opened[0].Windows, 2, "two questions")
	audiences := []string{opened[0].Windows[0].Audience, opened[0].Windows[1].Audience}
	s.ElementsMatch([]string{"fighter", "second"}, audiences)

	s.Require().NotEmpty(s.reactRow(mgr, "fighter").ID)
	s.Require().NotEmpty(s.reactRow(mgr, "second").ID)

	// THE FIRST ANSWER CHANGES NOTHING BUT THE LEDGER. The skeleton has not
	// moved, and the second fighter is still being asked.
	s.react(mgr, "fighter", session.Decline())
	s.Equal(hexCell(3, 0), s.where(mgr, "skel-1"), "the fight is still waiting on the second answer")
	s.Empty(s.reactRow(mgr, "fighter").ID)
	s.Require().NotEmpty(s.reactRow(mgr, "second").ID)

	s.react(mgr, "second", session.Take(""))
	s.Equal(hexCell(5, 0), s.where(mgr, "skel-1"), "the last answer resumes the turn")

	beats := s.reactionBeats(mgr, "second")
	s.Require().Len(beats, 1)
	s.Equal("second", beats[0].Actor)
	s.Equal(1, s.reactionsLeft("fighter"), "the one who held kept theirs")
	s.Equal(0, s.reactionsLeft("second"), "the one who struck spent theirs")
}

// TestAStrikeThatDropsTheMoverHoldsTheRestOfTheWindows: there is nothing left
// to react to, so the remaining audiences are answered on their behalf rather
// than offered a swing at a body — and the mover falls in the cell it was
// LEAVING, which is ruling R6 arriving through the pause.
func (s *ReactWindowSuite) TestAStrikeThatDropsTheMoverHoldsTheRestOfTheWindows() {
	// A SECOND SKELETON, standing far off and doing nothing, so the fight
	// SURVIVES the death of the one that is walking. That is what separates
	// this scene from the one below it: here the turn ends and the fight
	// carries on, there the fight itself ends underneath the question, and
	// the two took different paths through the composition's resume.
	mgr := s.twoFightersOneSkeleton("skel-2")
	s.frail("skel-1")

	s.react(mgr, "fighter", session.Take(""))

	s.Empty(s.reactRow(mgr, "second").ID, "the other question is closed, not left hanging")
	s.Equal(hexCell(3, 0), s.where(mgr, "skel-1"), "it falls in the cell it was leaving, not the one it was entering")
	for _, beat := range s.reactionBeats(mgr, "second") {
		s.NotEqual("second", beat.Actor, "nobody swung at a body")
	}
	s.Equal(1, s.reactionsLeft("second"), "a window held on somebody's behalf costs them nothing")

	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal("fighter", turn.Active, "and the turn is over")
}

// TestTheWindowAndTheSwingReachTheEventStreamTyped drives the other end of the
// wire: what a client actually receives.
//
// TWO GAPS CLOSE HERE. The window beat is a new kind, and the reaction on a
// struck beat has been written by the composition since rung 2 with nothing at
// this seam decoding it — a field dead on arrival that would have shipped a
// dock unable to say why the fighter dealt damage on a skeleton's turn.
func (s *ReactWindowSuite) TestTheWindowAndTheSwingReachTheEventStreamTyped() {
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{},
		TurnDriver: &pathWalker{
			paths:  map[string][]spatial.Position{"skel-1": {hexCell(4, 0), hexCell(5, 0)}},
			walked: map[string]bool{},
		},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: stream,
	})
	s.Require().NoError(err)
	s.start(mgr, []sceneSeat{seatAt("fighter", 2, 0)}, reactSkeleton("skel-1", 3, 0))
	s.endTurn(mgr, "fighter")

	var opened *session.WindowOpenedBody
	for _, event := range stream.published {
		if event.Kind != session.EventWindowOpened || event.Recipient != "fighter" {
			continue
		}
		body, ok := event.Body.(session.WindowOpenedBody)
		s.Require().True(ok, "a window event carries a typed body, not a shrug")
		opened = &body
	}
	s.Require().NotNil(opened, "the fighter is told, live, that they are being asked")
	s.Equal("skel-1", opened.Mover)
	s.Equal(hexCell(3, 0), opened.From)
	s.Equal(hexCell(4, 0), opened.To)
	s.Equal([]string{"fighter"}, opened.Audience)
	s.Equal(session.ReactionRef{Ref: oaRef(), Name: "Opportunity Attack"}, opened.Reaction)

	s.react(mgr, "fighter", session.Take(""))

	var struck *session.StruckBody
	var missed *session.MissedBody
	for _, event := range stream.published {
		if event.Recipient != "fighter" {
			continue
		}
		switch body := event.Body.(type) {
		case session.StruckBody:
			struck = &body
		case session.MissedBody:
			missed = &body
		}
	}
	s.Require().True(struck != nil || missed != nil, "the swing reaches the stream")
	switch {
	case struck != nil:
		s.Require().NotNil(struck.Reaction, "and it says what it was taken AS")
		s.Equal(oaRef(), struck.Reaction.Ref)
		s.Equal("Opportunity Attack", struck.Reaction.Name)
	default:
		s.Require().NotNil(missed.Reaction, "a reaction that missed is still a reaction")
		s.Equal(oaRef(), missed.Reaction.Ref)
	}
}

// TestAnOrdinarySwingCarriesNoReaction is the false-vs-absent half of the field
// above: nil is an answer — this was a declared attack — and a client that saw
// a populated Reaction on every swing would label the whole log.
func (s *ReactWindowSuite) TestAnOrdinarySwingCarriesNoReaction() {
	ctx := context.Background()
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: stream,
	})
	s.Require().NoError(err)
	s.start(mgr, []sceneSeat{seatAt("fighter", 2, 0)}, reactSkeleton("skel-1", 3, 0))

	_, err = mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "fighter", Target: "skel-1",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)

	seen := false
	for _, event := range stream.published {
		switch body := event.Body.(type) {
		case session.StruckBody:
			seen = true
			s.Nil(body.Reaction, "a declared swing was taken as nothing but itself")
		case session.MissedBody:
			seen = true
			s.Nil(body.Reaction)
		}
	}
	s.True(seen, "the swing reached the stream at all")
}

// TestAStrikeThatEndsTheFightHoldsTheRestAndCarriesOn is the scene above with
// the one skeleton that was left removed: the blow drops the LAST monster, so
// the fight itself ends underneath the question.
//
// IT IS THE ORDINARY CONSEQUENCE OF THE ANSWER, and for one commit it was
// refused as corruption. Dropping the last monster dissolves the bubble and
// splices the mover out of it, all recorded before anything resumes — so a
// mid-verb reload found a paused turn whose member was on no clock and called
// the blob invalid. encounter b0e8689d settled it the right way round: a
// paused member in no fight is legal, the turn is already over, and resuming
// clears the pause and lets the run continue.
//
// So this test's real subject is that nothing special happens. No error, no
// stranded question, and the players are free to walk again.
func (s *ReactWindowSuite) TestAStrikeThatEndsTheFightHoldsTheRestAndCarriesOn() {
	ctx := context.Background()
	mgr := s.twoFightersOneSkeleton()
	s.frail("skel-1")

	s.react(mgr, "fighter", session.Take(""))

	// BOTH QUESTIONS ARE CLOSED. The one that was answered, and the one that
	// was held on its audience's behalf because there was no longer a walk to
	// react to.
	s.Empty(s.reactRow(mgr, "fighter").ID)
	s.Empty(s.reactRow(mgr, "second").ID, "the other question is closed, not stranded by the dissolve")
	for _, beat := range s.reactionBeats(mgr, "second") {
		s.NotEqual("second", beat.Actor, "and nobody swung at a body")
	}

	// The second fighter's economy is GONE rather than down by one, because
	// the fight it belonged to ended in this same call — exitDissolvedCombatants
	// puts the light out for every player whose fight dissolved. Asserting a
	// reaction still in hand here would be asserting that the fight is still
	// running, which is the opposite of what this scene is about.
	stored, err := s.characters.GetCharacter(ctx, "second")
	s.Require().NoError(err)
	s.Nil(stored.ActionEconomy, "the fight ended, so the turn economy was put out")

	s.Equal(hexCell(3, 0), s.where(mgr, "skel-1"),
		"it falls in the cell it was leaving; the announced step never happened")

	beats := s.reactionBeats(mgr, "fighter")
	s.Require().Len(beats, 1)
	s.Equal("fighter", beats[0].Actor)
	s.Equal([]string{"skel-1"}, beats[0].Targets)
	s.Equal(oaRef(), beats[0].Reaction.Ref)

	// THE FIGHT IS OVER AND THE RUN IS NOT. Both players are back on the world
	// clock, and a free-roam walk — a change verb, so it passes through the
	// freeze that is no longer there — is accepted.
	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal(session.ClockWorld, turn.Clock, "the last monster is down, so the fight dissolved")

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(1, 0)},
	})
	s.Require().NoError(err, "nothing is frozen any more")
	s.Require().Len(out.Steps, 1)
}

// TestAnOpportunityAttackPausesAndBillsOnTake is the envelope's opportunity
// proof. One skeleton step asks two fighters at once, and both windows stand.
// Taking charges the first fighter's reaction through resolution's one door
// and tells the swing AS an opportunity attack; the step still waits on the
// second. Declining charges nothing, and the last answer takes the step and
// resumes the turn.
func (s *ReactWindowSuite) TestAnOpportunityAttackPausesAndBillsOnTake() {
	mgr := s.twoFightersOneSkeleton()
	s.Require().NotEmpty(s.reactRow(mgr, "fighter").ID, "control: the first fighter is asked")
	s.Require().NotEmpty(s.reactRow(mgr, "second").ID, "control: and so is the second, at once")
	s.Require().Equal(1, s.reactionsLeft("fighter"))
	s.Require().Equal(1, s.reactionsLeft("second"))

	s.react(mgr, "fighter", session.Take(""))
	s.Equal(0, s.reactionsLeft("fighter"), "taking spends the taker's reaction")
	beats := s.reactionBeats(mgr, "fighter")
	s.Require().Len(beats, 1, "the taken swing is told")
	s.Equal("fighter", beats[0].Actor)
	s.Equal([]string{"skel-1"}, beats[0].Targets)
	s.Equal(oaRef(), beats[0].Reaction.Ref, "told as the reaction it was")
	s.Equal("Opportunity Attack", beats[0].Reaction.Name)
	s.Equal(hexCell(3, 0), s.where(mgr, "skel-1"), "the step waits on the second answer (E8)")
	s.Require().NotEmpty(s.reactRow(mgr, "second").ID, "the second window still stands")

	s.react(mgr, "second", session.Decline())
	s.Equal(1, s.reactionsLeft("second"), "declining spends nothing")
	s.Len(s.reactionBeats(mgr, "fighter"), 1, "and swings at nobody")
	s.Equal(hexCell(5, 0), s.where(mgr, "skel-1"), "the last answer takes the step and resumes the turn")
}

// TestAReactRowStatesItsPrice: an opportunity row carries the price its pause
// states — one reaction — so the dock can say what taking it costs.
func (s *ReactWindowSuite) TestAReactRowStatesItsPrice() {
	mgr := s.twoSkeletons()
	s.endTurn(mgr, "fighter")
	row := s.reactRow(mgr, "fighter")
	s.Require().NotEmpty(row.ID)
	s.Equal(session.SlotReaction, row.Slot)
	s.Equal([]session.CostComponent{{Currency: session.CurrencyReaction, Needed: 1}}, row.Cost)
}

// TestTheZeroAnswerIsRefused: a ReactInput that names a real window and says
// nothing about what it chose is refused before anything is loaded — the
// session record is gone and the refusal is still the answer's, not the load's.
func (s *ReactWindowSuite) TestTheZeroAnswerIsRefused() {
	mgr := s.twoSkeletons()
	s.endTurn(mgr, "fighter")
	row := s.reactRow(mgr, "fighter")
	s.Require().NotEmpty(row.ID)

	delete(s.sessions.byID, "sess")
	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter", DeclarationID: row.ID,
	})
	s.Require().ErrorIs(err, session.ErrNotOffered)
}

// TestAStaleWindowIsRefused is ruling E5 on the session side: a window an
// earlier build posed — a pre-envelope pending_attack payload, or a payload
// whose version is not this build's — is refused with ErrStalePause before
// anything is resumed or charged.
func (s *ReactWindowSuite) TestAStaleWindowIsRefused() {
	for name, rewrite := range map[string]func([]byte) []byte{
		"pre-envelope pending_attack": func([]byte) []byte {
			return []byte(`{"kind":"pending_attack","audience":"fighter","attacker":"skel-1","target":"fighter",` +
				`"offer":{"ref":"dnd5e:conditions:opportunity_attack","name":"Opportunity Attack"},"frozen":"e30="}`)
		},
		"version 0": func(raw []byte) []byte {
			var window map[string]any
			s.Require().NoError(json.Unmarshal(raw, &window))
			window["version"] = 0
			out, err := json.Marshal(window)
			s.Require().NoError(err)
			return out
		},
	} {
		s.Run(name, func() {
			s.SetupTest()
			mgr := s.twoSkeletons()
			s.endTurn(mgr, "fighter")
			row := s.reactRow(mgr, "fighter")
			s.Require().NotEmpty(row.ID)

			stored := s.sessions.byID["sess"]
			rewritten := false
			for i := range stored.Windows.Windows {
				if string(stored.Windows.Windows[i].Audience) == "fighter" {
					stored.Windows.Windows[i].Payload = rewrite(stored.Windows.Windows[i].Payload)
					rewritten = true
				}
			}
			s.Require().True(rewritten, "control: the fighter's window is stored")

			_, err := mgr.React(context.Background(), &session.ReactInput{
				Session: "sess", Member: "fighter", DeclarationID: row.ID, Answer: session.Take(""),
			})
			s.Require().ErrorIs(err, session.ErrStalePause)
			s.Equal(1, s.reactionsLeft("fighter"), "nothing was charged")
			s.Empty(s.reactionBeats(mgr, "fighter"), "nothing was swung")
		})
	}
}
