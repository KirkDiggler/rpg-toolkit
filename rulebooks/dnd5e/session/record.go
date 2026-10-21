// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// record.go is THE RECORD STREAM'S SDK HALF (rpg-project#628,
// ideas/event-stream R5, R12, R13, R15, R27, Law 5): every beat the
// encounter writes, as written, for the debug panel.
//
// THE RECORD IS NOT PER RECIPIENT. A [RecordEvent] has no recipient and no
// per-member number: before release every seated client receives the whole
// record, per-observer beats included (R13, R26). It is not typed either —
// the beat's name and its fields are handed over whole; a typed reading of a
// beat is the act projection's job (E3). Its whole contract is that a reader
// can narrow it by act id and by beat name (R8).
//
// THE ID IS THE RECORD'S OWN, an ordering key and not a counter (R27): a gap
// between two ids means nothing, and loss is the transport's to report (R28).
//
// TWO ACT FORMS, UNTIL v1alpha1 IS DELETED. [Event.Correlation] keeps E1's
// `<verb>:<first seq>` for the v1alpha1 stream; [RecordEvent.Act] carries the
// v1alpha2 `<verb>:<id>`, which the encounter mints beside it at the act's
// open and stamps on each beat as the record tag [encounter.ActTag]. The SDK
// carries both by carrying the act under both names wherever it carries the
// act at all — the verb's scope, a posed window's story — and each stream
// reads its own form. Neither is derived from the other.

// RecordEvent is one record beat as written.
type RecordEvent struct {
	// Session is the session the beat belongs to — what a host's stream
	// routes by, since the record is per session, not per recipient.
	Session string

	// ID is the record's wire id: a ULID-style ordering key, compared as a
	// string. Never a count.
	ID string

	// At is the world-clock reading in rounds when the beat was written.
	At uint64

	// Act is the beat's v1alpha2 act id, `<verb>:<id>`, or "" for a beat
	// outside any act (or of an act reopened from before v2 ids existed).
	Act string

	// Beat is the payload's `beat` value verbatim — the encounter's word,
	// not this package's [EventKind]. "" when the payload names none.
	Beat string

	// Fields is the encounter's payload, whole.
	Fields json.RawMessage
}

// RecordStream is the live channel for the record. Required on [Config].
//
// Publishing is best effort, exactly as [EventStream]'s is (S10): it runs
// after the save has landed, a failure does not fail the verb, and a
// subscriber that missed beats catches up with [Manager.Record] after the
// last id it holds.
type RecordStream interface {
	// PublishRecord delivers one commit's beats, in id order.
	PublishRecord(ctx context.Context, events []RecordEvent) error
}

// DiscardRecord is a RecordStream that delivers nothing — a stated "no
// record stream" for tests and headless runs, as [DiscardEvents] is for
// events.
type DiscardRecord struct{}

// PublishRecord discards the batch and reports success.
func (DiscardRecord) PublishRecord(_ context.Context, _ []RecordEvent) error { return nil }

// RecordReadInput names a record catch-up read.
type RecordReadInput struct {
	// Session is the session whose record is read.
	Session string

	// AfterID is the last record id the caller holds; the read returns beats
	// with a greater id. Empty reads from the oldest retained beat.
	AfterID string

	// Limit caps the events returned, oldest first; zero or less is no cap.
	Limit int
}

// RecordReadOutput is the retained record after the requested id.
type RecordReadOutput struct {
	// Events are the beats after AfterID, in id order, at most Limit.
	Events []RecordEvent

	// Restarted is true when AfterID is older than retention: Events then
	// start at the oldest retained beat, and the caller replaces what it
	// holds rather than appending (R25).
	Restarted bool
}

// Record reads the session's retained record after an id (R15). It is a
// read: it saves nothing and trims nothing. Who may read it is the host's
// gate (R13); the SDK names no member.
//
// Errors: ErrNilInput; a session or world load failure.
func (m *Manager) Record(ctx context.Context, in *RecordReadInput) (*RecordReadOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("record: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	data, err := m.loadSessionData(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("record: %w", err)
	}
	enc, err := m.loadWorld(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("record: %w", err)
	}
	read, err := enc.ReadRecord(&encounter.ReadRecordInput{AfterID: in.AfterID})
	if err != nil {
		return nil, fmt.Errorf("record: %w", translate(err))
	}
	entries := read.Entries
	if in.Limit > 0 && len(entries) > in.Limit {
		entries = entries[:in.Limit]
	}
	return &RecordReadOutput{Events: recordEvents(in.Session, entries), Restarted: read.Restarted}, nil
}

// recordEvents is the one mapping from record entries to record events —
// live publish and catch-up read alike, so the two are byte-equal for the
// same id.
func recordEvents(session string, entries []record.Entry) []RecordEvent {
	if len(entries) == 0 {
		return nil
	}
	out := make([]RecordEvent, len(entries))
	for i, e := range entries {
		var peek struct {
			Beat string `json:"beat"`
		}
		_ = json.Unmarshal(e.Payload, &peek) // an unreadable payload names no beat
		out[i] = RecordEvent{
			Session: session,
			ID:      e.ID,
			At:      e.At,
			Act:     e.Tags[encounter.ActTag],
			Beat:    peek.Beat,
			Fields:  append(json.RawMessage(nil), e.Payload...),
		}
	}
	return out
}

// commitRecord is the record batch of one commit: every beat appended at or
// after the scope's baseline, read from the LIVE record before the save-point
// ToData trims it (the same reason [Manager.projectEvents] reads early).
func commitRecord(scope *writeScope) []RecordEvent {
	read, err := scope.enc.ReadRecord(&encounter.ReadRecordInput{})
	if err != nil {
		return nil // best effort: the record is still the truth, and readable
	}
	cut := len(read.Entries)
	for i, e := range read.Entries {
		if e.Seq >= scope.baseline {
			cut = i
			break
		}
	}
	return recordEvents(scope.session, read.Entries[cut:])
}

// deliverRecord hands one commit's record batch to the record stream, after
// the save (S9) and best effort (S10).
func (m *Manager) deliverRecord(ctx context.Context, events []RecordEvent) {
	if len(events) == 0 {
		return
	}
	_ = m.record.PublishRecord(ctx, events) // best effort; catch-up is Record after the last id
}
