// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// ErrStaleCheckpoint means a fenced save found the store's checkpoint version
// moved since this verb loaded it: another writer saved in between. Nothing
// this save carried landed, and the verb delivers nothing (rpg-project#879
// S12). Match it with errors.Is; the verb's [SaveError] unwraps to it.
var ErrStaleCheckpoint = errors.New("session checkpoint is stale")

// FencedEncounterRepository is an OPTIONAL capability of the host's
// [EncounterRepository], found by type assertion on Config.Encounters
// (rpg-project#879 S12a). It is not a Config field and not a method on
// EncounterRepository because a new required method is a silent runtime break
// for a host that has already adopted this package (the Config rule in
// session.go): a host without it gets today's two plain writes, unchanged.
//
// THE VERSION IS THE STORE'S. It is a counter the store keeps beside the
// encounter and increments on every save that lands, whatever the save
// carried. 0 means no checkpoint exists yet (a run Launch is about to write
// for the first time). The session never computes one; it reads it at the
// load and hands it back at the save.
//
// WHY A VERSION, AND NOT THE RECORD'S NextSeq. The record's next sequence is
// the only monotonic number in the blob, but a verb that appends no beat (a
// sneak declared while already sneaking commits a world it did not change)
// does not advance it. Two writers can therefore load the same NextSeq, both
// save, and neither would notice the other. The store's version moves on every
// save, so it cannot be fooled that way.
//
// WHY A FENCE AT ALL, WHEN THE LOCKER EXCLUDES. A lock held in a shared store
// is a lease, and a lease can expire while a verb is still running (a long
// raise, a paused process). Once it has, a second process can take the lock,
// load, and save; the first then wakes and saves over it. The fence is what
// keeps that late writer out: its save names the version it loaded, the store
// has moved past it, and the save fails closed with [ErrStaleCheckpoint].
//
// A host that wraps its repository (instrumentation, a cache) must forward
// these methods, or the wrapper hides the capability and the session falls
// back to plain writes.
type FencedEncounterRepository interface {
	// GetFencedEncounter returns the stored encounter and the checkpoint
	// version it was saved at, read together so the version describes exactly
	// the data returned. An absent encounter is an ErrNotFound error, as for
	// GetEncounter; success with no data violates the contract.
	GetFencedEncounter(ctx context.Context, in *GetFencedEncounterInput) (*GetFencedEncounterOutput, error)

	// SaveFenced writes the encounter, and the session record when one is
	// given, IN ONE TRANSACTION, only if the store's checkpoint version still
	// equals in.Expected; it then increments the version and returns the new
	// one. When the version has moved it writes nothing and returns an error
	// satisfying errors.Is(err, ErrStaleCheckpoint). Any error means nothing
	// landed: neither blob, and no new version.
	SaveFenced(ctx context.Context, in *SaveFencedInput) (*SaveFencedOutput, error)
}

// GetFencedEncounterInput names the encounter to read.
type GetFencedEncounterInput struct {
	// Encounter is the encounter's storage id.
	Encounter string
}

// GetFencedEncounterOutput is one stored encounter and the checkpoint version
// it was saved at.
type GetFencedEncounterOutput struct {
	// Data is the stored encounter. Never nil on success.
	Data *encounter.EncounterData

	// Version is the store's checkpoint version for this encounter: the
	// number of saves that have landed on it. 0 means none has.
	Version uint64
}

// SaveFencedInput is one verb's save: the world, the session record when the
// verb changed it, and the version the verb loaded.
type SaveFencedInput struct {
	// Encounter is the encounter's storage id.
	Encounter string

	// Data is the encounter to write. Never nil.
	Data *encounter.EncounterData

	// Session is the session record to write in the same transaction, or nil
	// when this verb did not change it (a verb that changed only the world
	// writes only the world).
	Session *SessionData

	// Expected is the checkpoint version this verb loaded (0 for a run Launch
	// writes for the first time). The save lands only if the store still
	// holds it.
	Expected uint64
}

// SaveFencedOutput is the version a landed save left the store at.
type SaveFencedOutput struct {
	// Version is the store's new checkpoint version: Expected plus one.
	Version uint64
}

// checkpointFence is the version a write verb loaded, when the host's
// repository is fenced. held false means UNKNOWN: the repository is plain and
// the save is the two unfenced writes it has always been. held true with a
// version of 0 means a checkpoint the store does not have yet.
type checkpointFence struct {
	held    bool
	version uint64
}

// fetchFencedWorld is [Manager.fetchWorld] through the fenced capability: the
// stored world and the checkpoint version it was saved at.
func (m *Manager) fetchFencedWorld(ctx context.Context, encID string) (*encounter.EncounterData, checkpointFence, error) {
	out, err := m.fenced.GetFencedEncounter(ctx, &GetFencedEncounterInput{Encounter: encID})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, checkpointFence{}, fmt.Errorf("%q: %w", encID, ErrNoEncounter)
		}
		return nil, checkpointFence{}, err
	}
	if out == nil || out.Data == nil {
		return nil, checkpointFence{}, fmt.Errorf(
			"%q: GetFencedEncounter reported success with no data: %w", encID, ErrBadRepository)
	}
	return out.Data, checkpointFence{held: true, version: out.Version}, nil
}

// persistFenced is [Manager.persist] over a fenced repository: the world and,
// when touched, the session record in one fenced save. Any failure means
// neither landed, so the report names both as failed; a stale fence unwraps
// to [ErrStaleCheckpoint]. A landed save moves the scope's fence to the new
// version, so a later save on the same scope is fenced on what it wrote.
func (m *Manager) persistFenced(
	ctx context.Context, scope *writeScope, data *encounter.EncounterData, report SaveReport,
) (SaveReport, error) {
	in := &SaveFencedInput{Encounter: scope.encounter, Data: data, Expected: scope.fence.version}
	aggregates := []string{"encounter:" + scope.encounter}
	if scope.touched {
		in.Session = scope.data
		aggregates = append(aggregates, "session:"+scope.session)
	}
	out, err := m.fenced.SaveFenced(ctx, in)
	if err == nil && (out == nil || out.Version == in.Expected) {
		err = fmt.Errorf("SaveFenced reported success without advancing the version: %w", ErrBadRepository)
	}
	if err != nil {
		report.Failed = append(report.Failed, aggregates...)
		return report, &SaveError{Report: report, Err: fmt.Errorf("saving world: %w", err)}
	}
	scope.fence.version = out.Version
	report.Written = append(report.Written, aggregates...)
	return report, nil
}
