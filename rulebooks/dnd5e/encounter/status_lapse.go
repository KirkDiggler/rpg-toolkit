// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// status_lapse.go is A STATUS GIVEN FOR A NUMBER OF ROUNDS (rpg-project#1080
// S2): one timer per application, armed by the host after the rules answer
// that a giving was fresh, saved with the run, and fired as one
// [StatusLapsed] boundary to that one holder when the world clock reaches
// the round it is due.
//
// THE ENCOUNTER NEVER SEES A STATUS ENDING. It cannot read sheets, and none
// of the existing endings passes through it, so a timer whose status already
// ended another way fires harmlessly: the boundary reaches the rules, no part
// listens for that application, nothing changes. The danger, a stale timer
// ending a later giving of the same status, is closed by the host: every
// fresh giving arms (replacing) or clears.

const (
	minStatusLapseRounds = 1
	maxStatusLapseRounds = 1000
)

// StatusLapseData is one armed timer, saved: who holds the status, which
// status, the application it ends, and the world round it is due.
type StatusLapseData struct {
	// Member is the holder.
	Member MemberID `json:"member"`
	// Status is the status's id in its site's `statuses:`.
	Status string `json:"status"`
	// Application is the host's own id for this one status on this one
	// holder. Opaque here; the key of the timer.
	Application string `json:"application"`
	// At is the world round it is due: it fires when the high-water reaches it.
	At uint64 `json:"at"`
}

// StatusUntilRefusal is the sentence for a rounds count no status can be
// given for, or "" when rounds is 1 to 1000. The content compiler and the
// engine say the same thing.
func StatusUntilRefusal(rounds int) string {
	if rounds < minStatusLapseRounds || rounds > maxStatusLapseRounds {
		return "rounds must be a whole number of one or more, at most 1000, such as 3"
	}
	return ""
}

// ArmStatusLapseInput is [Encounter.ArmStatusLapse]'s input.
type ArmStatusLapseInput struct {
	// Member is the holder; it must be a member.
	Member MemberID
	// Status is the status's id.
	Status string
	// Application is the host's id for this status on this holder; arming
	// the same application again replaces its timer.
	Application string
	// Rounds is how many rounds of world time, 1 to 1000.
	Rounds int
}

// ArmStatusLapse starts a timer at the world's high-water plus Rounds,
// replacing any timer for the same application. The host calls it after the
// rules answer that a giving was fresh; an ignored second giving never calls
// it.
//
// Errors: [ErrNilInput]; [ErrNotMember]; [ErrInvalidData] for an empty status
// or application, or rounds outside 1 to 1000.
func (e *Encounter) ArmStatusLapse(in *ArmStatusLapseInput) error {
	if in == nil {
		return fmt.Errorf("arm status lapse: %w", ErrNilInput)
	}
	if _, ok := e.members[in.Member]; !ok {
		return fmt.Errorf("arm status lapse: member %q: %w", in.Member, ErrNotMember)
	}
	if in.Status == "" || in.Application == "" {
		return fmt.Errorf("arm status lapse: a status and an application are required: %w", ErrInvalidData)
	}
	if why := StatusUntilRefusal(in.Rounds); why != "" {
		return fmt.Errorf("arm status lapse: %s: %w", why, ErrInvalidData)
	}
	at := uint64(e.clock.ToData().HighWater) + uint64(in.Rounds)
	e.armLapse(StatusLapseData{Member: in.Member, Status: in.Status, Application: in.Application, At: at})
	return nil
}

// ClearStatusLapseInput is [Encounter.ClearStatusLapse]'s input.
type ClearStatusLapseInput struct {
	// Application is the timer to take off.
	Application string
}

// ClearStatusLapse takes an application's timer off, so a stale one can never
// end a later giving of the same status. An application with no timer is no
// error.
//
// Errors: [ErrNilInput].
func (e *Encounter) ClearStatusLapse(in *ClearStatusLapseInput) error {
	if in == nil {
		return fmt.Errorf("clear status lapse: %w", ErrNilInput)
	}
	e.lapses.Disarm(in.Application)
	delete(e.lapseAt, in.Application)
	return nil
}

// armLapse puts one timer on the heap and in the record beside it.
func (e *Encounter) armLapse(d StatusLapseData) {
	if e.lapseAt == nil {
		e.lapseAt = map[string]StatusLapseData{}
	}
	e.lapses.Arm(d.Application, d.At)
	e.lapseAt[d.Application] = d
}

// lapseDue announces one [StatusLapsed] boundary for every timer due at the
// world's high-water, in application order. Each fired entry is deleted
// BEFORE it is announced, so a failure cannot fire it twice in a retry of the
// verb.
func (e *Encounter) lapseDue() error {
	if len(e.lapseAt) == 0 {
		return nil
	}
	due := e.lapses.Pop(uint64(e.clock.ToData().HighWater))
	if len(due) == 0 {
		return nil
	}
	crossed := make([]Boundary, 0, len(due))
	for _, application := range due {
		d, ok := e.lapseAt[application]
		delete(e.lapseAt, application)
		if !ok {
			continue
		}
		crossed = append(crossed, Boundary{Kind: StatusLapsed, Subject: d.Member, Application: application})
	}
	return e.announceBoundaries(crossed)
}

// dropLapsesOf takes every timer of a member off, for a member leaving the
// run: nothing fires about somebody who is gone.
func (e *Encounter) dropLapsesOf(member MemberID) {
	for application, d := range e.lapseAt {
		if d.Member == member {
			e.lapses.Disarm(application)
			delete(e.lapseAt, application)
		}
	}
}

// lapsesData is the saved record, sorted by application; nil when none.
func (e *Encounter) lapsesData() []StatusLapseData {
	if len(e.lapseAt) == 0 {
		return nil
	}
	out := make([]StatusLapseData, 0, len(e.lapseAt))
	for _, d := range e.lapseAt {
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b StatusLapseData) int {
		switch {
		case a.Application < b.Application:
			return -1
		case a.Application > b.Application:
			return 1
		}
		return 0
	})
	return out
}

// restoreLapses validates the saved timers against the restored roster and
// arms the heap. Refused with [ErrInvalidData]: a member not in the roster,
// an empty application, an application listed twice, or a round of 0. An older
// save has none, so nothing is armed.
func (e *Encounter) restoreLapses(saved []StatusLapseData) error {
	e.lapses = machine.Due[string]{}
	e.lapseAt = nil
	for _, d := range saved {
		if _, ok := e.members[d.Member]; !ok {
			return fmt.Errorf("status lapse %q names member %q who is not in the run: %w", d.Application, d.Member, ErrInvalidData)
		}
		if d.Application == "" {
			return fmt.Errorf("a status lapse has no application: %w", ErrInvalidData)
		}
		if d.At == 0 {
			return fmt.Errorf("status lapse %q is due at round 0: %w", d.Application, ErrInvalidData)
		}
		if _, dup := e.lapseAt[d.Application]; dup {
			return fmt.Errorf("status lapse %q is listed twice: %w", d.Application, ErrInvalidData)
		}
		e.armLapse(d)
	}
	return nil
}
