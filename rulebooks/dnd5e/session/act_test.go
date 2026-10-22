// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// act_test.go pins the act id at the session seam (rpg-project#627,
// ideas/event-stream/design.md R3, R24, R27, Law 2): every player verb is one
// act, every beat of it carries the act's id (`<verb>:<ulid>`) as its
// correlation, a reaction carries its trigger's id, and a driven creature's
// walk, swing and pick are acts of their own.

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// storyOf is one member's whole story, beat by beat.
func storyOf(t *testing.T, mgr *session.Manager, member string) []witnessed {
	t.Helper()
	return storyBeats(t, mgr, "sess", member)
}

// eventsOf is every beat of one word, narrowed to the beats whose payload
// names field = value when field is not empty.
func eventsOf(t *testing.T, events []witnessed, beat, field, value string) []witnessed {
	t.Helper()
	var out []witnessed
	for _, e := range beatsNamed(events, beat) {
		if field != "" && e.Fields[field] != value {
			continue
		}
		out = append(out, e)
	}
	return out
}

// actID is the act id's shape: `<verb>:<ulid>`, the id minted from the
// record's own id space — never `<verb>:<seq>`.
var actID = regexp.MustCompile(`^[a-z_]+:[0-9A-HJKMNP-TV-Z]{26}$`)

// requireActID asserts act is one act id of the given verb.
func requireActID(t *testing.T, act, verb string) {
	t.Helper()
	require.True(t, strings.HasPrefix(act, verb+":"), "want a %s act, got %q", verb, act)
	require.Regexp(t, actID, act, "an act id is `<verb>:<ulid>`")
}

// requireOneAct asserts every beat carries one act id with the given verb
// as its correlation, and returns it.
func requireOneAct(t *testing.T, events []witnessed, verb string) string {
	t.Helper()
	require.NotEmpty(t, events)
	act := events[0].Correlation
	requireActID(t, act, verb)
	for _, e := range events {
		require.Equal(t, act, e.Correlation, "%s at seq %d is in another act", e.Beat, e.Seq)
	}
	return act
}

// TestWalkStepsShareOneAct: a walk is one verb, so its steps are one act.
func (s *MoveTestSuite) TestWalkStepsShareOneAct() {
	s.startCorridor()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 2}},
	})
	s.Require().NoError(err)

	moved := eventsOf(s.T(), storyOf(s.T(), s.mgr, "alice"), encounter.BeatMoved, "member", "alice")
	s.Require().Len(moved, 3)
	requireOneAct(s.T(), moved, "move")
}

// TestOpportunityAttackMidWalkCarriesWalkAct is Law 2 through a window: a
// skeleton walks out of the fighter's reach, she is asked, she strikes, and
// the strike and the rest of the walk carry the walk's id.
func (s *ReactWindowSuite) TestOpportunityAttackMidWalkCarriesWalkAct() {
	mgr := s.managerWith(&pathWalker{
		paths:  map[string][]spatial.Position{"skel-1": {hexCell(5, 0), hexCell(6, 0)}},
		walked: map[string]bool{},
	})
	s.start(mgr, []sceneSeat{seatAt("fighter", 3, 0)}, reactSkeleton("skel-1", 4, 0))

	s.endTurn(mgr, "fighter")
	opened := eventsOf(s.T(), storyOf(s.T(), mgr, "fighter"), encounter.BeatWindowOpened, "", "")
	s.Require().Len(opened, 1)
	walk := opened[0].Correlation
	requireActID(s.T(), walk, "move")

	s.react(mgr, "fighter", session.Take(""))

	events := storyOf(s.T(), mgr, "fighter")
	var swings []witnessed
	swings = append(swings, eventsOf(s.T(), events, string(encounter.OutcomeStruck), "", "")...)
	swings = append(swings, eventsOf(s.T(), events, string(encounter.OutcomeMissed), "", "")...)
	s.Require().Len(swings, 1, "one opportunity attack")
	s.Equal(walk, swings[0].Correlation, "the reaction carries its trigger's act")

	moved := eventsOf(s.T(), events, encounter.BeatMoved, "member", "skel-1")
	s.Require().Len(moved, 2, "the announced step and the one after it")
	for _, m := range moved {
		s.Equal(walk, m.Correlation, "the resumed walk is the same act")
	}

	// THE TURN ENDS IN THE ACT THE WALK WAS OPENED INSIDE. The skeleton's
	// walk began under the fighter's end_turn; the encounter's pause kept
	// that act beneath the walk's, so after React the skeleton's turn-ended
	// carries the end_turn id, not the walk's that React reopened.
	fighterEnded := eventsOf(s.T(), events, encounter.BeatTurnEnded, "member", "fighter")
	s.Require().NotEmpty(fighterEnded)
	endTurn := fighterEnded[len(fighterEnded)-1].Correlation
	requireActID(s.T(), endTurn, "end_turn")
	skelEnded := eventsOf(s.T(), events, encounter.BeatTurnEnded, "member", "skel-1")
	s.Require().NotEmpty(skelEnded)
	s.Equal(endTurn, skelEnded[len(skelEnded)-1].Correlation, "not the walk's %s", walk)
}

// TestAPlayersWalkAndTheBiteItProvokesShareOneAct is Law 2 with no window: a
// monster's automatic opportunity attack on a player's walk is the walk's.
func (s *ReactWindowSuite) TestAPlayersWalkAndTheBiteItProvokesShareOneAct() {
	mgr := s.managerWith(session.Pass{})
	s.start(mgr, []sceneSeat{seatAt("fighter", 2, 0)}, reactSkeleton("skel-1", 3, 0))

	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(1, 0)},
		DeclarationID: currentMoveID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)

	events := storyOf(s.T(), mgr, "fighter")
	moved := eventsOf(s.T(), events, encounter.BeatMoved, "member", "fighter")
	walk := requireOneAct(s.T(), moved, "move")
	var swings []witnessed
	swings = append(swings, eventsOf(s.T(), events, string(encounter.OutcomeStruck), "", "")...)
	swings = append(swings, eventsOf(s.T(), events, string(encounter.OutcomeMissed), "", "")...)
	s.Require().Len(swings, 1)
	s.Equal(walk, swings[0].Correlation, "the bite the walk provoked is the walk's")
}

// TestWindowActSurvivesSaveLoad: the window keeps the act it answers in the
// session record, and a manager that never saw the question reads it back.
func (s *ReactWindowSuite) TestWindowActSurvivesSaveLoad() {
	mgr := s.twoSkeletons()
	s.endTurn(mgr, "fighter")

	opened := eventsOf(s.T(), storyOf(s.T(), mgr, "fighter"), encounter.BeatWindowOpened, "", "")
	s.Require().Len(opened, 1)
	walk := opened[0].Correlation
	requireActID(s.T(), walk, "move")

	stored, err := s.sessions.GetSession(context.Background(), "sess")
	s.Require().NoError(err)
	s.Require().Len(stored.Windows.Windows, 1)
	var payload struct {
		Story struct {
			ActID string `json:"act_id"`
		} `json:"story"`
	}
	s.Require().NoError(json.Unmarshal(stored.Windows.Windows[0].Payload, &payload))
	s.Equal(walk, payload.Story.ActID, "the window's story names the act it answers")

	// A fresh manager over the same stores is the restart: it TAKES the
	// swing, and the swing is recorded by the session under the act React
	// reopened from the stored window — the one beat only the stored act can
	// name, since the encounter's own pause reopens the walk's cells either
	// way.
	fresh := s.managerWith(&pathWalker{paths: map[string][]spatial.Position{}, walked: map[string]bool{}})
	s.react(fresh, "fighter", session.Take(""))
	events := storyOf(s.T(), fresh, "fighter")
	var swings []witnessed
	swings = append(swings, eventsOf(s.T(), events, string(encounter.OutcomeStruck), "actor", "fighter")...)
	swings = append(swings, eventsOf(s.T(), events, string(encounter.OutcomeMissed), "actor", "fighter")...)
	s.Require().Len(swings, 1, "the opportunity attack she took")
	s.Equal(walk, swings[0].Correlation, "the swing carries the act the stored window named")
}

// TestStrikeAndDownedShareCorrelation is R3: a lethal swing and the fall it
// caused are one act.
func (s *AttackTestSuite) TestStrikeAndDownedShareCorrelation() {
	mgr := s.duel(&sequenceDice{rolls: []int{15, 5}})
	s.characters.byID["bob"].HitPoints = 1

	_, err := s.swing(mgr)
	s.Require().NoError(err)

	events := storyOf(s.T(), mgr, "alice")
	struck := eventsOf(s.T(), events, string(encounter.OutcomeStruck), "", "")
	downed := eventsOf(s.T(), events, string(encounter.OutcomeDown), "", "")
	s.Require().Len(struck, 1)
	s.Require().Len(downed, 1, "the blow drops bob")
	act := requireOneAct(s.T(), append(struck, downed...), "attack")
	s.NotEmpty(act)
}

// TestSpeakAnswerSharesSpeakAct: the goblin's answer to a line is the line's
// act, and its own pick on the world round the line cost is a time act.
func (s *SpeakWorldSuite) TestSpeakAnswerSharesSpeakAct() {
	s.driver = session.Driver()
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := appealLine()
		line.Success = []encounter.Answer{{Weight: 1, Say: "Fine."}}
		in.Options = []encounter.Option{line}
		in.Table = encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}
	}
	mgr := s.front([]int{10, 1, 1})

	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	events := storyOf(s.T(), mgr, "alice")
	spoke := eventsOf(s.T(), events, encounter.BeatSpoke, "", "")
	spoken := eventsOf(s.T(), events, encounter.BeatAnswered, "verb", "speak")
	s.Require().Len(spoke, 1)
	s.Require().Len(spoken, 1)
	speak := requireOneAct(s.T(), append(spoke, spoken...), "speak")

	picks := eventsOf(s.T(), events, encounter.BeatAnswered, "key", string(encounter.AnswerTime))
	s.Require().NotEmpty(picks, "the line cost the world a round and the goblin was asked")
	for _, p := range picks {
		requireActID(s.T(), p.Correlation, "time")
		s.NotEqual(speak, p.Correlation)
	}
}

// TestDrivenTurnActsAreSeparate is R24 at the seam: a zombie the fighter's
// end_turn hands the turn to walks, then slams, and those are two acts, both
// apart from the end_turn whose turn-ended beat it carries.
func (s *MonsterTurnTestSuite) TestDrivenTurnActsAreSeparate() {
	mgr := s.tombManager(session.Driver(), testDice{})
	launchScene(s.T(), mgr, tombWith(12, 6,
		monsterAt("zombie", refs.Monsters.Zombie().String(), 4, 0)))

	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "fighter", DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)

	events := storyOf(s.T(), mgr, "fighter")
	ended := eventsOf(s.T(), events, encounter.BeatTurnEnded, "member", "fighter")
	s.Require().NotEmpty(ended)
	endTurn := ended[len(ended)-1].Correlation
	requireActID(s.T(), endTurn, "end_turn")

	moved := eventsOf(s.T(), events, encounter.BeatMoved, "member", "zombie")
	s.Require().NotEmpty(moved, "the zombie closes before it can slam")
	walk := requireOneAct(s.T(), moved, "move")

	var swings []witnessed
	swings = append(swings, eventsOf(s.T(), events, string(encounter.OutcomeStruck), "actor", "zombie")...)
	swings = append(swings, eventsOf(s.T(), events, string(encounter.OutcomeMissed), "actor", "zombie")...)
	s.Require().Len(swings, 1, "one slam")
	swing := requireOneAct(s.T(), swings, "attack")

	s.NotEqual(walk, swing)
	s.NotEqual(endTurn, walk)
	s.NotEqual(endTurn, swing)
	zombieEnded := eventsOf(s.T(), events, encounter.BeatTurnEnded, "member", "zombie")
	s.Require().NotEmpty(zombieEnded)
	s.Equal(endTurn, zombieEnded[0].Correlation, "the driven turn ends inside the fighter's end_turn")
}
