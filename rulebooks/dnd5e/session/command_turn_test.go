// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The compelled turn, end to end through the two verbs a host calls: the bard
// Casts a word and somebody Ends a turn, and the creature under the order
// spends its whole turn obeying.
//
// # Why every test here goes through the verbs
//
// The claim this slice makes is that five things agree — participation says
// Driven, the wrapper takes the turn instead of the brain, resolution says what
// the word means, the composition routes and walks it, and the compulsion
// expires on the turn end it caused. A unit test of any one of them passes with
// the other four wired wrong, which is exactly the bug this suite exists to
// catch.
//
// # The driver records, and the recording is the assertion
//
// recordingBehavior wraps a real brain, so a compelled turn that leaked
// through to it would produce a view. Design §5.2 says the host's driver is
// NOT called for a compelled member; the zero-view assertions are that
// sentence, and they fail the moment the wrapper delegates.
type CommandTurnSuite struct {
	suite.Suite

	mgr        *session.Manager
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	driver     *recordingBehavior

	// behind is the brain the recorder wraps, for the one test that needs a
	// driver of its own. Nil means the real behavior, which is what every
	// compelled test wants: a compelled turn that leaked through to it would
	// produce a view.
	behind session.TurnDriver
}

func TestCommandTurnSuite(t *testing.T) {
	suite.Run(t, new(CommandTurnSuite))
}

// commandingBard is the level-one bard with Command on the sheet and the slot
// to pay for it.
func commandingBard(id string) *character.Data {
	return castingBardWithSpells(id, spells.Command)
}

// scene opens a tomb, joins the players at their cells, and spawns the
// skeleton. whisperDice rolls a 10 on every d20, which fails the bard's DC 13
// and leaves the skeleton standing through any damage it takes.
func (s *CommandTurnSuite) scene(
	sheets []*character.Data, at map[string]spatial.Position, skeletonAt spatial.Position,
) {
	s.T().Helper()
	ctx := context.Background()

	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = newFakeCharacters(sheets...)
	if s.behind == nil {
		s.behind = session.Behavior()
	}
	s.driver = &recordingBehavior{next: s.behind}

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: whisperDice{}, TurnDriver: s.driver,
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	for _, sheet := range sheets {
		cell, placed := at[sheet.ID]
		s.Require().True(placed, "the scene gave %q no cell", sheet.ID)
		_, jerr := mgr.Join(ctx, &session.JoinInput{
			Session: "sess", Member: sheet.ID, Position: cell,
		})
		s.Require().NoError(jerr)
		// AFTER Join, for CastPauseSuite's reason: Join writes the joined
		// member's sheet from a freshly loaded character, so an economy
		// written first would be overwritten.
		s.inFight(sheet.ID)
	}

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(), Position: skeletonAt,
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "arriving in plain sight must start a fight")

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "bard"})
	s.Require().NoError(err)
	s.Require().Equal("bard", turn.Active, "the word is spoken on the bard's own turn")
}

// duel is the plain scene: the bard alone with a skeleton four cells away.
func (s *CommandTurnSuite) duel() {
	s.scene(
		[]*character.Data{commandingBard("bard")},
		map[string]spatial.Position{"bard": hexCell(0, 0)},
		hexCell(4, 0),
	)
}

// inFight gives a stored sheet the economy of somebody in a fight.
func (s *CommandTurnSuite) inFight(id string) {
	s.T().Helper()
	ctx := context.Background()
	stored, err := s.characters.GetCharacter(ctx, id)
	s.Require().NoError(err)
	stored.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
		ReactionsRemaining: 1, MovementRemaining: 30,
	}
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))
}

// command casts Command at a target with one of the menu's words, through the
// offered row.
func (s *CommandTurnSuite) command(target, word string) (*session.CastOutput, error) {
	s.T().Helper()
	return s.commandFrom("bard", target, word)
}

// commandFrom is the same cast from a named caster, for the scenes where two
// people give orders.
func (s *CommandTurnSuite) commandFrom(caster, target, word string) (*session.CastOutput, error) {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: caster,
	})
	s.Require().NoError(err)

	want := refs.Spells.Command().String()
	for _, row := range out.Declarations {
		if row.Verb != session.VerbCast || row.Spell == nil || row.Spell.Ref != want {
			continue
		}
		return s.mgr.Cast(context.Background(), &session.CastInput{
			Session: "sess", Member: caster, DeclarationID: row.ID,
			Targets: []string{target}, Option: word,
		})
	}
	s.Require().FailNowf("no Command row", "%q was offered no Command row", caster)
	return nil, nil
}

// storedConditions is one member's raw condition blobs, from wherever their
// sheet lives.
func (s *CommandTurnSuite) storedConditions(member string) []json.RawMessage {
	s.T().Helper()
	data, err := s.sessions.GetSession(context.Background(), "sess")
	s.Require().NoError(err)
	for _, npc := range data.NPCs {
		if npc.ID == member {
			return npc.Conditions
		}
	}
	sheet, err := s.characters.GetCharacter(context.Background(), member)
	s.Require().NoError(err)
	return sheet.Conditions
}

// endTurn ends one member's turn, which is what lets the clock reach the
// creature under the order.
func (s *CommandTurnSuite) endTurn(member string) error {
	s.T().Helper()
	_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: member,
		DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", member),
	})
	return err
}

// where is one member's cell.
func (s *CommandTurnSuite) where(member string) spatial.Position {
	s.T().Helper()
	out, err := s.mgr.Where(context.Background(), &session.WhereInput{
		Session: "sess", Member: member,
	})
	s.Require().NoError(err)
	return out.Position
}

// story is one member's whole story, decoded to the fields these tests read.
func (s *CommandTurnSuite) story(member string) []storyBeat {
	s.T().Helper()
	entries, err := s.mgr.Story(context.Background(), &session.StoryInput{
		Session: "sess", Member: member,
	})
	s.Require().NoError(err)

	out := make([]storyBeat, 0, len(entries))
	for _, entry := range entries {
		var beat storyBeat
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}
	return out
}

// appliedSources is the source named by every condition-applied beat for one
// ref, in the order they were recorded — who the fight says put it there.
func (s *CommandTurnSuite) appliedSources(ref string) []string {
	s.T().Helper()
	entries, err := s.mgr.Story(context.Background(), &session.StoryInput{
		Session: "sess", Member: "bard",
	})
	s.Require().NoError(err)

	var out []string
	for _, entry := range entries {
		var beat struct {
			Result *struct {
				Kind     string `json:"kind"`
				Ref      string `json:"ref"`
				SourceID string `json:"source_id"`
			} `json:"result"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Result == nil || beat.Result.Kind != "condition-applied" || beat.Result.Ref != ref {
			continue
		}
		out = append(out, beat.Result.SourceID)
	}
	return out
}

// walkedCells counts the movement beats one member's compelled walk wrote and
// proves each names the compulsion rather than the member's own will.
func (s *CommandTurnSuite) walkedCells(member string) int {
	s.T().Helper()
	walked := 0
	for _, beat := range s.story("bard") {
		if beat.Beat == "moved" && beat.Member == member {
			walked++
			s.Equal(refs.Conditions.Commanded().String(), beat.Cause,
				"a creature that did not choose to walk must not be narrated as though it had")
		}
	}
	return walked
}

// viewsOf is every view the host's brain was shown about one member.
func (s *CommandTurnSuite) viewsOf(member string) []session.MonsterView {
	s.T().Helper()
	var out []session.MonsterView
	for _, view := range s.driver.views {
		if view.Self == member {
			out = append(out, view)
		}
	}
	return out
}

// compulsionOn decodes the compulsion a member's stored sheet is holding.
func (s *CommandTurnSuite) compulsionOn(member string) (*conditions.CommandedConditionData, bool) {
	s.T().Helper()
	data, err := s.sessions.GetSession(context.Background(), "sess")
	s.Require().NoError(err)
	for _, npc := range data.NPCs {
		if npc.ID == member {
			found, held, decodeErr := conditions.DecodeCommanded(npc.Conditions)
			s.Require().NoError(decodeErr)
			return found, held
		}
	}
	stored, err := s.characters.GetCharacter(context.Background(), member)
	s.Require().NoError(err)
	found, held, decodeErr := conditions.DecodeCommanded(stored.Conditions)
	s.Require().NoError(decodeErr)
	return found, held
}

// holdsRef reports whether a member's stored sheet carries one condition ref.
func (s *CommandTurnSuite) holdsRef(member, ref string) bool {
	s.T().Helper()
	data, err := s.sessions.GetSession(context.Background(), "sess")
	s.Require().NoError(err)
	var stored []json.RawMessage
	for _, npc := range data.NPCs {
		if npc.ID == member {
			stored = npc.Conditions
		}
	}
	if stored == nil {
		sheet, charErr := s.characters.GetCharacter(context.Background(), member)
		s.Require().NoError(charErr)
		stored = sheet.Conditions
	}
	for _, raw := range stored {
		var peek struct {
			Ref string `json:"ref"`
		}
		if json.Unmarshal(raw, &peek) == nil && peek.Ref == ref {
			return true
		}
	}
	return false
}

// TestTheWordLandsAndNothingMovesYet is acceptance 2: the cast writes the
// compulsion and stops there. The walk belongs to the creature's own turn, and
// a spell that moved somebody the moment it was spoken would have made the
// whole compelled turn unobservable.
func (s *CommandTurnSuite) TestTheWordLandsAndNothingMovesYet() {
	s.duel()
	before := s.where("skeleton")

	out, err := s.command("skeleton", spells.CommandWordApproach)
	s.Require().NoError(err)
	s.Require().NotNil(out.Saved)
	s.False(out.Saved.Succeeded, "a 10 against DC 13 fails, and only a failure compels")

	commanded, held := s.compulsionOn("skeleton")
	s.Require().True(held)
	s.Equal(spells.CommandWordApproach, commanded.Word)
	s.Equal("bard", commanded.CasterID)
	s.Equal(before, s.where("skeleton"), "the order was given; the turn has not come round")
	s.Zero(s.walkedCells("skeleton"))
}

// TestApproachWalksTheCreatureToTheCasterWithoutAskingItsBrain is acceptance
// 4, and the zero-view assertion is design §5.2 held as a test: the host's
// driver is not consulted for a compelled member.
//
// The skeleton stops BESIDE the bard rather than on her, walks the shortest
// route, and its turn ends with the walk. Every cell names the compulsion,
// because a creature that did not choose to walk must not be narrated as
// though it had.
func (s *CommandTurnSuite) TestApproachWalksTheCreatureToTheCasterWithoutAskingItsBrain() {
	s.duel()

	_, err := s.command("skeleton", spells.CommandWordApproach)
	s.Require().NoError(err)
	s.Require().NoError(s.endTurn("bard"))

	s.Empty(s.viewsOf("skeleton"),
		"a compelled creature's brain is never asked; a wrapper that delegated would leave a view here")
	s.Equal(hexCell(1, 0), s.where("skeleton"), "beside the bard, and not on her")
	s.Equal(3, s.walkedCells("skeleton"), "four cells apart, three cells of walking")

	ended := 0
	for _, beat := range s.story("bard") {
		if beat.Beat == "turn-ended" && beat.Member == "skeleton" {
			ended++
		}
	}
	s.Equal(1, ended, "the walk IS the turn, so the turn ends with it")
}

// TestFleeWalksTheCreatureAwayAndEndsTheTurn is acceptance 6. The same
// machinery with the policy turned around, which is what makes the pair worth
// having: a driver that hardcoded one direction passes the Approach test.
func (s *CommandTurnSuite) TestFleeWalksTheCreatureAwayAndEndsTheTurn() {
	s.duel()
	before := s.where("skeleton")

	_, err := s.command("skeleton", spells.CommandWordFlee)
	s.Require().NoError(err)
	s.Require().NoError(s.endTurn("bard"))

	s.Empty(s.viewsOf("skeleton"))
	after := s.where("skeleton")
	s.NotEqual(before, after)
	s.Greater(after.X, before.X, "the bard is at the low end of the hall; running means away from her")
	s.Positive(s.walkedCells("skeleton"))
}

// TestGrovelLeavesTheCreatureProneAndTheTurnEnds is acceptance 7, and it is
// the word that proves the save is not optional: the prone is applied on
// resolution's bus and has to reach disk, or the next verb sees a creature
// standing up.
func (s *CommandTurnSuite) TestGrovelLeavesTheCreatureProneAndTheTurnEnds() {
	s.duel()
	before := s.where("skeleton")

	_, err := s.command("skeleton", spells.CommandWordGrovel)
	s.Require().NoError(err)
	s.Require().NoError(s.endTurn("bard"))

	s.Empty(s.viewsOf("skeleton"))
	s.True(s.holdsRef("skeleton", refs.Conditions.Prone().String()),
		"a grovelling creature is prone on the sheet a later verb reads")
	s.Equal(before, s.where("skeleton"), "falling down is not walking")
	s.Zero(s.walkedCells("skeleton"))
}

// TestTheCompulsionIsGoneAfterTheTurnItTook is acceptance 8. One turn end, and
// it is the creature's own — which is the whole of "until the end of your next
// turn" without a new duration mechanism.
//
// The following turn proves the other half: the brain IS asked again, so the
// wrapper stopped wrapping rather than the condition merely stopping counting.
func (s *CommandTurnSuite) TestTheCompulsionIsGoneAfterTheTurnItTook() {
	s.duel()

	_, err := s.command("skeleton", spells.CommandWordApproach)
	s.Require().NoError(err)
	s.Require().NoError(s.endTurn("bard"))

	_, held := s.compulsionOn("skeleton")
	s.False(held, "the compulsion expired on the turn end it caused")

	// Round two: the bard does nothing, and the skeleton's turn is its own.
	s.Require().NoError(s.endTurn("bard"))
	s.NotEmpty(s.viewsOf("skeleton"),
		"a creature no longer under an order is driven by its own brain again")
}

// TestASecondWordReplacesTheFirst is acceptance 11, and its scope narrowed once
// the compulsion's address grew a caster: ONE CASTER cannot have two words in
// force at once, because the second application replaces the first at the same
// address. Two DIFFERENT casters is the other case and it stands — see
// TestTwoCastersCommandsBothStandAndTheNewestIsObeyed.
//
// The sheet is counted rather than just read, because that is now the whole
// difference between the two rules: replacement leaves one blob and coexistence
// leaves two, and a test that only asked which word was in force would pass
// either way.
func (s *CommandTurnSuite) TestASecondWordReplacesTheFirst() {
	s.duel()

	_, err := s.command("skeleton", spells.CommandWordApproach)
	s.Require().NoError(err)
	first, held := s.compulsionOn("skeleton")
	s.Require().True(held)
	s.Equal(spells.CommandWordApproach, first.Word)

	// A second cast needs a second action; the fight gives her one next round.
	s.Require().NoError(s.endTurn("bard"))
	s.Require().NoError(s.endTurn("bard"))
	s.inFight("bard")

	_, err = s.command("skeleton", spells.CommandWordGrovel)
	s.Require().NoError(err)
	second, held := s.compulsionOn("skeleton")
	s.Require().True(held)
	s.Equal(spells.CommandWordGrovel, second.Word, "the newer word is the one in force")

	commanded := 0
	for _, raw := range s.storedConditions("skeleton") {
		var peek struct {
			Ref string `json:"ref"`
		}
		s.Require().NoError(json.Unmarshal(raw, &peek))
		if peek.Ref == refs.Conditions.Commanded().String() {
			commanded++
		}
	}
	s.Equal(1, commanded,
		"one caster holds one address, so the second word replaced the first rather than joining it")
}

// TestACommandedPlayerIsDrivenAndCannotActAfterwards is acceptance 9, and the
// reason a player is worth a test of its own: there is no brain to fall back
// on, so a wrapper that delegated for a player would fail differently from one
// that delegated for a monster.
//
// The fighter's turn is taken by the composition and then ENDED, so the verb
// he would have used is refused — not because he is compelled, but because it
// is no longer his turn, which is the ordinary refusal every player verb keeps.
func (s *CommandTurnSuite) TestACommandedPlayerIsDrivenAndCannotActAfterwards() {
	s.scene(
		[]*character.Data{commandingBard("bard"), armedFighter("fighter")},
		map[string]spatial.Position{"bard": hexCell(0, 0), "fighter": hexCell(4, 0)},
		hexCell(8, 0),
	)

	_, err := s.command("fighter", spells.CommandWordApproach)
	s.Require().NoError(err)
	commanded, held := s.compulsionOn("fighter")
	s.Require().True(held)
	s.Equal(spells.CommandWordApproach, commanded.Word)

	s.Require().NoError(s.endTurn("bard"))

	s.Equal(hexCell(1, 0), s.where("fighter"), "driven to the bard's side, exactly as a monster is")
	s.Positive(s.walkedCells("fighter"))

	_, err = s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "fighter",
		Path: []spatial.Position{hexCell(2, 0)},
	})
	s.ErrorIs(err, session.ErrNotYourTurn,
		"his turn was spent obeying, and the clock has moved on")
}

// TestADefeatedCommandedCreatureTakesNoCompelledTurn is acceptance 10 from the
// monster side: a creature the fight has removed is not brought back to be
// marched around, and the compulsion expires unused.
//
// # Grovel is the probe, and that is the whole point of the word choice
//
// Approach would prove only that nobody walked, which a broken driver that
// asked resolution and then dropped the answer would also satisfy. Grovel
// APPLIES a condition and the condition is persisted, so an absent Prone is
// the observable form of "Obey never ran" — there is no path by which the word
// could have been obeyed and left the sheet clean.
//
// The participation half of the same ruling is pinned one layer down, in
// TestAMemberWhoIsNotUpIsNeverDriven: the assessment answers AutoPass or
// Remove for these members and never Driven.
//
// WHAT THIS TEST CANNOT CATCH, stated so nobody reads more into it: forcing
// Driven onto a REMOVED member leaves it still passing, because the
// composition drops a removed member's slot whatever this seam says. The
// mutation-sensitive sibling is
// TestADyingCommandedPlayerGetsTheirDeathSaveTurn, where the member really is
// on the clock and only this seam's answer keeps the turn theirs.
func (s *CommandTurnSuite) TestADefeatedCommandedCreatureTakesNoCompelledTurn() {
	s.duel()
	before := s.where("skeleton")

	_, err := s.command("skeleton", spells.CommandWordGrovel)
	s.Require().NoError(err)
	s.Require().True(s.holdsRef("skeleton", refs.Conditions.Commanded().String()),
		"the order landed before the creature fell")

	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == "skeleton" {
			s.sessions.byID["sess"].NPCs[i].HitPoints = 0
		}
	}

	s.Require().NoError(s.endTurn("bard"))
	s.False(s.holdsRef("skeleton", refs.Conditions.Prone().String()),
		"the word was never obeyed: a Prone here would be Obey having run for a removed member")
	s.Equal(before, s.where("skeleton"), "a felled creature obeys nothing")
	s.Zero(s.walkedCells("skeleton"))
	s.Empty(s.viewsOf("skeleton"))
}

// TestADyingCommandedPlayerGetsTheirDeathSaveTurn is the same ruling from the
// side the design did not see, and the reason the gate is "can this member act"
// rather than "is this member waited for".
//
// A DYING player Waits — their turn is the death save they have to roll — so a
// compulsion that narrowed Wait alone would have marched a body across the room
// and skipped the save. Here the clock RESTS on the fighter, which is the
// observable difference between a turn taken for somebody and a turn that is
// still theirs to take.
//
// Grovel again, for the test above's reason: a clean sheet is the only way to
// see that Obey was never asked.
func (s *CommandTurnSuite) TestADyingCommandedPlayerGetsTheirDeathSaveTurn() {
	s.scene(
		[]*character.Data{commandingBard("bard"), armedFighter("fighter")},
		map[string]spatial.Position{"bard": hexCell(0, 0), "fighter": hexCell(4, 0)},
		hexCell(8, 0),
	)
	before := s.where("fighter")

	_, err := s.command("fighter", spells.CommandWordGrovel)
	s.Require().NoError(err)
	s.Require().True(s.holdsRef("fighter", refs.Conditions.Commanded().String()))

	s.characters.byID["fighter"].HitPoints = 0
	s.Require().NoError(s.endTurn("bard"))

	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{
		Session: "sess", Member: "fighter",
	})
	s.Require().NoError(err)
	s.Equal("fighter", turn.Active,
		"the clock rests on him: this turn is his death save, not somebody else's order")

	s.False(s.holdsRef("fighter", refs.Conditions.Prone().String()),
		"the word was never obeyed; a Prone here would be a body grovelling instead of saving")
	s.Equal(before, s.where("fighter"))
	s.Zero(s.walkedCells("fighter"))
}

// TestACompelledWalkThatProvokesHoldsTheTurnAndResumesOnTheAnswer is
// acceptance 5, and it is the test that proves a compelled walk is an ordinary
// walk: it leaves a reach, somebody is asked, the turn HOLDS while they decide,
// and answering finishes both the walk and the turn.
//
// # The window names the compulsion
//
// A player asked to swing at a creature running past them is being told why it
// is running. The beat carries the cause for the same reason every moved beat
// does: nothing here happened of the creature's own accord.
//
// # And the turn ends without a second Act
//
// Routed is terminal. A resumed walk that re-entered the driver would give the
// creature a second decision it never had, and the recording driver is what
// catches it: still zero views for the skeleton after the answer.
func (s *CommandTurnSuite) TestACompelledWalkThatProvokesHoldsTheTurnAndResumesOnTheAnswer() {
	s.scene(
		[]*character.Data{commandingBard("bard"), armedFighter("fighter")},
		map[string]spatial.Position{"bard": hexCell(0, 0), "fighter": hexCell(1, 0)},
		hexCell(2, 0),
	)
	announced := s.where("skeleton")

	_, err := s.command("skeleton", spells.CommandWordFlee)
	s.Require().NoError(err)
	s.Require().NoError(s.endTurn("bard"))
	s.Require().NoError(s.endTurn("fighter"))

	windows := 0
	for _, beat := range s.story("bard") {
		if beat.Beat == "window_opened" {
			windows++
			s.Equal(refs.Conditions.Commanded().String(), beat.Cause,
				"the player being asked is told what sent the creature past them")
		}
	}
	s.Equal(1, windows, "the one player whose reach the skeleton is leaving was asked")
	s.Equal(announced, s.where("skeleton"),
		"the announced step is NOT taken while the question stands")
	s.Zero(s.walkedCells("skeleton"), "no cell is walked until somebody answers")

	row := s.reactRow("fighter")
	s.Require().NotEmpty(row.ID, "and the dock has a row to click")
	_, err = s.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "fighter", DeclarationID: row.ID,
		Choice: session.ReactHold,
	})
	s.Require().NoError(err)

	s.NotEqual(announced, s.where("skeleton"), "answering finishes the walk")
	s.Positive(s.walkedCells("skeleton"))
	s.Empty(s.viewsOf("skeleton"),
		"a resumed compelled turn asks nobody a second time: Routed is terminal")

	ended := 0
	for _, beat := range s.story("bard") {
		if beat.Beat == "turn-ended" && beat.Member == "skeleton" {
			ended++
		}
	}
	s.Equal(1, ended, "and the turn ends with the walk it resumed")
}

// reactRow is one member's own open REACT declaration, or the zero value.
func (s *CommandTurnSuite) reactRow(member string) session.Declaration {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: member,
	})
	s.Require().NoError(err)
	for _, row := range out.Declarations {
		if row.Verb == session.VerbReact {
			return row
		}
	}
	return session.Declaration{}
}

// runsOnce hands back one Routed for the named member's first turn and passes
// on every other, which is what a monster deciding to run looks like from the
// seam's side: a policy, an anchor, and its own cause.
type runsOnce struct {
	member string
	anchor string
	cause  string
	asked  int
}

func (r *runsOnce) Act(view session.MonsterView) (session.TurnIntent, error) {
	if view.Self != r.member {
		return session.Pass{}, nil
	}
	r.asked++
	if r.asked > 1 {
		return session.Pass{}, nil
	}
	return session.Routed{
		Policy: session.MoveAway, Anchor: r.anchor, Cause: r.cause,
	}, nil
}

// TestAHostDriversRoutedReachesTheCompositionThroughTheSeam is the hand-off
// this PR leaves for the monster-flee lane, driven end to end.
//
// # Why the unit test beside it was not enough
//
// A reviewer deleted both Routed arms from turnDriverSeam.Act and the package
// stayed green: the compelled path builds encounter.Routed directly and never
// touches that switch, and the translation's own test calls
// routedToEncounter. So the DISPATCH — a host's session.Routed being
// recognised at all — had no test, and the one customer the PR names for it is
// a host driver.
//
// Nobody is compelled here. The skeleton runs because its own brain said so,
// and the beats carry the brain's own cause rather than the compulsion's,
// which is what proves all three fields crossed rather than being supplied by
// the compelled path.
func (s *CommandTurnSuite) TestAHostDriversRoutedReachesTheCompositionThroughTheSeam() {
	brain := &runsOnce{
		member: "skeleton", anchor: "bard",
		cause: refs.Conditions.Frightened().String(),
	}
	s.behind = brain
	s.scene(
		[]*character.Data{commandingBard("bard")},
		map[string]spatial.Position{"bard": hexCell(0, 0)},
		hexCell(2, 0),
	)
	before := s.where("skeleton")

	s.Require().NoError(s.endTurn("bard"))

	s.Equal(1, brain.asked, "the brain WAS asked: this member holds no compulsion")
	after := s.where("skeleton")
	s.NotEqual(before, after, "and the composition walked the route it was handed")
	s.Greater(after.X, before.X, "away from the anchor it named")

	walked := 0
	for _, beat := range s.story("bard") {
		if beat.Beat == "moved" && beat.Member == "skeleton" {
			walked++
			s.Equal(refs.Conditions.Frightened().String(), beat.Cause,
				"the driver's own cause crossed, not the compulsion's")
		}
	}
	s.Positive(walked)

	ended := 0
	for _, beat := range s.story("bard") {
		if beat.Beat == "turn-ended" && beat.Member == "skeleton" {
			ended++
		}
	}
	s.Equal(1, ended, "Routed is terminal for a host driver too")
	s.Equal(1, brain.asked, "and terminal means it is not asked again within the turn")
}

// TestTheAppliedBeatNamesWhoSpoke is the observable half of the compulsion's
// address carrying its caster: a table watching the log learns WHO the creature
// is now under orders from, which is the same fact the walk will later be
// measured against.
func (s *CommandTurnSuite) TestTheAppliedBeatNamesWhoSpoke() {
	s.duel()

	_, err := s.command("skeleton", spells.CommandWordApproach)
	s.Require().NoError(err)

	s.Equal([]string{"bard"}, s.appliedSources(refs.Conditions.Commanded().String()),
		"the caster, not the spell and not the target")
}

// TestTwoCastersCommandsBothStandAndTheNewestIsObeyed is the rule the
// caster-addressed compulsion created, driven end to end.
//
// Nothing removes another caster's spell, so the skeleton really is holding two
// orders when its turn arrives — and the second cast leaves the first's beat
// alone, which is how a table can see both. One turn cannot obey two words, so
// the driver takes the one said last.
//
// Grovel is the newest word on purpose: it leaves a Prone, which is a durable
// fact about WHICH word ran. Approach would only have shown that something
// walked somewhere, and the two orders here point at different casters standing
// in different places, so a walk would be a weaker signal than a condition.
func (s *CommandTurnSuite) TestTwoCastersCommandsBothStandAndTheNewestIsObeyed() {
	s.scene(
		[]*character.Data{commandingBard("bard"), commandingBard("cleric")},
		map[string]spatial.Position{"bard": hexCell(0, 0), "cleric": hexCell(1, 1)},
		hexCell(4, 0),
	)
	before := s.where("skeleton")

	_, err := s.command("skeleton", spells.CommandWordApproach)
	s.Require().NoError(err)
	s.Require().NoError(s.endTurn("bard"))

	_, err = s.commandFrom("cleric", "skeleton", spells.CommandWordGrovel)
	s.Require().NoError(err)

	stored := s.storedConditions("skeleton")
	commanded := 0
	for _, raw := range stored {
		var peek struct {
			Ref      string `json:"ref"`
			CasterID string `json:"caster_id"`
		}
		s.Require().NoError(json.Unmarshal(raw, &peek))
		if peek.Ref == refs.Conditions.Commanded().String() {
			commanded++
		}
	}
	s.Equal(2, commanded,
		"nothing removes another caster's spell: both orders stand, each on its own clock")
	s.Equal([]string{"bard", "cleric"}, s.appliedSources(refs.Conditions.Commanded().String()),
		"and the log names both, in the order they were spoken")

	s.Require().NoError(s.endTurn("cleric"))

	s.True(s.holdsRef("skeleton", refs.Conditions.Prone().String()),
		"the word said last is the one obeyed")
	s.Equal(before, s.where("skeleton"),
		"and the older Approach did not also run: grovelling is not walking")
	s.Zero(s.walkedCells("skeleton"))
	s.Empty(s.viewsOf("skeleton"))
}
