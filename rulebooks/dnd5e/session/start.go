// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// StartSessionInput begins a session in a copy of an authored world.
type StartSessionInput struct {
	// Session is the ID for the new session. Must not already exist.
	Session string

	// Encounter is the ID the session's own copy of the world is stored under.
	//
	// Supplied by the caller rather than derived, because a session outlives
	// any single encounter: a party moves from a tomb to a town to a quest, and
	// a derived one-to-one identity would have to be undone the first time that
	// happened.
	Encounter string

	// World is the authored content this session plays in — a tomb from a
	// content pipeline, not a live encounter.
	//
	// Passed in rather than fetched because the host already knows where its
	// content lives, and this is the only moment in a session's life that reads
	// it. A content repository would be a permanent interface for a
	// once-per-session lookup.
	//
	// It is copied, not referenced: two parties running the same tomb get their
	// own worlds, and neither can move the other's ogre.
	World *encounter.EncounterData
}

// StartSessionOutput reports what the new session is and what was written.
type StartSessionOutput struct {
	// Session is the new session's ID.
	Session string

	// Saved names the aggregates that were persisted, and any that were not.
	Saved SaveReport
}

// StartSession creates a session playing in a copy of the given authored world.
//
// Validation order, all before anything is written (nothing observable until
// the whole call succeeds): nil input, empty session ID, empty encounter ID,
// nil world, unloadable world, session already exists.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoEncounterID, ErrInvalidWorld,
// ErrSessionExists, or ErrSaveFailed with a populated SaveReport.
func (m *Manager) StartSession(ctx context.Context, in *StartSessionInput) (*StartSessionOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("startsession: %w", ErrNilInput)
	}
	if in.Session == "" {
		return nil, fmt.Errorf("startsession: %w", ErrNoSessionID)
	}
	if in.Encounter == "" {
		return nil, fmt.Errorf("startsession: %w", ErrNoEncounterID)
	}
	if in.World == nil {
		return nil, fmt.Errorf("startsession: nil world: %w", ErrInvalidWorld)
	}

	// Prove the world loads before persisting it. An authored blob that cannot
	// be reconstituted must be rejected at the door: writing it would create a
	// session that exists, looks fine to the host, and fails permanently on the
	// next verb — the worst of the three possible outcomes.
	//
	// The loaded encounter is deliberately discarded. This is a validation
	// call, and the manager holds no domain state between verbs (S1); the next
	// verb loads its own.
	if _, err := m.loadAuthored(ctx, in.World); err != nil {
		return nil, fmt.Errorf("startsession: %w", err)
	}

	// Refuse to overwrite a session in progress. A miss is the expected path
	// here, so only a real failure is propagated: a store that is down must not
	// be mistaken for a free ID and answered by clobbering whatever is actually
	// there once it recovers.
	// Three outcomes, and the third is the one worth spelling out. A repository
	// reporting success with no data has broken its contract, and neither
	// available guess is safe: reading it as "exists" gives a misleading error
	// for a session that may not be there, and reading it as "free" risks
	// overwriting one that is. So it is refused as what it actually is.
	existing, err := m.sessions.GetSession(ctx, in.Session)
	switch {
	case err == nil && existing != nil:
		return nil, fmt.Errorf("startsession: %q: %w", in.Session, ErrSessionExists)
	case err == nil:
		return nil, fmt.Errorf(
			"startsession: %q: GetSession reported success with no data: %w",
			in.Session, ErrBadRepository)
	case errors.Is(err, ErrNotFound):
		// Expected: the ID is free.
	default:
		return nil, fmt.Errorf("startsession: checking for an existing session: %w", err)
	}

	// Write the world first, then the session that points at it.
	//
	// The order is the failure mode. If the world lands and the session does
	// not, the leftover is an orphaned encounter — garbage, collectable, and
	// harmless, and the host can simply retry. Reversed, a session would be
	// persisted pointing at a world that was never written: a record that looks
	// healthy and is permanently broken. Given a partial failure is possible at
	// all (atomicity is deliberately unsolved for now), it should leave the
	// recoverable wreck rather than the convincing one.
	var report SaveReport

	if err := m.encounters.SaveEncounter(ctx, in.Encounter, in.World); err != nil {
		report.Failed = append(report.Failed, "encounter:"+in.Encounter)
		return nil, fmt.Errorf("startsession: saving world: %w: %w", ErrSaveFailed, err)
	}
	report.Written = append(report.Written, "encounter:"+in.Encounter)

	data := &SessionData{ID: in.Session, Encounter: in.Encounter}
	if err := m.sessions.SaveSession(ctx, data); err != nil {
		report.Failed = append(report.Failed, "session:"+in.Session)
		return nil, fmt.Errorf("startsession: saving session: %w: %w", ErrSaveFailed, err)
	}
	report.Written = append(report.Written, "session:"+in.Session)

	return &StartSessionOutput{Session: in.Session, Saved: report}, nil
}

// loadAuthored reconstitutes an authored world that no session holds yet.
//
// The capabilities are supplied over NO session record, which is the honest
// answer at this moment rather than a shortcut: no session exists, so nothing
// has been spawned and there are no session-scoped sheets to read. Real ones
// are built anyway — capabilities are supplied, never defaulted, and handing
// over a stand-in here would be a second answer to a question that has one.
// The Striker is the construction-only one (rpg-project#254): a driven turn
// reaching a world loaded here would be a bug in this package rather than
// anything a caller did.
//
// Two callers, one load. [Manager.StartSession] proves a world can be
// reconstituted before persisting it, and [Manager.AtlasOf] projects the map
// of a world nobody has started — and the day those two loaded differently
// would be the day a dungeon that previews fine refuses to start, or the
// reverse.
//
// Returns ErrInvalidWorld for a nil world or one that will not load. The
// reason is kept as TEXT, not as a chain: a blob this seam cannot reconstitute
// fails several modules deep, and every one of those is a module we intend to
// keep replaceable (S2).
func (m *Manager) loadAuthored(ctx context.Context, world *encounter.EncounterData) (*encounter.Encounter, error) {
	if world == nil {
		return nil, fmt.Errorf("nil world: %w", ErrInvalidWorld)
	}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       *world,
		Initiative: m.initiative,
		Standing:   m.standingFor(ctx, nil),
		Sight:      &sightSeam{members: append([]encounter.MemberData(nil), world.Members...)},
		TurnDriver: m.turnDriver,
		Striker:    encounter.RefusingStriker{},
		// Authored worlds are loaded to be inspected and re-serialized, never
		// driven — no clock advances here. Same reasoning as the Striker above.
		Announcer: encounter.RefusingAnnouncer{},
		// And the concealment pair: an authored world is never searched and
		// never refreshes sight, so this package's refusing stand-ins hold
		// the same line (conceal.go). AtlasOf deliberately answers the WHOLE
		// truth for an authored world — the author's own view — which is
		// why it stays on the unscoped Atlas.
		CheckResolver: refusingCheckResolver{},
		Witness:       refusingWitness{},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWorld, err)
	}
	return enc, nil
}
