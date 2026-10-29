// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_fell_test.go is FRANK'S LIST S2 in the castle (rpg-project#716):
// the king falls in sight of one knight and out of sight of the other. The
// knight who saw it holds the fall, and his `fell: { who: king }` row is on
// his table; the knight who did not holds nothing, and the row is not.
//
// THE RUN OPENS WITHOUT `king-slain`. The file's ending settles the run the
// moment the king is down, so no knight would ever pick again; this route
// plays past it to read the row the fall puts on a table. Whether the castle
// should end there is the file's call, not this route's.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// fellRowEntry is the knights' `fell` row: the first of keep-patrol's and
// keep-patrol-east's `time` rows.
const fellRowEntry = 0

// kingFighter is a second player, standing at keep-2 so the far knight has
// a fight of its own and so a turn to pick on.
const kingFellFighter = encounter.MemberID("fighter")

// pickCandidates is the fields of an `answered` beat this route reads.
type pickCandidates struct {
	Beat       string `json:"beat"`
	Creature   string `json:"creature"`
	Candidates []struct {
		Entry int `json:"entry"`
	} `json:"candidates"`
}

// openTheFall opens the castle at twenty feet of sight with the hero beside
// the king and a fighter beside keep-2, ten cells off: keep-1 sees the king
// and the hero, keep-2 sees neither and fights the fighter.
func openTheFall(t *testing.T) *kingScene {
	t.Helper()
	var keep2 spatial.Position
	for _, m := range loadContent(t, kingFixture).Monsters {
		id := encounter.MemberID(m.MemberID)
		if id == kingKeep2 {
			keep2 = m.At
		}
		if id == kingKeep1 || id == kingKeep2 {
			row := m.Table[encounter.AnswerTime][fellRowEntry]
			require.Equal(t, []encounter.When{{Deed: "fell", Within: 3, Who: theKing, Scope: encounter.ScopeAlly}},
				row.When, "precondition: %s's first row waits on the king's fall", id)
		}
	}
	s := openTheCastleAs(t, castleHost{
		at:      besideTheKing,
		sightFt: 20,
		without: []string{"king-slain"},
		players: []encounter.MemberInput{{
			ID: kingFellFighter, Kind: encounter.KindPlayer, Position: spatial.Position{X: keep2.X + 1, Y: keep2.Y},
		}},
	})
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, kingKeep1), "precondition: keep-1 saw the hero")
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, kingKeep2), "precondition: keep-2 saw the fighter")
	require.False(t, s.seesLive(kingKeep2, theKing), "precondition: keep-2 cannot see the king")
	require.True(t, s.seesLive(kingKeep1, theKing), "precondition: keep-1 can")

	return s
}

// heldFall is the king's fall by the hero as a knight holds it, decoded:
// a fall is filed by who fell too ([deed.FiledUnder]).
func (s *kingScene) heldFall(knight encounter.MemberID) (deed.Deed, bool) {
	s.t.Helper()
	held, err := s.enc.View(&encounter.ViewInput{Member: knight})
	require.NoError(s.t, err)
	for _, h := range held {
		if h.Channel == deed.Channel &&
			h.Subject == deed.FiledUnder(deed.Deed{Verb: encounter.DeedFell, Actor: hero, Target: theKing}) {
			saw, derr := deed.Decode(h.Payload)
			require.NoError(s.t, derr)

			return saw, true
		}
	}

	return deed.Deed{}, false
}

// candidatesOf is every row each of a creature's picks put on its table, one
// list per pick, in story order.
func (s *kingScene) candidatesOf(creature encounter.MemberID) [][]int {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: hero})
	require.NoError(s.t, err)
	var out [][]int
	for _, entry := range story {
		var b pickCandidates
		require.NoError(s.t, json.Unmarshal(entry.Payload, &b))
		if b.Beat != encounter.BeatAnswered || b.Creature != string(creature) {
			continue
		}
		rows := make([]int, 0, len(b.Candidates))
		for _, c := range b.Candidates {
			rows = append(rows, c.Entry)
		}
		out = append(out, rows)
	}

	return out
}

// playUntilBothKnightsPick ends each player's turn as it comes up until
// both knights have picked past the counts given.
func (s *kingScene) playUntilBothKnightsPick(keep1, keep2 int) {
	s.t.Helper()
	for range 8 {
		if len(s.candidatesOf(kingKeep1)) > keep1 && len(s.candidatesOf(kingKeep2)) > keep2 {
			return
		}
		for _, player := range []encounter.MemberID{hero, kingFellFighter} {
			on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: player})
			require.NoError(s.t, err)
			if on.Kind != encounter.ClockTurn || on.Active != player {
				continue
			}
			s.reload()
			_, err = s.enc.EndTurn(&encounter.EndTurnInput{Member: player})
			require.NoError(s.t, err, "%s ends its turn", player)
		}
	}
	s.t.Fatal("the knights never both picked")
}

// TestMadKingAKnightWhoSawTheKingFallHasTheRow is the slice's checklist line:
// drop the king in sight of one loyalist and out of sight of another, and the
// Debug record shows the `fell` row eligible for the first and absent for the
// second.
func TestMadKingAKnightWhoSawTheKingFallHasTheRow(t *testing.T) {
	t.Parallel()
	s := openTheFall(t)
	before := len(s.candidatesOf(kingKeep1))

	s.struck(theKing, true)

	t.Run("the knight who saw it holds the fall, and who caused it", func(t *testing.T) {
		saw, ok := s.heldFall(kingKeep1)
		require.True(t, ok, "keep-1 saw the king fall")
		require.Equal(t, hero, saw.Actor, "the hero dropped him")
		require.Equal(t, theKing, saw.Target, "and the king is who fell")
	})

	t.Run("the knight who did not holds nothing", func(t *testing.T) {
		_, ok := s.heldFall(kingKeep2)
		require.False(t, ok, "keep-2 was ten cells off, out of sight")
	})

	s.playUntilBothKnightsPick(before, len(s.candidatesOf(kingKeep2)))

	t.Run("keep-1's next pick has the fell row on its table", func(t *testing.T) {
		picks := s.candidatesOf(kingKeep1)
		require.Greater(t, len(picks), before, "keep-1 picked after the fall")
		require.Contains(t, picks[len(picks)-1], fellRowEntry, "the row that waits on the king's fall is a candidate")
	})

	t.Run("keep-2's pick does not", func(t *testing.T) {
		picks := s.candidatesOf(kingKeep2)
		require.NotEmpty(t, picks, "keep-2 picked against the fighter")
		require.NotContains(t, picks[len(picks)-1], fellRowEntry, "a fall he never saw is no row of his")
	})
}
