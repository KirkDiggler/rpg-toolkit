// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// memberdown_test.go is the run's end (rpg-project#268): TriggerMemberDown,
// the declared ending that fires when a named member's standing reaches down.
// The boss flag dungeonspec has carried unread since rpg-project#256 finally
// has a trigger to become — declared by the host, over the member it spawns.
//
// The beat order below is Kirk's ruling (rpg-project#269 §6.6), not an
// accident of implementation: down → bubble-dissolved → ended. The fight
// dissolves FIRST (rpg-toolkit#959 fork (c), untouched by this trigger) and
// the encounter closes on the world clock — the run ends with everyone
// standing in the world, not inside a bubble.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type MemberDownSuite struct {
	deathScene
}

func TestMemberDownSuite(t *testing.T) {
	suite.Run(t, new(MemberDownSuite))
}

// doomKey is the ending the wolf's death fires. The KEY is content
// vocabulary and nothing here interprets it (rpg-project#269 §6.3) — the
// composition only knows "this member's death ends things".
const doomKey = "boss-down"

// doomed is the trio — alice against the goblin and the wolf, all in plain
// sight — with the wolf's death declared as an ending. Not the shared scene()
// fixture, because that one hard-codes its endings and endings are what this
// suite is about.
func (s *MemberDownSuite) doomed(standing encounter.Standing) *encounter.Encounter {
	s.T().Helper()

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Sight: everyoneSeesTheWholeMap{}, Standing: standing,
		Retention: encounter.RetentionUnbounded,
		Field:     encounter.FieldInput{Canvas: openAir(), Regions: []encounter.RegionInput{rectRegion(cryptID, 0, 0, 12, 12)}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
			{ID: wolf, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 10}},
		},
		Endings: []encounter.EndingInput{
			{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
			{Key: doomKey, Trigger: encounter.TriggerMemberDown{Member: wolf}},
		},
	})
	s.Require().NoError(err)

	return enc
}

// TestTheNamedMemberDownEndsTheRun is the whole feature: the fight the wolf
// dies in dissolves by defeat, and THEN the run is over — one consult, ruled
// order, no caller.
func (s *MemberDownSuite) TestTheNamedMemberDownEndsTheRun() {
	down := &downList{}
	enc := s.doomed(down)
	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, alice), "control: a fight is running")

	down.down = []encounter.MemberID{goblin, wolf}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	status, err := enc.Status()
	s.Require().NoError(err)
	s.False(status.Open, "the wolf is down, so the run is over and nobody had to say so")
	s.Require().NotNil(status.Outcome)
	s.Equal(doomKey, status.Outcome.Ending, "and the outcome names the declared ending that fired")

	s.Equal([]string{"scene-opened", "bubble-formed", "tick", "down", "down", "bubble-dissolved", "ended"},
		s.beatKindsOf(enc, alice),
		"the ruled order (rpg-project#269 §6.6): the bodies are news, the fight ends, and only then the run — the close lands on the world clock")

	s.Empty(enc.ToData().Bubbles, "no bubble outlives the close")
}

// TestAnotherBodyIsNotTheDoom is the control that gives the pin above its
// meaning: a death the ending does not name changes the fight, not the run.
func (s *MemberDownSuite) TestAnotherBodyIsNotTheDoom() {
	down := &downList{}
	enc := s.doomed(down)

	down.down = []encounter.MemberID{goblin}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	status, err := enc.Status()
	s.Require().NoError(err)
	s.True(status.Open, "the goblin was not the named member, so the run goes on")
	s.NotContains(s.beatKindsOf(enc, alice), "ended", "and no close was narrated")
}

// TestSetupRefusesADoomNamingNobody: empty is refused, not defaulted — "any
// player down" and "every player down" are different endings (first blood
// versus a party wipe), and choosing between them belongs to the wave that
// brings death saves (TriggerMemberDown's doc).
func (s *MemberDownSuite) TestSetupRefusesADoomNamingNobody() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Sight: everyoneSeesTheWholeMap{}, Standing: &downList{},
		Retention: encounter.RetentionUnbounded,
		Field:     encounter.FieldInput{Canvas: openAir(), Regions: []encounter.RegionInput{rectRegion(cryptID, 0, 0, 12, 12)}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
		},
		Endings: []encounter.EndingInput{
			{Key: doomKey, Trigger: encounter.TriggerMemberDown{}},
		},
	})
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
}

// TestTheDoomSurvivesTheRoundTrip: the ending is declared state, and declared
// state round-trips — a reloaded encounter still ends when the named member
// falls, fed by the SAME consult (the fire is guarded by the outcome, not by
// any story ledger, so a member already down when the blob loads still fires
// at the next consult).
func (s *MemberDownSuite) TestTheDoomSurvivesTheRoundTrip() {
	saved := s.doomed(&downList{}).ToData()

	s.Require().Len(saved.Endings, 2)
	s.Equal("member_down", saved.Endings[1].Kind)
	s.Equal(wolf, saved.Endings[1].Member, "persisted by name, exactly as declared")

	down := &downList{down: []encounter.MemberID{goblin, wolf}}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       saved,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Sight: everyoneSeesTheWholeMap{}, Standing: down,
	})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	status, err := enc.Status()
	s.Require().NoError(err)
	s.False(status.Open, "the reloaded doom still fires")
	s.Require().NotNil(status.Outcome)
	s.Equal(doomKey, status.Outcome.Ending)
}

// TestLoadRefusesADoomNamingNobody: the trust boundary makes the same refusal
// Setup does — a member_down ending with no member is rejected by name, never
// loaded as an ending that can never fire.
func (s *MemberDownSuite) TestLoadRefusesADoomNamingNobody() {
	saved := s.doomed(&downList{}).ToData()
	saved.Endings[1].Member = ""

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       saved,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Sight: everyoneSeesTheWholeMap{}, Standing: &downList{},
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
}
