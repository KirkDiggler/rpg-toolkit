// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// act_test.go pins the act id (rpg-project#627, ideas/event-stream/design.md
// R3, R20, R24, Law 2): one act is one verb, minted as `<verb>:<first seq>`,
// and every beat appended inside it carries that id as its correlation. A
// reaction carries its trigger's id, and a driven activation is one act per
// verb rather than part of the act that set it off.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

type ActTestSuite struct {
	deathScene
}

func TestActSuite(t *testing.T) {
	suite.Run(t, new(ActTestSuite))
}

// told is one story entry as a test reads it: its kind, its decoded payload,
// and the record fields this file is about.
type told struct {
	kind        string
	seq         uint64
	at          uint64
	correlation string
	beat        map[string]any
}

// story is one audience's whole story, decoded, in order.
func (s *ActTestSuite) story(enc *encounter.Encounter, audience encounter.MemberID) []told {
	s.T().Helper()
	entries, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	out := make([]told, 0, len(entries))
	for _, entry := range entries {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		kind, _ := beat["beat"].(string)
		out = append(out, told{kind: kind, seq: entry.Seq, at: entry.At, correlation: entry.Correlation, beat: beat})
	}
	return out
}

// ofKind is every entry of one kind, optionally narrowed to one member.
func ofKind(entries []told, kind string, member encounter.MemberID) []told {
	var out []told
	for _, e := range entries {
		if e.kind != kind {
			continue
		}
		if member != "" && e.beat["member"] != string(member) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// open opens one act and returns its id.
func (s *ActTestSuite) open(enc *encounter.Encounter, verb, act string) string {
	s.T().Helper()
	out, err := enc.OpenAct(&encounter.OpenActInput{Verb: verb, Act: act})
	s.Require().NoError(err)
	return out.Act
}

// missAt records one missed swing from alice at the goblin.
func (s *ActTestSuite) missAt(enc *encounter.Encounter) {
	s.T().Helper()
	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	s.Require().NoError(err)
}

// TestOpenActMintsVerbAndFirstSeq is R20: the id is the verb and the seq
// the act's first beat takes, which is the story's next seq at open.
func (s *ActTestSuite) TestOpenActMintsVerbAndFirstSeq() {
	enc := s.pair(&downList{})

	n, err := enc.NextStorySeq()
	s.Require().NoError(err)
	act := s.open(enc, "attack", "")
	s.Equal(fmt.Sprintf("attack:%d", n), act)

	s.missAt(enc)
	missed := ofKind(s.story(enc, alice), string(encounter.OutcomeMissed), "")
	s.Require().Len(missed, 1)
	s.Equal(n, missed[0].seq, "the act's first beat takes the seq its id names")
	s.Equal(act, missed[0].correlation)
	s.Require().NoError(enc.CloseAct())
}

// TestOpenActWithNothingToNameIsRefused: an act with no verb and no id has
// nothing to be called, and closing an act nobody opened is a caller bug.
func (s *ActTestSuite) TestOpenActWithNothingToNameIsRefused() {
	enc := s.pair(&downList{})

	_, err := enc.OpenAct(&encounter.OpenActInput{})
	s.ErrorIs(err, encounter.ErrInvalidData)
	_, err = enc.OpenAct(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
	s.ErrorIs(enc.CloseAct(), encounter.ErrInvalidData)
}

// TestStrikeAndDownedShareAct is R3: the strike and the body it dropped are
// one act, and so is the fight ending the body caused.
func (s *ActTestSuite) TestStrikeAndDownedShareAct() {
	down := &downList{}
	enc := s.pair(down)

	act := s.open(enc, "attack", "")
	down.down = []encounter.MemberID{goblin}
	_, err := s.blowFrom(enc)
	s.Require().NoError(err)
	s.Require().NoError(enc.CloseAct())

	entries := s.story(enc, alice)
	struck := ofKind(entries, string(encounter.OutcomeStruck), "")
	downed := ofKind(entries, "down", "")
	s.Require().Len(struck, 1)
	s.Require().Len(downed, 1)
	s.Equal(act, struck[0].correlation)
	s.Equal(act, downed[0].correlation, "the body is the strike's own consequence")
}

// blowFrom records one landed strike from alice at the goblin.
func (s *ActTestSuite) blowFrom(enc *encounter.Encounter) (uint64, error) {
	out, err := recordOne(enc, &encounter.RecordInput{
		Kind:    encounter.OutcomeStruck,
		Actor:   alice,
		Targets: []encounter.MemberID{goblin},
		Values:  map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	if err != nil {
		return 0, err
	}
	return out.Seq, nil
}

// TestBeatOutsideActCarriesNone: absence means outside any act, and the
// scene-opened beat is stamped with the world clock's reading (R4).
func (s *ActTestSuite) TestBeatOutsideActCarriesNone() {
	enc := s.pair(&downList{})

	opened := ofKind(s.story(enc, alice), encounter.BeatSceneOpened, "")
	s.Require().Len(opened, 1)
	s.Empty(opened[0].correlation, "nobody's verb opened the scene")
	s.Equal(uint64(enc.ToData().Clock.HighWater), opened[0].at)

	s.missAt(enc)
	missed := ofKind(s.story(enc, alice), string(encounter.OutcomeMissed), "")
	s.Require().Len(missed, 1)
	s.Empty(missed[0].correlation, "a beat recorded with no act open carries none")
}

// TestReopenedActKeepsId is Law 2's mechanism: a reaction reopens its
// trigger's act, and its beats carry the trigger's id, not a new one.
func (s *ActTestSuite) TestReopenedActKeepsId() {
	enc := s.pair(&downList{})

	act := s.open(enc, "react", "move:41")
	s.Equal("move:41", act)

	s.missAt(enc)
	missed := ofKind(s.story(enc, alice), string(encounter.OutcomeMissed), "")
	s.Require().Len(missed, 1)
	s.Equal("move:41", missed[0].correlation)
	s.Require().NoError(enc.CloseAct())
}

// TestCloseActRestoresTheOuterAct: acts nest, and the inner one names the
// beats only while it is open.
func (s *ActTestSuite) TestCloseActRestoresTheOuterAct() {
	enc := s.pair(&downList{})

	s.Empty(enc.CurrentAct(), "nothing is open yet")
	outer := s.open(enc, "attack", "")
	inner := s.open(enc, "react", "move:7")
	s.Equal(inner, enc.CurrentAct())
	s.missAt(enc)
	s.Require().NoError(enc.CloseAct())
	s.Equal(outer, enc.CurrentAct(), "closing the inner act restores the outer")
	s.missAt(enc)
	s.Require().NoError(enc.CloseAct())
	s.Empty(enc.CurrentAct())

	missed := ofKind(s.story(enc, alice), string(encounter.OutcomeMissed), "")
	s.Require().Len(missed, 2)
	s.Equal(inner, missed[0].correlation)
	s.Equal(outer, missed[1].correlation)
}

// drivenScene is the pause suite's one-room fight — alice at (2,2), the
// goblin at (6,2) — with the driver, striker and mover named.
func (s *ActTestSuite) drivenScene(
	driver encounter.Driver, striker encounter.Striker, mover encounter.Mover,
) *encounter.Encounter {
	s.T().Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Retention: encounter.RetentionUnbounded,
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 2}},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        pauseSheets,
			Participation: &downList{},
			Initiative:    orderAsGiven{},
			Driver:        driver,
			Actors: encounter.Actors{
				Striker:   striker,
				Mover:     mover,
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

// closeIn is the three-cell walk that brings the goblin beside alice.
var closeIn = []spatial.Position{{X: 5, Y: 2}, {X: 4, Y: 2}, {X: 3, Y: 2}}

// closeThenSwing walks the path its own view offers to alice, then swings at
// her: a driven walk-then-attack whose walk is the view's, so the swing is
// in reach on whatever geometry the scene has.
type closeThenSwing struct {
	calls int
	path  int
}

func (d *closeThenSwing) Act(view encounter.MonsterView) (encounter.Decision, error) {
	d.calls++
	switch d.calls {
	case 1:
		for _, sm := range view.Seen {
			if sm.ID == alice {
				d.path = len(sm.Path)
				return encounter.Decision{Intent: encounter.Move{Path: sm.Path, Target: alice}}, nil
			}
		}
	case 2:
		return encounter.Decision{Intent: encounter.Attack{Target: alice, Action: testMeleeAction}}, nil
	}
	return encounter.Decision{Intent: encounter.Pass{}}, nil
}

// TestDrivenWalkThenAttackIsTwoActs is R24: a driven walk-then-attack is
// two acts, neither of them the player's end_turn that set the drive off.
func (s *ActTestSuite) TestDrivenWalkThenAttackIsTwoActs() {
	driver := &closeThenSwing{}
	enc := s.drivenScene(driver, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})

	endTurn := s.open(enc, "end_turn", "")
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().NoError(enc.CloseAct())

	entries := s.story(enc, alice)
	s.Require().GreaterOrEqual(driver.path, 2, "the goblin has a walk of several cells to take")
	moved := ofKind(entries, encounter.BeatMoved, goblin)
	s.Require().Len(moved, driver.path)
	walk := moved[0].correlation
	s.True(strings.HasPrefix(walk, "move:"), "the walk is a move act, got %q", walk)
	for _, m := range moved {
		s.Equal(walk, m.correlation, "every step of one walk is one act")
	}

	missed := ofKind(entries, string(encounter.OutcomeMissed), "")
	s.Require().Len(missed, 1)
	swing := missed[0].correlation
	s.True(strings.HasPrefix(swing, "attack:"), "the swing is an attack act, got %q", swing)
	s.NotEqual(walk, swing)

	ended := ofKind(entries, encounter.BeatTurnEnded, "")
	s.Require().NotEmpty(ended)
	for _, e := range ended {
		s.Equal(endTurn, e.correlation, "the turn ends inside the player's end_turn act")
	}
	s.NotEqual(endTurn, walk)
	s.NotEqual(endTurn, swing)
}

// actSeeingMover is a pausingMover that records the act open at every step
// it is told about — what a host's Mover reads when it poses a window.
type actSeeingMover struct {
	pausingMover
	acts []string
}

func (m *actSeeingMover) Move(ctx context.Context, enc *encounter.Encounter, step encounter.MoveStep) error {
	m.acts = append(m.acts, enc.CurrentAct())
	return m.pausingMover.Move(ctx, enc, step)
}

// TestDrivenPauseKeepsWalkAct is Law 2 across a window: the Mover that poses
// the window sees the walk's act, the pause keeps it through a save, and the
// cells walked after the answer carry it.
func (s *ActTestSuite) TestDrivenPauseKeepsWalkAct() {
	mover := &actSeeingMover{pausingMover: pausingMover{pauseAt: map[int]bool{1: true}}}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Move{Path: closeIn}}}
	enc := s.drivenScene(driver, passStriker{}, mover)

	s.open(enc, "end_turn", "")
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().NoError(enc.CloseAct())
	s.Require().True(enc.Paused())

	before := ofKind(s.story(enc, alice), encounter.BeatMoved, goblin)
	s.Require().Len(before, 1)
	walk := before[0].correlation
	s.True(strings.HasPrefix(walk, "move:"))
	s.Equal(walk, enc.PausedAct(), "the pause belongs to the walk")
	s.Require().Len(mover.acts, 2)
	s.Equal(walk, mover.acts[1], "the Mover posing the window sees the walk's act")

	data := enc.ToData()
	s.Require().NotNil(data.Pause)
	s.Equal(walk, data.Pause.Act, "the blob carries the walk's act")

	pt := &PauseTestSuite{}
	pt.SetT(s.T())
	loaded := pt.reload(enc, &pausingMover{}, &downList{})
	s.Equal(walk, loaded.PausedAct(), "a reload keeps it")

	s.open(loaded, "react", walk)
	_, err = loaded.Resume(context.Background())
	s.Require().NoError(err)
	s.Require().NoError(loaded.CloseAct())

	after := ofKind(s.story(loaded, alice), encounter.BeatMoved, goblin)
	s.Require().Len(after, 3)
	for _, m := range after {
		s.Equal(walk, m.correlation, "the resumed cells are the same walk")
	}
}

// TestTimePickIsItsOwnAct is R24 on the world clock: a creature's pick,
// drawn by a player's walk raising the clock, is the creature's own act.
func (s *ActTestSuite) TestTimePickIsItsOwnAct() {
	wt := &WorldTimeSuite{}
	wt.SetT(s.T())
	wt.SetupTest()
	enc := wt.hallWith(
		[]encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"vendors", encounter.FactionParty},
			Stance:  encounter.StanceNeutral,
		}},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Faction: "vendors",
			Position: spatial.Position{X: 20, Y: 6}, Table: hunts()},
	)

	walk := s.open(enc, "move", "")
	wt.walk(enc, alice, 1, 6, 1) // the sixth cell pays the world a round
	s.Require().NoError(enc.CloseAct())

	entries := s.story(enc, alice)
	var picks []told
	for _, e := range ofKind(entries, encounter.BeatAnswered, "") {
		if e.beat["creature"] == string(goblin) {
			picks = append(picks, e)
		}
	}
	s.Require().NotEmpty(picks, "the round the walk raised asked the goblin")
	for _, p := range picks {
		s.True(strings.HasPrefix(p.correlation, "time:"), "a pick is a time act, got %q", p.correlation)
		s.NotEqual(walk, p.correlation, "the goblin's answer is not the walker's act")
		s.Equal(fmt.Sprintf("time:%d", p.seq), p.correlation, "the pick is its act's first beat")
	}
	for _, m := range ofKind(entries, encounter.BeatMoved, alice) {
		s.Equal(walk, m.correlation)
	}
}

// TestSpeakAnswerCarriesSpeakAct is Law 2 for speech: the creature's answer
// to a line is the line's own consequence, not a time pick.
func (s *ActTestSuite) TestSpeakAnswerCarriesSpeakAct() {
	at := &AnswerTestSuite{}
	at.SetT(s.T())
	at.SetupTest()
	enc := at.front(nil, threat([]encounter.Answer{{Weight: 1, Say: "Fine.", Fact: campFact}}, nil))

	act := s.open(enc, "speak", "")
	_, err := enc.Speak(context.Background(), &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin,
		Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.Require().NoError(enc.CloseAct())

	entries := s.story(enc, alice)
	spoke := ofKind(entries, encounter.BeatSpoke, "")
	s.Require().Len(spoke, 1)
	s.Equal(act, spoke[0].correlation)

	var answered []told
	for _, e := range ofKind(entries, encounter.BeatAnswered, "") {
		if e.beat["verb"] == "speak" {
			answered = append(answered, e)
		}
	}
	s.Require().Len(answered, 1)
	s.Equal(act, answered[0].correlation, "the answer is the line's act")
}

// equipScene is the session-verbs suite's free-roam yard.
func (s *ActTestSuite) equipScene() *encounter.Encounter {
	sv := &SessionVerbsSuite{}
	sv.SetT(s.T())
	return sv.freeRoam()
}

// swap draws a longsword over a dagger in alice's main hand.
var swap = &encounter.RecordEquipInput{
	Member: alice, Slot: "main_hand", Stowed: "dnd5e:weapons:dagger", Drawn: "dnd5e:weapons:longsword",
}

// TestRecordEquipWithoutAct: with no act open the equip mints its own, as it
// always has.
func (s *ActTestSuite) TestRecordEquipWithoutAct() {
	enc := s.equipScene()

	out, err := enc.RecordEquip(swap)
	s.Require().NoError(err)
	s.Require().Len(out.Seqs, 2)
	s.Equal(fmt.Sprintf("equip:%d", out.Seqs[0]), out.Correlation)

	beats := ofKind(s.story(enc, alice), encounter.BeatEquipmentChanged, "")
	s.Require().Len(beats, 2)
	for _, b := range beats {
		s.Equal(out.Correlation, b.correlation)
	}
}

// TestRecordEquipUnderAct: under an open act the equip is that act. The act
// is a reopened id rather than a minted `equip:` one, because a minted one
// would equal the id RecordEquip mints for itself and prove nothing.
func (s *ActTestSuite) TestRecordEquipUnderAct() {
	enc := s.equipScene()

	act := s.open(enc, "react", "trade:3")
	out, err := enc.RecordEquip(swap)
	s.Require().NoError(err)
	s.Require().NoError(enc.CloseAct())
	s.Equal(act, out.Correlation)

	beats := ofKind(s.story(enc, alice), encounter.BeatEquipmentChanged, "")
	s.Require().Len(beats, 2)
	for _, b := range beats {
		s.Equal(act, b.correlation)
	}
}
