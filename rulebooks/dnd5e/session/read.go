// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// AtlasInput asks for a session's static world map.
type AtlasInput struct {
	// Session is the session whose world to describe.
	Session string
}

// StatusInput asks whether a session's encounter is still running.
type StatusInput struct {
	// Session is the session to report on.
	Session string
}

// WhereInput asks where a member stands.
type WhereInput struct {
	// Session is the session to look inside.
	Session string

	// Member is the member whose own placement is being asked for.
	Member string
}

// ViewInput asks what one member currently perceives.
type ViewInput struct {
	// Session is the session to look inside.
	Session string

	// Member is the observer.
	Member string
}

// StoryInput asks for what a member has witnessed.
type StoryInput struct {
	// Session is the session to read from.
	Session string

	// Member is the viewer whose story is requested.
	Member string

	// FromSeq is the INCLUSIVE lower bound: entries begin AT this sequence.
	// Zero means "I hold nothing, send what you have" and is always
	// answerable, however much has aged out of the retention window.
	//
	// To resume after entry N, pass N+1. Named for what it does, unlike the
	// composition's own field, whose name predates its behaviour.
	FromSeq uint64
}

// Atlas returns the session's static world map: one set of cells, the ones
// that block sight, the walls between them, and every doorway's kissing pair.
//
// Construction truth — unchanged by movement, joins, exits or endings — so a
// host should fetch it once per session rather than per frame.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoSession, or ErrNoEncounter.
func (m *Manager) Atlas(ctx context.Context, in *AtlasInput) (*Atlas, error) {
	if in == nil {
		return nil, fmt.Errorf("atlas: %w", ErrNilInput)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("atlas: %w", err)
	}

	atlas, err := enc.Atlas()
	if err != nil {
		return nil, fmt.Errorf("atlas: %w", translate(err))
	}

	projected := projectAtlas(atlas)
	return &projected, nil
}

// Status reports whether the session's encounter is open, and how it ended if
// not.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoSession, or ErrNoEncounter.
func (m *Manager) Status(ctx context.Context, in *StatusInput) (*Status, error) {
	if in == nil {
		return nil, fmt.Errorf("status: %w", ErrNilInput)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}

	status, err := enc.Status()
	if err != nil {
		return nil, fmt.Errorf("status: %w", translate(err))
	}

	return projectStatus(enc, status), nil
}

// Where returns the cell a member stands on, in dungeon-absolute space.
//
// A client knows its own cell today only by remembering the last Move it made,
// which holds until the moment it matters: a reconnect, a fresh tab, a second
// device, a response it never received. This is the question asked directly.
//
// ONE MEMBER'S OWN PLACEMENT, and there is deliberately no read that returns
// everybody's. A roster of positions would hand a client the cells of members
// it has never perceived — around a corner, in a room it has not entered,
// behind a door it has not opened — which is the one thing perception exists to
// prevent. Where somebody ELSE is, is View's answer, and View gives it only for
// members the observer actually holds.
//
// THE HOST MUST BIND Member TO THE AUTHENTICATED CALLER. This package cannot
// know who is asking — a verb takes IDs, not identities — so the one check
// that keeps this read self-only is necessarily the host's: wire a
// client-supplied member ID through unchecked and this verb becomes the
// unperceived-position roster it refuses to be, one ID at a time, with
// monster IDs learnable from Story beats. The refusal above is only as good
// as the host's binding.
//
// It is a live read, not a stored one: the position comes from the composition's
// roster, which projects each member's cell through their room's anchor when
// asked. A member who has walked is reported where they are now.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, or ErrNoMember if the member is not in this encounter.
func (m *Manager) Where(ctx context.Context, in *WhereInput) (*WhereOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("where: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("where: %w", ErrNoMemberID)
	}

	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("where: %w", err)
	}

	members, err := enc.Members()
	if err != nil {
		return nil, fmt.Errorf("where: %w", translate(err))
	}

	// Converted once, at the boundary, and compared as the newtype it is —
	// the same direction View and Story take a member id. Comparing the other
	// way, by stringifying the composition's ID, would work today and would
	// stop being checked the moment MemberID means anything more than a string.
	want := encounter.MemberID(in.Member)
	for _, member := range members {
		if member.ID == want {
			return &WhereOutput{Position: member.Position}, nil
		}
	}
	return nil, fmt.Errorf("where: %q: %w", in.Member, ErrNoMember)
}

// View returns what one member currently perceives.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, or ErrNoMember if the member is not in this encounter.
func (m *Manager) View(ctx context.Context, in *ViewInput) ([]Sighting, error) {
	if in == nil {
		return nil, fmt.Errorf("view: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("view: %w", ErrNoMemberID)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("view: %w", err)
	}

	holdings, err := enc.View(&encounter.ViewInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("view: %w", translate(err))
	}

	return projectSightings(holdings), nil
}

// Story returns the beats a member has witnessed, from FromSeq onward
// inclusive.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoMember, or ErrStoryTrimmed when the requested resume
// point has aged out of the retention window — in which case the caller must
// resync from zero rather than resume, since a short answer would be
// indistinguishable from a complete one.
func (m *Manager) Story(ctx context.Context, in *StoryInput) ([]StoryEntry, error) {
	if in == nil {
		return nil, fmt.Errorf("story: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("story: %w", ErrNoMemberID)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("story: %w", err)
	}

	entries, err := enc.Story(&encounter.StoryInput{
		Audience: encounter.MemberID(in.Member),
		// The composition's AfterSeq is an inclusive lower bound despite its
		// name; ours says so, and the value passes through unchanged.
		AfterSeq: in.FromSeq,
	})
	if err != nil {
		return nil, fmt.Errorf("story: %w", translate(err))
	}

	return projectStory(entries), nil
}

// open loads a session and reconstitutes the encounter it points at.
//
// Every read verb begins here and none of them saves: S4's save step is simply
// empty for a read, and S1 holds — nothing is retained after the call.
func (m *Manager) open(ctx context.Context, sessionID string) (*encounter.Encounter, error) {
	if sessionID == "" {
		return nil, ErrNoSessionID
	}
	data, err := m.loadSessionData(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return m.loadWorld(ctx, data.Encounter)
}

// loadSessionData fetches session state, translating the repository's
// ErrNotFound into this package's vocabulary so hosts match on one set of
// sentinels rather than two.
func (m *Manager) loadSessionData(ctx context.Context, sessionID string) (*SessionData, error) {
	data, err := m.sessions.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%q: %w", sessionID, ErrNoSession)
		}
		return nil, err
	}
	if data == nil {
		// A repository reporting success with no data has broken its contract.
		// Reported as that rather than as a miss, and certainly not
		// dereferenced: a nil here would panic several frames later with
		// nothing pointing back to its origin.
		return nil, fmt.Errorf(
			"%q: GetSession reported success with no data: %w", sessionID, ErrBadRepository)
	}
	return data, nil
}

// loadWorld fetches and reconstitutes an encounter by ID.
func (m *Manager) loadWorld(ctx context.Context, encID string) (*encounter.Encounter, error) {
	enc, _, err := m.loadWorldWithBaseline(ctx, encID)
	return enc, err
}

// loadWorldWithBaseline additionally reports the log's next sequence at load
// time — the boundary between what was already recorded and what this verb is
// about to record.
//
// Read out of the blob that was loaded anyway, so it costs nothing. Anything
// with a sequence at or above the baseline is news, which is how a verb knows
// which beats to fan out without the composition having to report them.
func (m *Manager) loadWorldWithBaseline(
	ctx context.Context, encID string,
) (*encounter.Encounter, uint64, error) {
	world, err := m.encounters.GetEncounter(ctx, encID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, 0, fmt.Errorf("%q: %w", encID, ErrNoEncounter)
		}
		return nil, 0, err
	}
	if world == nil {
		return nil, 0, fmt.Errorf(
			"%q: GetEncounter reported success with no data: %w", encID, ErrBadRepository)
	}

	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       *world,
		Initiative: m.initiative,
	})
	if err != nil {
		// The reason is kept as TEXT, not as a chain. A blob this seam cannot
		// reconstitute fails several modules deep — the composition's own
		// validation, or a leaf's (clock, intel, record) underneath it — and
		// every one of those is a module we intend to keep replaceable. %v
		// hands whoever debugs it the whole account and hands a host nothing to
		// match on but ours (S2).
		return nil, 0, fmt.Errorf("%q: %w: %v", encID, ErrInvalidWorld, err)
	}
	return enc, world.Log.NextSeq, nil
}

// translate maps the composition's sentinels onto this package's own.
//
// The boundary test cannot see this. It reads exported signatures, and a
// sentinel error is not a type in a signature — so an inner package can become
// part of the host's contract through a channel the AST never looks at. If
// hosts matched on encounter.ErrTrimmed, replacing the composition would break
// their error handling exactly as surely as leaking a struct would, and
// nothing in CI would have said a word.
//
// Unrecognised errors pass through UNCHANGED rather than being flattened. This
// function adds nothing to them; the calling verb wraps what comes back with
// its own prefix, which is where "view:" and "move:" come from.
//
// That limit is deliberate rather than an oversight. The default arm carries
// errors that ORIGINATED WITH THE HOST as well as composition ones we did not
// anticipate — a failing Roller comes back out through a composition verb — and
// flattening those would break the host's matching on its own errors to protect
// it from ours. So the guarantee here is that every sentinel this seam can
// reach has an arm, and the mechanical part of that guarantee lives in the
// tests: sentinels_test.go over the refusals a caller can drive, and
// translate_internal_test.go over every arm below (rpg-toolkit#1058).
//
// A translated error carries our sentinel ALONE. Wrapping both — fmt.Errorf(
// "%w: %w", ours, theirs) — reads like generosity and is the leak itself: a
// host can still match on theirs. Where the inner message is worth keeping, the
// call site keeps it with %v, which is text rather than a chain.
func translate(err error) error {
	switch {
	case errors.Is(err, encounter.ErrTrimmed):
		return fmt.Errorf("%w", ErrStoryTrimmed)
	case errors.Is(err, encounter.ErrNoMember), errors.Is(err, encounter.ErrNotMember):
		return fmt.Errorf("%w", ErrNoMember)
	case errors.Is(err, encounter.ErrClosed):
		return fmt.Errorf("%w", ErrClosed)
	case errors.Is(err, encounter.ErrNoEnding):
		return fmt.Errorf("%w", ErrNoEnding)
	case errors.Is(err, encounter.ErrNoConnection), errors.Is(err, encounter.ErrBadConnection):
		return fmt.Errorf("%w", ErrNoConnection)
	case errors.Is(err, encounter.ErrBadPlacement):
		return fmt.Errorf("%w", ErrBadPosition)
	case errors.Is(err, encounter.ErrNoField), errors.Is(err, encounter.ErrInvalidData):
		// Both mean the stored world cannot answer: a field that is defective
		// or does not hold the room somebody stands in, and a blob that cannot
		// be reconstituted at all. One sentinel because the host's remedy is
		// the same either way — this encounter's data is unusable, and the
		// repair is upstream of anything a caller can retry.
		return fmt.Errorf("%w", ErrInvalidWorld)
	case errors.Is(err, encounter.ErrInBubble):
		return fmt.Errorf("%w", ErrInBubble)
	case errors.Is(err, encounter.ErrNoBubble):
		return fmt.Errorf("%w", ErrNotInFight)
	default:
		return err
	}
}
