// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// acts_read.go is the act stream's two paths — the commit's live batch and
// [Manager.Acts]'s catch-up — and the one fold ([actsAfter]) both call.

// ActsInput names an act catch-up read.
type ActsInput struct {
	// Session is the session whose acts are read.
	Session string

	// Member is the recipient: the acts are folded from this member's own
	// story, so they hold only what this member perceived.
	Member string

	// AfterID is the id of the last act the caller holds — a record id
	// (R31). The read returns the acts after it. Empty reads from the oldest
	// retained beat.
	AfterID string
}

// ActsOutput is the member's acts after the requested id.
type ActsOutput struct {
	// Acts are the member's act segments after AfterID, in record order.
	Acts []Act

	// Restarted is true when AfterID is older than retention: Acts then
	// start at the oldest retained beat, and the caller replaces what it
	// holds rather than appending (R25).
	Restarted bool
}

// Acts reads one member's acts after an id — the catch-up half of the act
// stream. It folds through the same function the commit folds live acts
// through, so the acts after a segment's id are the acts a live subscriber
// received after it.
//
// It is a read: it saves nothing and trims nothing.
//
// Errors: ErrNilInput; ErrNoMemberID; ErrBadRecordID for an AfterID that is
// not a record id; a session or world load failure.
func (m *Manager) Acts(ctx context.Context, in *ActsInput) (*ActsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("acts: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if in.Member == "" {
		return nil, fmt.Errorf("acts: %w", ErrNoMemberID)
	}
	data, err := m.loadSessionData(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("acts: %w", err)
	}
	enc, err := m.loadWorld(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("acts: %w", err)
	}
	// The record answers the retention question, by the record's own rule:
	// an AfterID below a trimmed floor restarts the read.
	read, err := enc.ReadRecord(&encounter.ReadRecordInput{AfterID: in.AfterID})
	if errors.Is(err, encounter.ErrInvalidData) {
		return nil, fmt.Errorf("acts: after id %q: %w", in.AfterID, ErrBadRecordID)
	}
	if err != nil {
		return nil, fmt.Errorf("acts: %w", translate(err))
	}
	entries, err := enc.Story(&encounter.StoryInput{Audience: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("acts: %w", translate(err))
	}
	after := in.AfterID
	if read.Restarted {
		after = ""
	}
	return &ActsOutput{Acts: actsAfter(in.Session, in.Member, entries, after), Restarted: read.Restarted}, nil
}

// commitActs is one commit's act batch: for each member ever in the run, the
// acts after the last beat that member held before this verb, folded from
// the LIVE story before the save-point ToData trims it (the same reason
// [commitRecord] reads early).
func commitActs(scope *writeScope, members []encounter.MemberID) []Act {
	var out []Act
	for _, member := range members {
		entries, err := scope.enc.Story(&encounter.StoryInput{Audience: member})
		if err != nil {
			continue // best effort: one unreadable recipient must not silence the rest
		}
		held := ""
		for _, e := range entries {
			if e.Seq >= scope.baseline {
				break
			}
			held = e.ID
		}
		out = append(out, actsAfter(scope.session, string(member), entries, held)...)
	}
	return out
}

// deliverActs hands one commit's acts to the act stream, after the save (S9)
// and best effort (S10).
func (m *Manager) deliverActs(ctx context.Context, acts []Act) error {
	if len(acts) == 0 {
		return nil
	}
	return m.acts.PublishActs(ctx, acts) // best effort; catch-up is Acts after the last id
}
