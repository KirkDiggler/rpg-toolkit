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

// Atlas returns the session's static world map: every room's absolute
// footprint and every doorway's kissing pair.
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
		return nil, fmt.Errorf("atlas: %w", err)
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
		return nil, fmt.Errorf("status: %w", err)
	}

	return projectStatus(status), nil
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

	data, err := m.sessions.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%q: %w", sessionID, ErrNoSession)
		}
		return nil, err
	}
	if data == nil {
		// A repository reporting success with no data is broken, not empty.
		// Treated as a miss rather than dereferenced, because a nil here would
		// otherwise panic several frames later with nothing pointing back here.
		return nil, fmt.Errorf("%q: repository returned no data: %w", sessionID, ErrNoSession)
	}

	world, err := m.encounters.GetEncounter(ctx, data.Encounter)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%q: %w", data.Encounter, ErrNoEncounter)
		}
		return nil, err
	}
	if world == nil {
		return nil, fmt.Errorf("%q: repository returned no data: %w", data.Encounter, ErrNoEncounter)
	}

	enc, err := encounter.LoadEncounter(*world, nil)
	if err != nil {
		return nil, fmt.Errorf("%q: %w: %w", data.Encounter, ErrInvalidWorld, err)
	}
	return enc, nil
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
// Unrecognised errors pass through wrapped rather than being flattened: an
// error we did not anticipate is more useful with its own message intact than
// re-labelled as something we do recognise.
func translate(err error) error {
	switch {
	case errors.Is(err, encounter.ErrTrimmed):
		return fmt.Errorf("%w", ErrStoryTrimmed)
	case errors.Is(err, encounter.ErrNoMember), errors.Is(err, encounter.ErrNotMember):
		return fmt.Errorf("%w", ErrNoMember)
	default:
		return err
	}
}
