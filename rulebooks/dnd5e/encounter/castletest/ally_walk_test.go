// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// ally_walk_test.go is THE ALLY WALK (rpg-project#717, Frank's list S3): the
// walk site cut from the castle's gatehouse, played through the castle's own
// host. A neutral veteran the hero persuades to `join` becomes the party's
// side through his faction's disposition (`then: allied`), follows the hero
// with `toward: ally`, and fights the thugs beside the party, never the
// party. Not joined, he holds.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const allyWalkFixture = "testdata/ally-walk.yaml"

// The ally walk's cast and sides, by the ids the file gives them.
const (
	allyVeteran = encounter.MemberID("veteran")
	allyThug1   = encounter.MemberID("thug-1")
	allyThug2   = encounter.MemberID("thug-2")
	veterans    = encounter.FactionID("veterans")
	thugs       = encounter.FactionID("thugs")
)

// Where the hero stands and walks to on the site's floor.
var (
	// besideTheVeteran is the open ground beside the veteran, outside the
	// gatehouse and out of the thugs' sight, as an offset cell for
	// [encounter.MemberInput.Position].
	besideTheVeteran = spatial.Position{X: 2, Y: 16}

	// westOfTheGatehouse is the open ground off the gatehouse's south-west
	// corner, nine cells from the veteran.
	westOfTheGatehouse = castleCell(-7, 16)

	// insideTheSouthDoor is the gatehouse floor just in from its open south
	// door, where both thugs in its south corners see whoever stands.
	insideTheSouthDoor = castleCell(0, 11)
)

// openTheAllyWalk opens the site with the hero beside the veteran.
func openTheAllyWalk(t *testing.T) *kingScene {
	t.Helper()

	return openTheCastleAs(t, castleHost{at: besideTheVeteran, fixture: allyWalkFixture})
}

// pickedTowardAlly counts the times a creature's own story shows it picking
// `toward: ally`.
func (s *kingScene) pickedTowardAlly(id encounter.MemberID) int {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: id})
	require.NoError(s.t, err)
	n := 0
	for _, beat := range story {
		var body struct{ Beat, Creature, Word, Selector string }
		require.NoError(s.t, json.Unmarshal(beat.Payload, &body))
		if body.Beat == encounter.BeatAnswered && body.Creature == string(id) &&
			body.Word == "toward" && body.Selector == "ally" {
			n++
		}
	}

	return n
}

// markTime spends world rounds without the hero going anywhere: a cell out
// and a cell back, so creatures on the world clock have time to act.
func (s *kingScene) markTime(rounds int) {
	s.t.Helper()
	for i := 0; i < rounds*3 && clockKind(s.t, s.enc, hero) == encounter.ClockWorld; i++ {
		from := s.at(hero)
		out, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveAway, Anchor: s.at(allyVeteran), Budget: 1})
		require.NoError(s.t, err)
		require.NotEmpty(s.t, out.Path, "the hero has room to mark time (%s)", out.StoppedBy)
		for _, cell := range []spatial.Position{out.Path[0], from} {
			round := s.enc.ToData().Clock.HighWater
			_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: true})
			require.NoError(s.t, err)
			s.reloadOnRound(round)
		}
	}
}

// endTurnsWhileFighting ends the active member's turn, whoever it is, up to n
// times while the fight lasts: [kingScene.captainsFirstStrike]'s loop. A
// member who joins a fight during another's driven turn holds the active slot
// undriven until somebody ends it (found building rpg-project#717).
func (s *kingScene) endTurnsWhileFighting(n int) {
	s.t.Helper()
	for range n {
		on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: hero})
		require.NoError(s.t, err)
		if on.Kind != encounter.ClockTurn {
			return
		}
		s.reload()
		_, err = s.enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
		require.NoError(s.t, err, "%s ends its turn", on.Active)
	}
}

// TestAllyWalkTheVeteranJoinsFollowsAndFights is the site's whole route:
// join, walk away, be followed, walk into the thugs, fight beside him.
func TestAllyWalkTheVeteranJoinsFollowsAndFights(t *testing.T) {
	t.Parallel()
	s := openTheAllyWalk(t)
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "precondition: nobody is fighting")
	require.Equal(t, encounter.StanceNeutral, s.stance(encounter.FactionParty, veterans))

	s.said(allyVeteran, "join", "persuasion", true)

	t.Run("joining turns the veteran's side allied to the party, and hostile to the thugs", func(t *testing.T) {
		require.True(t, s.learners("veteran-joined")[allyVeteran], "the veteran, his faction's mind, said it")
		require.Equal(t, encounter.StanceAllied, s.stance(encounter.FactionParty, veterans))
		require.Equal(t, encounter.StanceHostile, s.stance(veterans, thugs))
		allied, known := s.enc.IsAllied(allyVeteran, hero)
		require.True(t, known)
		require.True(t, allied)
	})

	posted := s.at(allyVeteran)
	s.walkTo(westOfTheGatehouse)
	s.markTime(2)

	t.Run("the hero walks away and the veteran follows, on his toward ally row", func(t *testing.T) {
		require.Positive(t, s.pickedTowardAlly(allyVeteran), "the record shows `toward ally` firing")
		now := s.at(allyVeteran)
		require.NotEqual(t, posted, now, "he left his post")
		require.LessOrEqual(t, s.enc.Distance(now, s.at(hero)), 2.0, "and stands beside the hero")
	})

	s.walkTo(insideTheSouthDoor)

	t.Run("walking into the thugs, a fight forms with the party and the veteran on one side", func(t *testing.T) {
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "a thug saw the hero")
		require.False(t, s.hostile(hero, allyVeteran), "the party and the veteran are not enemies")
		require.True(t, s.hostile(allyVeteran, allyThug1))
		require.True(t, s.hostile(allyVeteran, allyThug2))
	})

	s.endTurnsWhileFighting(8)

	t.Run("he strikes a thug and never the hero", func(t *testing.T) {
		var swung bool
		for _, st := range *s.strikes {
			if st.attacker != allyVeteran {
				continue
			}
			require.NotEqual(t, hero, st.target, "the veteran never swings at the party")
			require.Contains(t, []encounter.MemberID{allyThug1, allyThug2}, st.target)
			swung = true
		}
		require.True(t, swung, "the veteran fought the thugs")
		on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: hero})
		require.NoError(t, err)
		require.Contains(t, on.Order, allyVeteran, "he is in the party's fight")
	})
}

// TestAllyWalkNotJoinedTheVeteranHolds is the control: the hero walks away
// without saying `join`, and the veteran, neutral and no one's ally, holds
// where he stands and never picks the follow row.
func TestAllyWalkNotJoinedTheVeteranHolds(t *testing.T) {
	t.Parallel()
	s := openTheAllyWalk(t)
	posted := s.at(allyVeteran)

	s.walkTo(westOfTheGatehouse)
	s.markTime(2)

	require.Equal(t, encounter.StanceNeutral, s.stance(encounter.FactionParty, veterans))
	require.Zero(t, s.pickedTowardAlly(allyVeteran), "no follow row fires for a veteran nobody recruited")
	require.Equal(t, posted, s.at(allyVeteran), "he holds where he stands")
}
