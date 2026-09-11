// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CastPauseSuite is the flee that stops to ask, end to end through the two
// verbs a host actually calls.
//
// # It is one scene the old stack could not hold
//
// A directive that PROVOKES is new with Dissonant Whispers, and everything it
// touches was built for the other case. A push suppressed the swings, so no
// window ever opened during a directed walk; Direct answered a pause with an
// error; the cast verb read that error as a cast that failed to be recorded.
// The whole of what is proven here is that those three now agree: the walk is
// held, the cast commits, and the react verb knows which continue-verb owns
// the answer.
//
// Every test drives Manager.Cast and Manager.React and reads the story, because
// the claim is that five things agree — the mover seam poses, the composition
// holds the walk, the ledger persists the question, React answers it, and the
// held walk finishes. A unit test of any one of them passes with the other four
// wired wrong.
type CastPauseSuite struct {
	suite.Suite

	mgr        *session.Manager
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestCastPauseSuite(t *testing.T) {
	suite.Run(t, new(CastPauseSuite))
}

// whisperDice rolls a 10 on every d20 and a 1 on everything else.
//
// Chosen so both halves of the scene survive it: a 10 fails the bard's DC 13
// against a skeleton's Wisdom and hits its armour class on the reactor's swing,
// while a 1 on every damage die leaves the skeleton standing through both. A
// dropped creature is not asked to run and a dropped mover ends the step, so a
// roller that felled it would test neither.
type whisperDice struct{}

func (whisperDice) Roll(_ context.Context, size int) (int, error) {
	if size == 20 {
		return 10, nil
	}
	return 1, nil
}

// whisperingBard is the level-one bard with Dissonant Whispers on the sheet and
// the slot to pay for it.
func whisperingBard(id string) *character.Data {
	return castingBardWithSpells(id, spells.DissonantWhispers)
}

// armedWhisperingBard is the same bard with a longsword in hand, so she is a
// reactor as well as the caster.
func armedWhisperingBard(id string) *character.Data {
	bard := whisperingBard(id)
	armForSwinging(bard)
	return bard
}

// scene opens a tomb, joins the players at their cells, and spawns the skeleton
// the bard is going to whisper at. Returns nothing: every later read goes
// through a verb.
func (s *CastPauseSuite) scene(sheets []*character.Data, at map[string]spatial.Position, skeletonAt spatial.Position) {
	s.T().Helper()
	ctx := context.Background()

	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = newFakeCharacters(sheets...)

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: whisperDice{}, TurnDriver: session.Pass{},
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
		_, jerr := mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: sheet.ID, Position: cell})
		s.Require().NoError(jerr)
		// AFTER Join, never before: Join writes the joined member's sheet from
		// a freshly loaded character. A reactor with no reaction in hand is
		// never asked, which would make every scene here pass vacuously.
		s.inFight(sheet.ID)
	}

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(), Position: skeletonAt,
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "arriving in plain sight must start a fight")

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "bard"})
	s.Require().NoError(err)
	s.Require().Equal("bard", turn.Active, "the whisper is cast on the bard's own turn")
}

// inFight gives a stored sheet the economy of somebody in a fight, with an
// action to cast with and a reaction to be asked about.
func (s *CastPauseSuite) inFight(id string) {
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

// whisper casts Dissonant Whispers at the skeleton through the offered row.
func (s *CastPauseSuite) whisper() (*session.CastOutput, error) {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "bard"})
	s.Require().NoError(err)

	want := refs.Spells.DissonantWhispers().String()
	for _, row := range out.Declarations {
		if row.Verb != session.VerbCast || row.Spell == nil || row.Spell.Ref != want {
			continue
		}
		return s.mgr.Cast(context.Background(), &session.CastInput{
			Session: "sess", Member: "bard", Target: "skeleton", DeclarationID: row.ID,
		})
	}
	s.Require().FailNow("the bard was offered no Dissonant Whispers row")
	return nil, nil
}

// reactRow is one member's own open REACT declaration, or the zero value.
func (s *CastPauseSuite) reactRow(member string) session.Declaration {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	for _, row := range out.Declarations {
		if row.Verb == session.VerbReact {
			return row
		}
	}
	return session.Declaration{}
}

// react answers member's own open window.
func (s *CastPauseSuite) react(member string, choice session.ReactChoice) {
	s.T().Helper()
	row := s.reactRow(member)
	s.Require().NotEmpty(row.ID, "no open window for %q", member)
	_, err := s.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: member, DeclarationID: row.ID, Choice: choice,
	})
	s.Require().NoError(err)
}

// where is one member's cell.
func (s *CastPauseSuite) where(member string) spatial.Position {
	s.T().Helper()
	out, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Position
}

// story is one member's whole story, decoded to the fields these tests read.
func (s *CastPauseSuite) story(member string) []storyBeat {
	s.T().Helper()
	entries, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	out := make([]storyBeat, 0, len(entries))
	for _, entry := range entries {
		var beat storyBeat
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}
	return out
}

// fledCells counts the movement beats the skeleton's flight wrote, and proves
// each one names the whisper that caused it.
func (s *CastPauseSuite) fledCells() int {
	s.T().Helper()
	fled := 0
	for _, beat := range s.story("bard") {
		if beat.Beat == "moved" && beat.Member == "skeleton" {
			fled++
			s.Equal(refs.Spells.DissonantWhispers().String(), beat.Cause,
				"a creature running from a whisper did not choose to; every cell names the spell")
		}
	}
	return fled
}

// windowAudiences is every audience the story has been asked about, in order.
func (s *CastPauseSuite) windowAudiences(member string) []string {
	s.T().Helper()
	entries, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	var out []string
	for _, entry := range entries {
		var peek struct {
			Beat string `json:"beat"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &peek))
		if peek.Beat != "window_opened" {
			continue
		}
		var body windowBeat
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		for _, window := range body.Windows {
			out = append(out, window.Audience)
		}
	}
	return out
}

// swings is every beat in member's story that names a reaction.
func (s *CastPauseSuite) swings(member string) []reactedBeat {
	s.T().Helper()
	entries, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	var out []reactedBeat
	for _, entry := range entries {
		var body reactedBeat
		if json.Unmarshal(entry.Payload, &body) != nil || body.Reaction == nil {
			continue
		}
		out = append(out, body)
	}
	return out
}

// endBardsTurn is the freeze test that costs nothing to run: EndTurn is one of
// the whole-encounter gates a hold closes, so a turn that ends is a table that
// is no longer waiting on anybody.
func (s *CastPauseSuite) endBardsTurn() error {
	s.T().Helper()
	_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "bard",
		DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "bard"),
	})
	return err
}

// oneFighterScene: the bard whispers from two cells back and the fighter stands
// in the skeleton's way out.
func (s *CastPauseSuite) oneFighterScene() {
	s.scene(
		[]*character.Data{whisperingBard("bard"), armedFighter("fighter")},
		map[string]spatial.Position{"bard": hexCell(0, 0), "fighter": hexCell(1, 0)},
		hexCell(2, 0),
	)
}

// TestAFleeThatProvokesAPlayerPausesTheCastAndResumesOnTheAnswer is the whole
// slice in one scene, and the three claims are the three things that used to be
// impossible.
//
// THE CAST COMMITS. A pause is not a failed record: the pose has already
// written the windows into the session record by the time Direct answers, so
// the verb falls through to commit and reports the freeze on its own output.
//
// THE WALK IS HELD, NOT TAKEN. The skeleton still stands where the step was
// announced from while the fighter decides.
//
// THE REACT VERB PICKS THE RIGHT CONTINUE-VERB. Nobody's turn is paused here —
// it is the BARD's turn, and she is not the one walking — so resuming the turn
// would be the wrong answer and resuming the held directive is the right one.
func (s *CastPauseSuite) TestAFleeThatProvokesAPlayerPausesTheCastAndResumesOnTheAnswer() {
	s.oneFighterScene()
	announced := s.where("skeleton")

	out, err := s.whisper()
	s.Require().NoError(err, "a walk that stopped to ask is a checkpoint, not a failure")
	s.Require().NotNil(out)
	s.True(out.Paused, "the caster learns synchronously that the table is waiting")
	s.Require().NotNil(out.Saved)
	s.False(out.Saved.Succeeded, "only a failed save runs")

	s.Equal([]string{"fighter"}, s.windowAudiences("bard"),
		"the one player whose reach the skeleton is leaving was asked")
	row := s.reactRow("fighter")
	s.Require().NotEmpty(row.ID, "and the dock has a row to click")
	s.Require().NotNil(row.Reaction)
	s.Equal(oaRef(), row.Reaction.Ref)
	s.Equal(announced, s.where("skeleton"), "the announced step is NOT taken while the question stands")
	s.Zero(s.fledCells(), "no cell is walked until somebody answers")

	// And the freeze is real: the bard cannot end her own turn out from under
	// the question.
	s.Require().Error(s.endBardsTurn(), "the table does not advance while a player is deciding")

	s.react("fighter", session.ReactStrike)

	swings := s.swings("bard")
	s.Require().Len(swings, 1, "one swing, at the creature running past her")
	s.Equal("fighter", swings[0].Actor)
	s.Equal([]string{"skeleton"}, swings[0].Targets)
	s.Equal(oaRef(), swings[0].Reaction.Ref)
	s.Equal("Opportunity Attack", swings[0].Reaction.Name)

	s.Equal(6, s.fledCells(), "the held walk finished its thirty feet")
	s.NotEqual(announced, s.where("skeleton"), "and the skeleton is somewhere else")
	s.Empty(s.reactRow("fighter").ID, "nothing is being asked any more")
	s.Require().NoError(s.endBardsTurn(), "and the table moves again")
}

// TestAHeldSwingStillLetsTheWalkFinish — declining is the whole reason the
// question is asked (rpg-project#316), and it must cost the walk nothing.
func (s *CastPauseSuite) TestAHeldSwingStillLetsTheWalkFinish() {
	s.oneFighterScene()

	out, err := s.whisper()
	s.Require().NoError(err)
	s.True(out.Paused)

	s.react("fighter", session.ReactHold)

	s.Empty(s.swings("bard"), "holding swings at nobody")
	s.Equal(6, s.fledCells(), "and the creature runs the same thirty feet either way")
	s.Require().NoError(s.endBardsTurn())
}

// TestTheCasterIsAskedOnHerOwnTurn. The bard whispers at the skeleton standing
// next to her, and the whisper sends it past her own blade.
//
// A WINDOW ON YOUR OWN TURN IS NOT NEW TO THE WINDOW, and this is the first
// scene that produces one: every earlier pause was posed during somebody else's
// driven turn. It is the caster's own action that opened it, and she answers it
// mid-verb-sequence like anybody else.
func (s *CastPauseSuite) TestTheCasterIsAskedOnHerOwnTurn() {
	s.scene(
		[]*character.Data{armedWhisperingBard("bard")},
		map[string]spatial.Position{"bard": hexCell(1, 0)},
		hexCell(2, 0),
	)

	out, err := s.whisper()
	s.Require().NoError(err)
	s.True(out.Paused, "the caster is the reactor, and she is asked rather than swung for")
	s.Equal([]string{"bard"}, s.windowAudiences("bard"))

	s.react("bard", session.ReactStrike)

	swings := s.swings("bard")
	s.Require().Len(swings, 1)
	s.Equal("bard", swings[0].Actor)
	s.Equal([]string{"skeleton"}, swings[0].Targets)
	s.Equal(6, s.fledCells(), "and the walk she interrupted finishes")
}

// TestTwoPlayersAreBothAskedBeforeTheWalkResumes — ruling R3: every player
// reactor of one step is asked at once, and the walk waits for all of them. The
// first answer changes nothing but the ledger.
func (s *CastPauseSuite) TestTwoPlayersAreBothAskedBeforeTheWalkResumes() {
	s.scene(
		[]*character.Data{whisperingBard("bard"), armedFighter("fighter"), armedFighter("second")},
		// Both fighters are in reach of the skeleton's cell and out of reach of
		// the cell it runs to first, which is what makes one step two
		// questions. The bard stands two cells back: she is the anchor the
		// flight is measured from, not a reactor.
		map[string]spatial.Position{
			"bard": hexCell(0, 2), "fighter": hexCell(1, 2), "second": hexCell(2, 1),
		},
		hexCell(2, 2),
	)
	announced := s.where("skeleton")

	out, err := s.whisper()
	s.Require().NoError(err)
	s.True(out.Paused)
	s.ElementsMatch([]string{"fighter", "second"}, s.windowAudiences("bard"),
		"both players whose reach it is leaving were asked, on the one step")

	s.react("fighter", session.ReactStrike)
	s.Equal(announced, s.where("skeleton"),
		"one answer is not the answer; the walk waits for the second")
	s.Zero(s.fledCells())

	s.react("second", session.ReactHold)
	s.Equal(6, s.fledCells(), "answered by both, the held walk finishes")
	s.Require().NoError(s.endBardsTurn())
}
