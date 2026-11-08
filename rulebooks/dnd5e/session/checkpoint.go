// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
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
// EVERY PIECE RIDES THE ONE SAVE. Under the fence nothing a verb writes under
// the session's guard lands on its own: the sheets and seats it writes
// mid-verb are held ([pendingWrites]) and handed to SaveFenced beside the
// world and the session record. A late writer therefore lands no piece at
// all, and a verb that fails before its save leaves the store as it found it
// (the plain path's mid-verb sheet writes are durable on their own). A walk's
// spent movement is part of its commit, not written ahead of it.
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

	// SaveFenced writes every piece the input carries (the encounter, the
	// session record, the character sheets and the seats) IN ONE
	// TRANSACTION, only if the store's checkpoint version still equals
	// in.Expected; it then increments the version and returns the new one.
	// When the version has moved it writes nothing and returns an error
	// satisfying errors.Is(err, ErrStaleCheckpoint). Any error means nothing
	// landed: no piece, and no new version.
	//
	// The sheets and seats are the ones the host's CharacterRepository and
	// SeatRepository hold, under the same keys: a fenced verb never writes
	// them through those repositories, so the host writes them here.
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

	// Characters is every character sheet this verb changed, at its last
	// state, in id order; nil when it changed none. Each carries its own id,
	// as for SaveCharacter.
	Characters []*character.Data

	// Seats is every seat this verb wrote (a Launch seating its party, a
	// closing run clearing them), in character order; nil when it wrote none.
	// A seat with an empty Session is a cleared seat, as for SaveSeat.
	Seats []*SeatData

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

	// saved reports that this scope's fenced save landed. A sheet or seat
	// written after it has no save left to ride, so it is refused
	// ([errWrittenAfterTheSave]) rather than dropped at the guard's release.
	saved bool
}

// errWrittenAfterTheSave is a fenced verb writing a sheet or seat after its
// one fenced save already landed: the piece has no save to ride and would be
// dropped when the guard is released, so the write is refused loudly
// (rpg-toolkit#2332, a seat clear that ran after the save and never landed).
func errWrittenAfterTheSave(scope *writeScope, aggregate string) error {
	return &SaveError{
		Report: SaveReport{Failed: []string{aggregate}},
		Err: fmt.Errorf("%s written after session %q's fenced save landed: %w",
			aggregate, scope.session, ErrBadRepository),
	}
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

// persistFenced is [Manager.persist] over a fenced repository: the sheets and
// seats the verb deferred, the world and, when touched, the session record,
// in one fenced save. Any failure means none of them landed, so the report
// names them all as failed; a stale fence unwraps to [ErrStaleCheckpoint].
func (m *Manager) persistFenced(
	ctx context.Context, scope *writeScope, data *encounter.EncounterData, report SaveReport,
) (SaveReport, error) {
	in := &SaveFencedInput{Encounter: scope.encounter, Data: data, Expected: scope.fence.version}
	in.Characters, in.Seats = m.pending.take(scope.session)
	var aggregates []string
	for _, sheet := range in.Characters {
		aggregates = append(aggregates, "character:"+sheet.ID)
	}
	for _, seat := range in.Seats {
		aggregates = append(aggregates, "seat:"+seat.Character)
	}
	aggregates = append(aggregates, "encounter:"+scope.encounter)
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
	scope.fence.saved = true
	report.Written = append(report.Written, aggregates...)
	return report, nil
}

// pendingWrites holds the sheets and seats fenced verbs have written but not
// yet saved: under the fence they ride the verb's one fenced save instead of
// landing on their own repositories mid-verb (a late writer would otherwise
// land them before its world save is refused).
//
// IT IS THE REPOSITORY'S STAND-IN FOR THE VERB THAT WROTE. Every sheet and
// seat read consults it first ([sheetStore.load], [Manager.seatOf]), so a
// verb reads its own writes exactly as it did when they were written through;
// no read site needs a scope to see them. An entry belongs to the session
// whose guard its writer holds, and no other verb can be reading that
// character meanwhile: a seated sheet is under its session's guard, and a seat
// changes under both guards.
//
// IT NEVER OUTLIVES THE GUARD. The fenced save takes the session's entries,
// and releasing the session's guard drops whatever is left (a verb that
// failed before its save), so nothing a verb did not save is visible to the
// next. It is the only state a Manager holds, and only while a verb holds the
// guard. Nil on a Manager whose repository is plain.
type pendingWrites struct {
	mu     sync.Mutex
	sheets map[string]pendingSheet
	seats  map[string]pendingSeat
}

type pendingSheet struct {
	session string
	data    *character.Data
}

type pendingSeat struct {
	session string
	data    SeatData
}

func newPendingWrites() *pendingWrites {
	return &pendingWrites{sheets: map[string]pendingSheet{}, seats: map[string]pendingSeat{}}
}

// putSheet keeps a copy of one written sheet for the session's fenced save.
// A copy, because the repository it stands in for would not see the caller's
// later edits either.
func (p *pendingWrites) putSheet(session string, data *character.Data) error {
	copied, err := copySheet(data)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sheets[data.ID] = pendingSheet{session: session, data: copied}
	return nil
}

// sheet returns a copy of the pending sheet for a character, if one is held.
func (p *pendingWrites) sheet(id string) (*character.Data, bool, error) {
	p.mu.Lock()
	held, ok := p.sheets[id]
	p.mu.Unlock()
	if !ok {
		return nil, false, nil
	}
	copied, err := copySheet(held.data)
	return copied, true, err
}

// putSeat keeps one written seat for the session's fenced save.
func (p *pendingWrites) putSeat(session string, data SeatData) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seats[data.Character] = pendingSeat{session: session, data: data}
}

// seat returns the pending seat for a character, if one is held.
func (p *pendingWrites) seat(character string) (SeatData, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	held, ok := p.seats[character]
	return held.data, ok
}

// take removes and returns the session's pending sheets and seats, each in
// id order, for its fenced save.
func (p *pendingWrites) take(session string) ([]*character.Data, []*SeatData) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var sheets []*character.Data
	for id, held := range p.sheets {
		if held.session == session {
			sheets = append(sheets, held.data)
			delete(p.sheets, id)
		}
	}
	var seats []*SeatData
	for character, held := range p.seats {
		if held.session == session {
			seat := held.data
			seats = append(seats, &seat)
			delete(p.seats, character)
		}
	}
	sort.Slice(sheets, func(i, j int) bool { return sheets[i].ID < sheets[j].ID })
	sort.Slice(seats, func(i, j int) bool { return seats[i].Character < seats[j].Character })
	return sheets, seats
}

// drop forgets whatever the session's verb wrote and did not save.
func (p *pendingWrites) drop(session string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, held := range p.sheets {
		if held.session == session {
			delete(p.sheets, id)
		}
	}
	for character, held := range p.seats {
		if held.session == session {
			delete(p.seats, character)
		}
	}
}

// copySheet is a deep copy through the sheet's persisted form, the form the
// repository it stands in for would hold.
func copySheet(data *character.Data) (*character.Data, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("character %q: %w: %v", data.ID, ErrBadCharacter, err)
	}
	var out character.Data
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("character %q: %w: %v", data.ID, ErrBadCharacter, err)
	}
	return &out, nil
}

// saveSeatedSheetFenced is a seated character's sheet written by a verb that
// does not open its run (LevelUp): under the fence it still rides a fenced
// save of that run, so a level written by a writer whose lease ran out is
// refused with every other piece. The world is written back as it was read.
func (m *Manager) saveSeatedSheetFenced(ctx context.Context, session string, record *character.Data) (SaveReport, error) {
	data, err := m.loadSessionData(ctx, session)
	if err != nil {
		return SaveReport{}, err
	}
	world, fence, err := m.fetchFencedWorld(ctx, data.Encounter)
	if err != nil {
		return SaveReport{}, err
	}
	scope := &writeScope{session: session, encounter: data.Encounter, data: data, fence: fence}
	if err := m.pending.putSheet(session, record); err != nil {
		return SaveReport{}, err
	}
	return m.persistFenced(ctx, scope, world, SaveReport{})
}
