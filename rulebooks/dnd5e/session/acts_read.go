// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
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

// ActListeners is what an [ActStream] may also be: the stream's own answer
// to who can receive a live act in a session right now — the members with a
// subscriber open. It is optional. A stream that implements it has the
// commit fold acts only for the members it names; a stream that does not
// gets every member ever in the run, as before.
//
// NOBODY LOSES AN ACT BY NOT LISTENING. Catch-up is [Manager.Acts], which
// folds on demand from the member's own retained story through the same
// fold, so a subscriber that arrives after a commit reads, after the last
// id it holds, exactly the acts it would have been handed live.
//
// THAT RELIES ON ONE ORDER: a client subscribes to the live stream BEFORE it
// reads catch-up (as the api's StreamEvents does). A member counted as
// listening from the moment it subscribes, and caught up after, has no gap:
// a commit before the subscription is in the catch-up read, and one after it
// is folded live. A client that read catch-up first and subscribed later
// would miss the commits between the two. The same order makes the race
// between Listening and PublishActs harmless: a member that subscribes after
// a commit asked Listening is not folded for that commit, and its catch-up
// read, made after it subscribed, returns that act.
//
// Listening is asked once per commit, after the verb and before the save,
// and must be cheap. An error is not a verb failure: the commit folds for
// every member, as a stream that cannot say would get. A name that is not a
// member of the run is ignored.
type ActListeners interface {
	// Listening names the members of session who can receive a live act.
	Listening(ctx context.Context, session string) ([]string, error)
}

// actStory is the one story read the live fold makes: member's story from
// the commit's baseline, widened to the acts it continues. A package
// variable so a test can watch what each commit reads, against the baseline
// it was given rather than the read's own input ([WatchActFoldForTest]).
var actStory = func(enc *encounter.Encounter, baseline uint64, member encounter.MemberID) ([]record.Entry, error) {
	return enc.Story(&encounter.StoryInput{Audience: member, AfterSeq: baseline, WholeActs: true})
}

// commitActs is one commit's act batch: for each recipient, the acts after
// the last beat that recipient held before this verb, folded from the LIVE
// story before the save-point ToData trims it (the same reason
// [commitRecord] reads early).
//
// THE FOLD READS THE COMMIT'S DELTA. Each recipient's story is read from
// from, the scope's commit cursor ([writeScope.deltaFrom]): the act's
// baseline, or after a pace commit landed the first seq it did not hold, widened back only to the first beat of an act the delta
// continues ([encounter.StoryInput.WholeActs]) — exactly the entries
// [actsAfter] folds from a whole story — so what a commit reads follows the
// verb and the acts it touched, never the length of the record. The held id
// is the last entry of that slice before the baseline; when the slice holds
// none, no act it folds can be at or before what the recipient held, and
// actsAfter's cut is "".
func commitActs(ctx context.Context, scope *writeScope, stream ActStream, members []encounter.MemberID, from uint64) []Act {
	var out []Act
	for _, member := range actRecipients(ctx, scope.session, stream, members) {
		entries, err := actStory(scope.enc, from, member)
		if err != nil {
			continue // best effort: one unreadable recipient must not silence the rest
		}
		held := ""
		for _, e := range entries {
			if e.Seq >= from {
				break
			}
			held = e.ID
		}
		out = append(out, actsAfter(scope.session, string(member), entries, held)...)
	}
	return out
}

// actRecipients is who the commit folds live acts for: the members the
// stream says are listening, in the run's own member order, or every member
// when the stream cannot say ([ActListeners]).
func actRecipients(ctx context.Context, session string, stream ActStream, members []encounter.MemberID) []encounter.MemberID {
	listeners, ok := stream.(ActListeners)
	if !ok {
		return members
	}
	listening, err := listeners.Listening(ctx, session)
	if err != nil {
		return members // best effort: a stream that cannot say gets everyone
	}
	named := make(map[string]bool, len(listening))
	for _, id := range listening {
		named[id] = true
	}
	var out []encounter.MemberID
	for _, member := range members {
		if named[string(member)] {
			out = append(out, member)
		}
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
