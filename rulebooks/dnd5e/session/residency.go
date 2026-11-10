// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"container/list"
	"context"
	"maps"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// residency.go is THE RESIDENT COPY (rpg-project#879 S6, R6,
// rpg-project#927): a write verb's loaded encounter, kept in memory after the
// verb's final save landed, so the session's next write verb skips the
// fenced read and the load.
//
// THE COPY IS A CACHE; THE STORE IS THE AUTHORITY. A kept copy carries the
// checkpoint version its own save left the store at. The next verb reads the
// store's version (one GET, [CheckpointVersionReader]); on a match it rebinds
// the copy to its own capabilities ([encounter.Encounter.Rebind]) and runs on
// it, with the fence's version, record baseline and piece marks the copy
// carries; on a miss it drops the copy and loads as it always has. One key is
// enough: the version moves on every save that lands, whatever wrote it, and
// the copy that holds version V is the copy that wrote V.
//
// TAKE ON OPEN, KEEP ON LAND. A write verb takes the copy out of the
// residency before it runs, so a copy in use is in no map, and only the
// guard's release puts one back, and only a copy staged by the verb's final
// commit after its save landed and its batches were handed to the streams.
// So every exit that does not reach that point drops the copy by structure:
// a stale save, a refused verb, a commit error, a canceled context, a
// continuation that failed after a pace commit, a panic in a host
// collaborator, a run that closed. Nothing has to remember to drop it.
//
// READS DO NOT USE THE COPY in this cut: they load from the store under the
// read capabilities, as before.

// CheckpointVersionReader is an OPTIONAL capability of the host's
// [EncounterRepository], found by type assertion on Config.Encounters like
// the fence (rpg-project#879 S6). A host that has it, beside
// [FencedEncounterRepository], lets the session keep a write verb's encounter
// resident between verbs ([Config.Residency]); a host without it never uses
// the residency, and every verb loads, as before.
//
// A host that wraps its repository must forward it, or the wrapper hides it
// and every verb loads.
type CheckpointVersionReader interface {
	// GetCheckpointVersion returns the store's checkpoint version for the
	// encounter: the number of fenced saves that have landed on it, the same
	// number [FencedEncounterRepository.GetFencedEncounter] returns beside
	// the data. One read of one key. An absent encounter may answer 0 or an
	// error; either makes the session load from the store, which then
	// answers ErrNoEncounter as it always has.
	GetCheckpointVersion(ctx context.Context, encounter string) (uint64, error)
}

// Residency holds the sessions' resident copies between verbs
// ([Config.Residency]). The session calls it only under the session's guard,
// so two calls for one session never overlap; calls for different sessions
// may, so an implementation must be safe for concurrent use.
type Residency interface {
	// Take removes the session's copy and returns it, or nil when none is
	// kept. A taken copy is the caller's until it is kept again or dropped.
	Take(session string) *Resident

	// Keep holds r as the session's copy, replacing any it holds. A bounded
	// implementation may evict any copy, this one included: an evicted copy
	// costs the next verb one load and nothing else.
	Keep(session string, r *Resident)
}

// Resident is one session's kept copy: the encounter its last write verb
// left, and what that verb's landed save left the fence at. Opaque: a host
// holds it and hands it back, and never reaches the encounter through it.
type Resident struct {
	// encounter is the storage id of the encounter the copy is of; a
	// session record naming another one never uses it.
	encounter string
	enc       *encounter.Encounter
	// fence is the version, record baseline and piece marks the copy's
	// own save left the store at. saved is false: a verb that takes the
	// copy has not saved yet.
	fence checkpointFence
	// kinds is every member's kind the verb's standing knew, placed and
	// waiting, for the next verb's standing (a load reads it off the stored
	// world, which a kept copy skips).
	kinds map[string]encounter.MemberKind
	// bytes is the estimated memory the copy holds ([residentBytes]).
	bytes int64
}

// Version is the checkpoint version the copy was saved at.
func (r *Resident) Version() uint64 { return r.fence.version }

// Bytes is the estimated memory the copy holds: a fixed part for the
// instance and a part per record entry, since the record is the part of a
// launched run that grows without bound ([residentBytes]).
func (r *Resident) Bytes() int64 { return r.bytes }

// The estimate behind [Resident.Bytes], measured on the benchmark keep
// (rpg-project#927 map §5): about 1.6 MB per instance with the site shared,
// and about 0.9 KB per record entry.
const (
	residentFixedBytes = 1_600_000
	residentEntryBytes = 900
)

// residentBytes estimates a copy's memory from its record's next seq.
func residentBytes(next uint64) int64 {
	return residentFixedBytes + residentEntryBytes*int64(next) //nolint:gosec // a seq is far below 2^63
}

// Defaults for [NewResidency].
const (
	// DefaultResidentEntries is how many copies the default residency keeps.
	DefaultResidentEntries = 64

	// DefaultResidentMaxBytes is how many estimated bytes the default
	// residency keeps across its copies: 512 MiB.
	DefaultResidentMaxBytes = 512 << 20
)

// ResidencyConfig bounds a [MemoryResidency].
type ResidencyConfig struct {
	// MaxEntries is how many copies it keeps; the oldest kept is evicted
	// first. Zero is [DefaultResidentEntries]; negative keeps none.
	MaxEntries int

	// MaxBytes is how many estimated bytes ([Resident.Bytes]) it keeps
	// across its copies; the oldest kept is evicted first, and a copy larger
	// than the bound is not kept. Zero is [DefaultResidentMaxBytes];
	// negative keeps none.
	MaxBytes int64
}

// MemoryResidency is the in-process [Residency]: copies by session, bounded by
// count and by estimated bytes, the least recently kept evicted first. The
// default when Config.Residency is nil.
type MemoryResidency struct {
	mu         sync.Mutex
	maxEntries int
	maxBytes   int64
	bytes      int64
	order      *list.List // front is the most recently kept
	bySession  map[string]*list.Element
}

type residentEntry struct {
	session  string
	resident *Resident
}

// NewResidency returns a [MemoryResidency] bounded by cfg; nil is the
// defaults.
func NewResidency(cfg *ResidencyConfig) *MemoryResidency {
	r := &MemoryResidency{
		maxEntries: DefaultResidentEntries, maxBytes: DefaultResidentMaxBytes,
		order: list.New(), bySession: map[string]*list.Element{},
	}
	if cfg != nil {
		if cfg.MaxEntries != 0 {
			r.maxEntries = cfg.MaxEntries
		}
		if cfg.MaxBytes != 0 {
			r.maxBytes = cfg.MaxBytes
		}
	}
	return r
}

// Take implements [Residency].
func (r *MemoryResidency) Take(session string) *Resident {
	r.mu.Lock()
	defer r.mu.Unlock()
	el, ok := r.bySession[session]
	if !ok {
		return nil
	}
	return r.remove(el)
}

// Keep implements [Residency].
func (r *MemoryResidency) Keep(session string, kept *Resident) {
	if kept == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if el, ok := r.bySession[session]; ok {
		r.remove(el)
	}
	if r.maxEntries <= 0 || r.maxBytes <= 0 || kept.bytes > r.maxBytes {
		return
	}
	r.bySession[session] = r.order.PushFront(&residentEntry{session: session, resident: kept})
	r.bytes += kept.bytes
	for r.order.Len() > r.maxEntries || r.bytes > r.maxBytes {
		r.remove(r.order.Back())
	}
}

// Len is how many copies are kept.
func (r *MemoryResidency) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.order.Len()
}

// Bytes is the estimated bytes of every kept copy.
func (r *MemoryResidency) Bytes() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bytes
}

func (r *MemoryResidency) remove(el *list.Element) *Resident {
	entry := r.order.Remove(el).(*residentEntry) //nolint:errcheck,forcetypeassert // the list holds only entries
	delete(r.bySession, entry.session)
	r.bytes -= entry.resident.bytes
	return entry.resident
}

// stagedResidents is each session's copy staged by its verb's final commit,
// waiting for the guard's release to keep it. Like [pendingWrites], it never
// outlives the guard: the release takes whatever is staged.
type stagedResidents struct {
	mu       sync.Mutex
	sessions map[string]*Resident
}

func (s *stagedResidents) put(session string, r *Resident) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = map[string]*Resident{}
	}
	s.sessions[session] = r
}

func (s *stagedResidents) take(session string) *Resident {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.sessions[session]
	delete(s.sessions, session)
	return r
}

// residentOn reports whether this Manager keeps copies: a fenced host with
// the version reader.
func (m *Manager) residentOn() bool { return m.versions != nil }

// takeResident takes the session's copy out of the residency, and returns it
// only when it is of the encounter the record names and the store's version
// is the one it was saved at. Anything else is dropped: the verb loads.
func (m *Manager) takeResident(ctx context.Context, data *SessionData) *Resident {
	r := m.residency.Take(data.ID)
	if r == nil || r.encounter != data.Encounter {
		return nil
	}
	version, err := m.versions.GetCheckpointVersion(ctx, data.Encounter)
	if err != nil || version != r.fence.version {
		return nil
	}
	return r
}

// openResident binds a taken copy to this verb's scope, as
// [Manager.loadGivenWorld] binds a loaded world: the standing from the kinds
// the copy kept, every capability bound to the scope, the encounter rebound
// to them. It reports false when the copy cannot be rebound, and the verb
// then loads.
func (m *Manager) openResident(ctx context.Context, data *SessionData, scope *writeScope, r *Resident) bool {
	standing := m.standingFor(ctx, data, r.kinds)
	scope.standing = standing
	if err := r.enc.Rebind(&encounter.RebindInput{Capabilities: m.writeCapabilities(ctx, scope)}); err != nil {
		return false
	}
	baseline, err := r.enc.NextStorySeq()
	if err != nil {
		return false
	}
	standing.live.enc = r.enc
	scope.enc = r.enc
	scope.baseline = baseline
	scope.fence = r.fence
	scope.fence.saved = false
	return true
}

// stageResident stages the scope's encounter as the session's copy, once the
// verb's final commit landed and its batches were handed to the streams.
// A run that closed stages nothing: no verb follows it.
func (m *Manager) stageResident(scope *writeScope) {
	if !m.residentOn() || !scope.fence.held {
		return
	}
	status, err := scope.enc.Status()
	if err != nil || !status.Open {
		return
	}
	fence := scope.fence
	fence.saved = false
	m.staged.put(scope.session, &Resident{
		encounter: scope.encounter,
		enc:       scope.enc,
		fence:     fence,
		kinds:     maps.Clone(scope.standing.kinds),
		bytes:     residentBytes(fence.recordNext),
	})
}

// keepStaged keeps the session's staged copy, at the guard's release. A copy
// whose record moved after its save (something appended past what landed)
// is not the store's and is dropped.
func (m *Manager) keepStaged(session string) {
	if !m.residentOn() {
		return
	}
	r := m.staged.take(session)
	if r == nil {
		return
	}
	next, err := r.enc.NextStorySeq()
	if err != nil || next != r.fence.recordNext {
		return
	}
	m.residency.Keep(session, r)
}
