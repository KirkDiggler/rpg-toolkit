// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// sneak_test.go is SNEAKING's focused cases (rpg-project#606, rpg-toolkit#1020;
// design R6–R12): the mode verb and its refusals, the plain-view lost verdict,
// one roll per entry, suppression through the real sight pass, the ghost a won
// roll leaves, and every verb that shows the sneaker.
//
// THE SCENE. Two rooms, room-a west and room-b east, joined by one open
// doorway at row 5 (twoRoomWall, twoRoomDoor). The rogue starts deep in
// room-a, out of everybody's sight. The knight stands in room-b with a
// four-cell sight, so its zone reaches the doorway's east cell and no
// further west: the doorway's far side is the crossing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	sneakRogue  = encounter.MemberID("rogue")
	sneakKnight = encounter.MemberID("knight")
	sneakCook   = encounter.MemberID("cook")
	sneakSentry = encounter.MemberID("sentry")
)

// The knight sees four cells; the rogue sees everything.
const sneakKnightSight = 4

// offsetOf is an authored [col, row]: the frame a MemberInput's position is
// given in. Every cell a verb answers is the axial [cellAt].
func offsetOf(col, row int) spatial.Position {
	return spatial.Position{X: float64(col), Y: float64(row)}
}

var (
	// sneakOutside is the doorway's west cell: five cells from the knight,
	// one outside its zone.
	sneakOutside = cellAt(9, 5)
	// sneakInside is the doorway's east cell: four cells from the knight,
	// the first cell of its zone a walk from the west enters.
	sneakInside = cellAt(10, 5)
	// sneakKnightAt is where the knight stands.
	sneakKnightAt = cellAt(14, 5)
)

// scriptedStealth is a CheckResolver whose automatic rolls answer from a
// script, one verdict per call, and that records what it was asked. An
// unscripted call is an error, so a roll nobody expected fails the scene.
type scriptedStealth struct {
	script []bool
	asked  []encounter.ResolveCheckInput
	err    error
}

// ResolveCheck is never asked by these scenes.
func (r *scriptedStealth) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, errors.New("sneak scene: no explicit check is made")
}

// ResolveDiscoveryCheck answers the next scripted verdict.
func (r *scriptedStealth) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	r.asked = append(r.asked, *in)
	if r.err != nil {
		return nil, r.err
	}
	if len(r.script) == 0 {
		return nil, fmt.Errorf("sneak scene: roll %d was not scripted", len(r.asked))
	}
	beaten := r.script[0]
	r.script = r.script[1:]
	total := map[bool]int{true: 22, false: 9}[beaten]

	return &encounter.ResolveCheckOutput{Beaten: beaten, Applied: in.Approaches[0], Total: total}, nil
}

// sneakSight is a per-member sight list that counts how often it is asked:
// a refresh asks it, so a call that asks nothing refreshed nothing.
type sneakSight struct {
	reach map[encounter.MemberID]int
	calls int
}

func (c *sneakSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	c.calls++
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = unlimitedSight
		if cells, ok := c.reach[id]; ok {
			out[id] = cells
		}
	}

	return out, nil
}

// sneakOptions shapes one scene.
type sneakOptions struct {
	// extra members beyond the rogue and the knight.
	extra []encounter.MemberInput
	// knightTable is the knight's table; nil stands it still.
	knightTable encounter.Table
	// knightSpeed is the knight's speed in feet.
	knightSpeed int
	// reset is the knight's re-arm distance in cells.
	reset int
	// rogueSight bounds the rogue's sight; zero is unlimited.
	rogueSight int
	// noResolver installs no CheckResolver at all.
	noResolver bool
	// noKnight leaves the knight out.
	noKnight bool
}

type SneakSuite struct {
	suite.Suite
	enc     *encounter.Encounter
	stealth *scriptedStealth
	sight   *sneakSight
	caps    encounter.Capabilities
	setupIn *encounter.SetupInput
}

func TestSneakSuite(t *testing.T) { suite.Run(t, new(SneakSuite)) }

// open builds the scene.
func (s *SneakSuite) open(opts sneakOptions, script ...bool) {
	s.stealth = &scriptedStealth{script: script}
	s.sight = &sneakSight{reach: map[encounter.MemberID]int{sneakKnight: sneakKnightSight, sneakSentry: sneakKnightSight}}
	if opts.rogueSight > 0 {
		s.sight.reach[sneakRogue] = opts.rogueSight
	}
	members := []encounter.MemberInput{{ID: sneakRogue, Kind: encounter.KindPlayer, Position: offsetOf(4, 5)}}
	if !opts.noKnight {
		members = append(members, encounter.MemberInput{
			ID: sneakKnight, Kind: encounter.KindMonster, Position: offsetOf(14, 5), Table: opts.knightTable,
		})
	}
	members = append(members, opts.extra...)
	sheets := sheetFacts{
		sneakRogue:  {SpeedFeet: 30, PassivePerception: 12},
		sneakKnight: {SpeedFeet: opts.knightSpeed, PassivePerception: 10, WatchResetCells: opts.reset},
		sneakCook:   {PassivePerception: 9},
		sneakSentry: {PassivePerception: 14},
	}
	s.caps = encounter.Capabilities{
		Sight:         s.sight,
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        sheets,
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        tableDriver(),
		Actors: encounter.Actors{
			Striker:   passStriker{},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
	if !opts.noResolver {
		s.caps.CheckResolver = s.stealth
	}
	s.setupIn = &encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("room-a", 0, 0, 10, 10), rectRegion("room-b", 10, 0, 10, 10)},
			Walls:   twoRoomWall(),
			Doors:   []encounter.DoorInput{twoRoomDoor},
		},
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention:    encounter.RetentionUnbounded,
		Capabilities: s.caps,
	}
	enc, err := encounter.NewEncounter(s.setupIn)
	s.Require().NoError(err)
	s.enc = enc
}

// sneak declares the rogue's mode.
func (s *SneakSuite) sneak(on bool) *encounter.SetSneakingOutput {
	out, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: sneakRogue, Sneaking: on})
	s.Require().NoError(err)
	s.Equal(on, out.Sneaking)

	return out
}

// walkTo steps the rogue along the pathfinder's route to a cell.
func (s *SneakSuite) walkTo(to spatial.Position) {
	route, err := s.enc.Route(encounter.RouteInput{Mover: sneakRogue, Policy: encounter.MoveToward, Anchor: to, Budget: 40})
	s.Require().NoError(err)
	path := route.Path
	if len(path) == 0 || path[len(path)-1] != to {
		path = append(path, to)
	}
	for _, cell := range path {
		if s.at(sneakRogue) == cell {
			continue
		}
		_, err := s.enc.Step(&encounter.StepInput{Member: sneakRogue, To: cell})
		s.Require().NoError(err, "the rogue steps to %v", cell)
	}
}

// at is where a member stands.
func (s *SneakSuite) at(id encounter.MemberID) spatial.Position {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.FailNow("not in the run", "%s", id)

	return spatial.Position{}
}

// holding is observer's sight holding of subject, or nil.
func (s *SneakSuite) holding(observer, subject encounter.MemberID) *perception.Holding {
	held, err := s.enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for i := range held {
		if held[i].Channel == perception.Sight && encounter.MemberID(held[i].Subject) == subject {
			return &held[i]
		}
	}

	return nil
}

// seesLive reports whether observer holds subject live now.
func (s *SneakSuite) seesLive(observer, subject encounter.MemberID) bool {
	h := s.holding(observer, subject)
	return h != nil && len(h.CurrentVia) > 0
}

// snuck is every snuck beat a member's story holds, decoded.
func (s *SneakSuite) snuck(audience encounter.MemberID) []map[string]any {
	story, err := s.enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	var out []map[string]any
	for _, entry := range story {
		var body map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		if body["beat"] == encounter.BeatSnuck {
			out = append(out, body)
		}
	}

	return out
}

// storyLen is how many beats the whole story holds.
func (s *SneakSuite) storyLen() int {
	story, err := s.enc.Story(&encounter.StoryInput{Audience: sneakRogue})
	s.Require().NoError(err)

	return len(story)
}

// verdict is the rogue's stored verdict against a watcher.
func (s *SneakSuite) verdict(watcher encounter.MemberID) (encounter.SneakVerdictData, bool) {
	v, ok := s.enc.ToData().SneakVerdicts[sneakRogue][watcher]
	return v, ok
}

// inFight reports whether the rogue is on a turn clock.
func (s *SneakSuite) inFight() bool {
	on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: sneakRogue})
	s.Require().NoError(err)

	return on.Kind == encounter.ClockTurn
}

// cook is the civil cook in room-a, beside the rogue's start.
func cook() encounter.MemberInput {
	return encounter.MemberInput{ID: sneakCook, Kind: encounter.KindWorld, Position: offsetOf(3, 5),
		Options: []encounter.Option{{ID: "hello", Say: "Hello.", Check: []encounter.CheckApproach{{Ability: "persuasion", DC: 10}}}}}
}

// TestRefusals: the mode is a player's, declared in free roam, on an open run
// whose resolver can roll it.
func (s *SneakSuite) TestRefusals() {
	s.open(sneakOptions{})
	_, err := s.enc.SetSneaking(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
	_, err = s.enc.SetSneaking(&encounter.SetSneakingInput{Member: sneakKnight, Sneaking: true})
	s.ErrorIs(err, encounter.ErrNotMember, "a monster never sneaks")
	_, err = s.enc.SetSneaking(&encounter.SetSneakingInput{Member: "nobody", Sneaking: true})
	s.ErrorIs(err, encounter.ErrNotMember)

	s.Run("in a fight", func() {
		s.open(sneakOptions{})
		s.walkTo(sneakInside)
		s.Require().True(s.inFight(), "the rogue walked into the knight's sight")
		_, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: sneakRogue, Sneaking: true})
		s.ErrorIs(err, encounter.ErrInBubble)
	})
	s.Run("closed", func() {
		s.open(sneakOptions{})
		_, err := s.enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = s.enc.SetSneaking(&encounter.SetSneakingInput{Member: sneakRogue, Sneaking: true})
		s.ErrorIs(err, encounter.ErrClosed)
	})
	s.Run("no resolver", func() {
		s.open(sneakOptions{noResolver: true})
		_, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: sneakRogue, Sneaking: true})
		s.ErrorIs(err, encounter.ErrNoCheckResolver)
		s.False(s.enc.Sneaking(sneakRogue))
	})
}

// TestDeclaringInPlainViewIsLost: the cook sees the rogue when the rogue
// declares, so the pair is lost with no roll and no beat, and the cook keeps
// holding the rogue (I1: nobody vanishes in plain view).
func (s *SneakSuite) TestDeclaringInPlainViewIsLost() {
	s.open(sneakOptions{extra: []encounter.MemberInput{cook()}})
	s.Require().True(s.seesLive(sneakCook, sneakRogue))

	s.sneak(true)

	s.Empty(s.stealth.asked, "nobody rolls to vanish from a creature that sees them")
	s.Empty(s.snuck(sneakRogue))
	v, ok := s.verdict(sneakCook)
	s.Require().True(ok, "the cook's pair holds a verdict")
	s.Equal(encounter.SneakVerdictData{Beaten: false, Armed: false, InZone: true}, v)
	s.True(s.seesLive(sneakCook, sneakRogue), "the cook still sees the rogue")

	s.walkTo(cellAt(5, 5))
	s.Empty(s.stealth.asked, "a lost pair does not roll while it stands")
	s.True(s.seesLive(sneakCook, sneakRogue))
	_, knight := s.verdict(sneakKnight)
	s.False(knight, "a watcher that never saw the rogue holds no verdict")
}

// TestDeclaringTheModeHeldIsANoOp: on while on, and off while off, rolls
// nothing, re-arms nothing, refreshes nothing and writes no beat.
func (s *SneakSuite) TestDeclaringTheModeHeldIsANoOp() {
	s.open(sneakOptions{extra: []encounter.MemberInput{cook()}})
	calls, beats := s.sight.calls, s.storyLen()
	out := s.sneak(false)
	s.Nil(out.IntelDeltas)
	s.Equal(calls, s.sight.calls, "off while off runs no refresh")
	s.Equal(beats, s.storyLen())

	s.sneak(true)
	before := s.enc.ToData().SneakVerdicts
	calls, beats = s.sight.calls, s.storyLen()
	out = s.sneak(true)
	s.Nil(out.IntelDeltas)
	s.Nil(out.Formed)
	s.Equal(calls, s.sight.calls, "on while on runs no refresh")
	s.Equal(beats, s.storyLen(), "and writes no beat")
	s.Equal(before, s.enc.ToData().SneakVerdicts, "and changes no verdict")
	s.Empty(s.stealth.asked)
}

// TestAWonRollKeepsTheRogueUnseen: one roll on the crossing, through the real
// sight pass: the knight's percept never gains the rogue, no fight forms, and
// the beat goes to the rogue alone and names the knight the rogue sees.
func (s *SneakSuite) TestAWonRollKeepsTheRogueUnseen() {
	s.open(sneakOptions{}, true)
	s.sneak(true)
	s.walkTo(sneakOutside)
	s.Empty(s.stealth.asked, "outside the zone nothing rolls")

	s.walkTo(sneakInside)
	s.Require().Len(s.stealth.asked, 1, "the crossing rolls once")
	s.Equal(sneakRogue, s.stealth.asked[0].Member)
	s.Equal([]encounter.CheckApproach{{Ability: "stealth", DC: 10}}, s.stealth.asked[0].Approaches,
		"the rogue's Stealth against the knight's passive Perception")
	s.Nil(s.holding(sneakKnight, sneakRogue), "the knight's percept never gained the rogue")
	s.False(s.inFight())

	beats := s.snuck(sneakRogue)
	s.Require().Len(beats, 1)
	s.Equal(string(sneakRogue), beats[0]["member"])
	s.Equal(string(sneakKnight), beats[0]["watcher"], "the rogue sees the knight, so the beat names it")
	s.Equal(float64(10), beats[0]["dc"])
	s.Equal(float64(22), beats[0]["total"])
	s.Equal(true, beats[0]["beaten"])
	s.Empty(s.snuck(sneakKnight), "the beat is the rogue's alone")

	// Deeper in: still one roll.
	s.walkTo(cellAt(11, 5))
	s.Len(s.stealth.asked, 1, "a walk through one zone rolls once")
	s.Nil(s.holding(sneakKnight, sneakRogue))
	s.False(s.inFight())
}

// TestALostRollFormsTheFightAtFullSpeed: a loss lets the knight see the rogue,
// today's contact law forms the fight, and formation ends the mode and its
// verdicts before the first turn reads the rogue's speed.
func (s *SneakSuite) TestALostRollFormsTheFightAtFullSpeed() {
	s.open(sneakOptions{}, false)
	s.sneak(true)
	s.walkTo(sneakInside)
	s.Require().Len(s.stealth.asked, 1)
	s.True(s.seesLive(sneakKnight, sneakRogue))
	s.True(s.inFight())
	s.False(s.enc.Sneaking(sneakRogue), "formation ended the mode")
	s.Nil(s.enc.ToData().SneakVerdicts, "and every verdict with it")
}

// TestDeclaringOffInsideAZoneIsSeenAtOnce: after a won roll, declaring the
// mode off in the knight's zone makes the knight hold the rogue in the same
// call, and that sighting forms the fight.
func (s *SneakSuite) TestDeclaringOffInsideAZoneIsSeenAtOnce() {
	s.open(sneakOptions{}, true)
	s.sneak(true)
	s.walkTo(sneakInside)
	s.Require().Nil(s.holding(sneakKnight, sneakRogue))

	out := s.sneak(false)
	s.True(s.seesLive(sneakKnight, sneakRogue), "the refresh in the same call")
	s.NotNil(out.Formed, "the sighting the declaration caused forms the fight")
	s.Nil(s.enc.ToData().SneakVerdicts)
}

// TestANonSneakerIsSeenAsBefore: the same doorway without the mode is seen
// exactly as today, and nothing rolls (R2).
func (s *SneakSuite) TestANonSneakerIsSeenAsBefore() {
	s.open(sneakOptions{})
	s.walkTo(sneakInside)
	s.Empty(s.stealth.asked)
	s.True(s.seesLive(sneakKnight, sneakRogue))
	s.True(s.inFight())
}

// TestAWonRollKeepsTheGhost: a watcher that saw the rogue before and holds a
// memory of it keeps that ghost through a won roll — never removed, never
// refreshed (R11).
func (s *SneakSuite) TestAWonRollKeepsTheGhost() {
	sentry := encounter.MemberInput{ID: sneakSentry, Kind: encounter.KindWorld, Position: offsetOf(14, 5)}
	s.open(sneakOptions{noKnight: true, extra: []encounter.MemberInput{sentry}}, true)
	s.walkTo(sneakInside)
	s.Require().True(s.seesLive(sneakSentry, sneakRogue), "the civil sentry sees the rogue in the doorway")
	s.walkTo(cellAt(7, 5))
	ghost := s.holding(sneakSentry, sneakRogue)
	s.Require().NotNil(ghost)
	s.Require().Empty(ghost.CurrentVia, "out of its sight, the sentry holds a ghost")

	s.sneak(true)
	s.walkTo(cellAt(11, 5))
	s.Require().Len(s.stealth.asked, 1)
	after := s.holding(sneakSentry, sneakRogue)
	s.Require().NotNil(after, "the ghost is kept, not erased")
	s.Empty(after.CurrentVia, "and stays a ghost")
	s.Equal(ghost.Payload, after.Payload, "at the last position it saw")
	s.Equal(ghost.Observed, after.Observed)
}

// TestAPacingZoneEdgeRollsOnce: the knight paces one cell back and forth, so
// its zone's edge sweeps over the still rogue every other round. The first
// sweep rolls; the rest do not, until the rogue stands the re-arm distance
// away and comes back.
func (s *SneakSuite) TestAPacingZoneEdgeRollsOnce() {
	pace := encounter.Table{encounter.AnswerTime: {
		// A three-cell loop: the knight's cell, one west (four cells from
		// the doorway's west cell, inside the zone's reach of it), and one
		// north-east of that, back out of reach.
		{Weight: 1, Toward: &encounter.Selector{Path: []spatial.Position{
			sneakKnightAt, {X: sneakKnightAt.X - 1, Y: sneakKnightAt.Y}, {X: sneakKnightAt.X, Y: sneakKnightAt.Y - 1},
		}}},
	}}
	s.open(sneakOptions{knightTable: pace, knightSpeed: 5, reset: 2}, true, true)
	s.sneak(true)
	s.walkTo(sneakOutside)
	for range 6 {
		s.Require().NoError(encounter.ExportedSpendWorldAction(s.enc, sneakRogue))
	}
	s.Len(s.stealth.asked, 1, "the edge crossed the rogue more than once and rolled once")
	s.False(s.inFight())

	// Two cells clear of every zone cell, then back in.
	s.walkTo(cellAt(6, 5))
	for range 2 {
		s.Require().NoError(encounter.ExportedSpendWorldAction(s.enc, sneakRogue))
	}
	s.walkTo(sneakOutside)
	for range 2 {
		s.Require().NoError(encounter.ExportedSpendWorldAction(s.enc, sneakRogue))
	}
	s.Len(s.stealth.asked, 2, "past the re-arm distance, the next entry rolls again")
}

// TestAnUnseenWatcherIsNotNamed: the rogue sees one cell, so the knight that
// rolls against it is a creature the rogue cannot see, and the beat does not
// name it (R12).
func (s *SneakSuite) TestAnUnseenWatcherIsNotNamed() {
	s.open(sneakOptions{rogueSight: 1}, true)
	s.sneak(true)
	s.walkTo(sneakInside)
	beats := s.snuck(sneakRogue)
	s.Require().Len(beats, 1)
	s.NotContains(beats[0], "watcher")
	s.Equal(true, beats[0]["beaten"])
}

// TestASaveBetweenStepsDoesNotReRoll: the verdict and InZone persist, so a
// reload inside the zone is not a crossing.
func (s *SneakSuite) TestASaveBetweenStepsDoesNotReRoll() {
	s.open(sneakOptions{}, true)
	s.sneak(true)
	s.walkTo(sneakInside)
	s.Require().Len(s.stealth.asked, 1)

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: s.enc.ToData(), Capabilities: s.caps})
	s.Require().NoError(err)
	s.enc = loaded
	s.True(s.enc.Sneaking(sneakRogue), "the mode persists")
	s.walkTo(cellAt(11, 5))
	s.Len(s.stealth.asked, 1, "no second roll after the load")
	s.Nil(s.holding(sneakKnight, sneakRogue))
	s.False(s.inFight())
}

// TestASingleObserverPassJudgesOnlyItsObservers: a pass whose only observer is
// the rogue — the concealment reveal's shape — judges no pair, even with the
// rogue standing in the knight's zone; the next full pass does.
func (s *SneakSuite) TestASingleObserverPassJudgesOnlyItsObservers() {
	s.open(sneakOptions{}, true)
	s.sneak(true)
	s.walkTo(sneakOutside)
	s.Require().Empty(s.stealth.asked)

	// The knight's sight widens by one: the rogue now stands in its zone.
	s.sight.reach[sneakKnight] = sneakKnightSight + 1
	s.Require().NoError(encounter.ExportedRebuildPerceptsFor(s.enc, sneakRogue))
	s.Empty(s.stealth.asked, "the knight observed nothing in that pass, so its pair is untouched")
	_, ok := s.verdict(sneakKnight)
	s.False(ok)

	s.Require().NoError(encounter.ExportedRebuildPerceptsFor(s.enc, sneakRogue, sneakKnight))
	s.Len(s.stealth.asked, 1, "the knight's own pass crosses its zone over the rogue: a watcher's turn is an entry too")
	s.Nil(s.holding(sneakKnight, sneakRogue))
}

// TestShowingYourselfEndsTheMode: an attack, a cast and a spoken line each
// end the mode and clear its verdicts (R7).
func (s *SneakSuite) TestShowingYourselfEndsTheMode() {
	s.Run("attack", func() {
		s.open(sneakOptions{extra: []encounter.MemberInput{cook()}})
		s.sneak(true)
		s.Require().NotNil(s.enc.ToData().SneakVerdicts)
		_, err := s.enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &encounter.RecordInput{
			Kind: encounter.OutcomeMissed, Actor: sneakRogue, Targets: []encounter.MemberID{sneakKnight},
			Attack: &encounter.AttackIdentity{Ref: "dnd5e:weapons:dagger", Name: "Dagger", DamageType: "piercing"},
		}}}})
		s.Require().NoError(err)
		s.False(s.enc.Sneaking(sneakRogue))
		s.Nil(s.enc.ToData().SneakVerdicts)
	})
	s.Run("cast", func() {
		s.open(sneakOptions{extra: []encounter.MemberInput{cook()}})
		s.sneak(true)
		_, err := s.enc.RecordCast(&encounter.RecordCastInput{
			Actor: sneakRogue, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:minor-illusion", Name: "Minor Illusion"},
		})
		s.Require().NoError(err)
		s.False(s.enc.Sneaking(sneakRogue))
		s.Nil(s.enc.ToData().SneakVerdicts)
	})
	s.Run("speak", func() {
		s.open(sneakOptions{extra: []encounter.MemberInput{cook()}})
		s.sneak(true)
		_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
			Actor: sneakRogue, Target: sneakCook, OptionID: "hello", Skill: "persuasion",
			Beaten: true, DC: 10, Total: 15, Roller: rollsLowest{},
		})
		s.Require().NoError(err)
		s.False(s.enc.Sneaking(sneakRogue))
		s.Nil(s.enc.ToData().SneakVerdicts)
	})
	s.Run("exit", func() {
		s.open(sneakOptions{extra: []encounter.MemberInput{cook()}})
		s.sneak(true)
		_, err := s.enc.Exit(&encounter.ExitInput{Member: sneakCook})
		s.Require().NoError(err)
		s.True(s.enc.Sneaking(sneakRogue), "a watcher leaving ends nobody's mode")
		s.Nil(s.enc.ToData().SneakVerdicts, "but no verdict names a member who left")
	})
}

// TestAResolverErrorAbortsTheVerb: a roll that cannot be judged fails the
// step that crossed, by name.
func (s *SneakSuite) TestAResolverErrorAbortsTheVerb() {
	s.open(sneakOptions{})
	s.stealth.err = errors.New("no sheet for the rogue")
	s.sneak(true)
	s.walkTo(sneakOutside)
	_, err := s.enc.Step(&encounter.StepInput{Member: sneakRogue, To: sneakInside})
	s.ErrorContains(err, "no sheet for the rogue")
}

// TestALoadRefusesAVerdictThisBuildWouldNotWrite: a verdict for a mover who
// is not sneaking is refused at load.
func (s *SneakSuite) TestALoadRefusesAVerdictThisBuildWouldNotWrite() {
	s.open(sneakOptions{})
	data := s.enc.ToData()
	data.SneakVerdicts = map[encounter.MemberID]map[encounter.MemberID]encounter.SneakVerdictData{
		sneakRogue: {sneakKnight: {Beaten: true, InZone: true}},
	}
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.caps})
	s.ErrorIs(err, encounter.ErrInvalidData)
}

// TestANegativeResetIsRefused: the re-arm distance is supplied, and a
// negative one is a defect the first time the sheets are asked.
func (s *SneakSuite) TestANegativeResetIsRefused() {
	s.open(sneakOptions{reset: -1})
	_, err := s.enc.Step(&encounter.StepInput{Member: sneakRogue, To: cellAt(5, 5)})
	s.ErrorIs(err, encounter.ErrInvalidData)
}
