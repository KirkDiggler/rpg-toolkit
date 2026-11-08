// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

type PendingWritesSuite struct{ suite.Suite }

func TestPendingWritesSuite(t *testing.T) { suite.Run(t, new(PendingWritesSuite)) }

// TestAFencedVerbReadsItsOwnSheetUntilItsGuardIsReleased: a read-only sheet
// store (the kind most read sites build) sees a fenced verb's unsaved sheet
// and seat, a copy rather than the writer's pointer, and releasing the
// session's guard forgets them.
func (s *PendingWritesSuite) TestAFencedVerbReadsItsOwnSheetUntilItsGuardIsReleased() {
	m := &Manager{pending: newPendingWrites()}
	release, err := m.acquireSession(context.Background(), "sess")
	s.Require().NoError(err)
	scope := &writeScope{session: "sess", fence: checkpointFence{held: true}}

	written := &character.Data{ID: "alice", HitPoints: 7}
	s.Require().NoError(m.sheetsFor(scope).save(context.Background(), written))
	s.Require().NoError(m.writeSeat(context.Background(), scope, "alice", "sess"))
	written.HitPoints = 1

	read, err := m.sheetsFor(nil).load(context.Background(), "member", "alice")
	s.Require().NoError(err)
	s.Equal(7, read.HitPoints, "the read sees the write, not the writer's later edit")
	seat, err := m.seatOf(context.Background(), "alice")
	s.Require().NoError(err)
	s.Equal("sess", seat)
	s.Empty(scope.written, "an unsaved sheet is not on the report")
	s.True(scope.sheetsWritten["alice"], "but it is a touched member")

	release()

	_, held, err := m.pending.sheet("alice")
	s.Require().NoError(err)
	s.False(held, "the guard's release forgot the unsaved sheet")
	_, held = m.pending.seat("alice")
	s.False(held, "and the unsaved seat")
}

// TestTakeIsTheSessionsOwnInIDOrder: the fenced save takes only its own
// session's pieces, sorted, and leaves another session's alone.
func (s *PendingWritesSuite) TestTakeIsTheSessionsOwnInIDOrder() {
	p := newPendingWrites()
	for _, id := range []string{"carol", "alice"} {
		s.Require().NoError(p.putSheet("sess", &character.Data{ID: id}))
	}
	s.Require().NoError(p.putSheet("other", &character.Data{ID: "bob"}))
	p.putSeat("sess", SeatData{Character: "carol", Session: ""})

	sheets, seats := p.take("sess")

	var sheetIDs, seatIDs []string
	for _, sheet := range sheets {
		sheetIDs = append(sheetIDs, sheet.ID)
	}
	for _, seat := range seats {
		seatIDs = append(seatIDs, seat.Character)
	}
	s.Equal([]string{"alice", "carol"}, sheetIDs)
	s.Equal([]string{"carol"}, seatIDs)
	_, held, err := p.sheet("bob")
	s.Require().NoError(err)
	s.True(held, "another session's sheet is not taken")
	again, _ := p.take("sess")
	s.Empty(again, "taken pieces are gone")
}

// TestAWriteAfterTheFencedSaveIsRefused: a piece written once the scope's
// fenced save has landed has nothing left to ride, so it is refused rather
// than dropped silently at the guard's release (rpg-toolkit#2332).
func (s *PendingWritesSuite) TestAWriteAfterTheFencedSaveIsRefused() {
	m := &Manager{pending: newPendingWrites()}
	scope := &writeScope{session: "sess", fence: checkpointFence{held: true, saved: true}}

	err := m.sheetsFor(scope).save(context.Background(), &character.Data{ID: "alice"})
	s.ErrorIs(err, ErrBadRepository)
	s.ErrorIs(err, ErrSaveFailed)
	err = m.writeSeat(context.Background(), scope, "alice", "")
	s.ErrorIs(err, ErrBadRepository)

	sheets, seats := m.pending.take("sess")
	s.Empty(sheets, "nothing was held for a save that already landed")
	s.Empty(seats)
}
