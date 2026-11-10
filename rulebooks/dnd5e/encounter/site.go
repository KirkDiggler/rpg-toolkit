// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// site.go is THE SITE IS CONTENT THE RECORD REFERENCES (rpg-project#879 S5,
// design §Where the instance lives, R3/R4).
//
// A [Site] is the authored field compiled once: the floor, its regions and
// partitions, the walls, the props and the placed footprints, the
// concealments, the door GEOMETRY and the crossings each door's rectangle lies
// across, and the raw answer to what a cell sees for a given opacity. Every
// one of those depends only on the content, so one compiled site serves every
// game on that content, in this process, and a checkpoint names it by its
// [SiteHash] instead of carrying it.
//
// # Site and instance
//
//	| site (shared, keyed by hash)                   | instance (one game)                              |
//	|------------------------------------------------|--------------------------------------------------|
//	| regions, scenery, sealed, walls, segments      | members, clocks, journal, perception             |
//	| props, placed footprints, structural layout    | door STATE (open, shut, locked) and its lock     |
//	| concealments, intel, factions, exits, start    | the holdings reader a placed prop is asked by    |
//	| door geometry and openings, partitions         | the canvas members stand on                      |
//	| placement index, cell centres                  | sight sweep keys, held-testimony decodes         |
//	| raw visible-from by sweep key (no hidden face) | visible-from entries under a concealment view    |
//	|                                                | behavior definitions and progress (on members)   |
//
// BEHAVIORS ARE NOT SITE DATA HERE. Authored behavior definitions are content,
// but they ride on the roster ([MemberData.Behaviors]), not on the field, so
// they are not in the site's hash input and a compiled program cannot be
// shared through it; a creature's progress is instance state either way. A
// definition memo keyed by its own bytes is the follow-up, not this file's.
//
// # The laws
//
// A SITE IS READ-ONLY AFTER COMPILE. Its field's maps and slices are written
// by [CompileSite] and never after; an instance runs on a shallow copy of the
// site's field ([Site.instanceField]) whose own pointers — the holdings
// reader, the door records, the pass memo, the blocking mask — are the
// instance's. The two mutable parts are memos whose every entry is a pure
// function of the content and its key, each behind its own lock: the
// partition pass ([partitionMemo]) and the raw sight answers ([siteSight]).
// Two games racing to fill one entry write the same answer.
//
// THE HASH IS OF THE AUTHORED BYTES THE ENCOUNTER IS BUILT FROM: SHA-256 of
// the JSON encoding of [SiteData] — [SiteDataVersion], the field exactly as
// [FieldData] writes it, and every door's geometry in id order with its state
// written as closed and no lock ([DoorData]). A door's state and lock are the
// game's, so two games that start with a door in different states share a
// site. The same content compiled from a [FieldInput] or read back from its
// [SiteData] hashes the same, because both are written through the one
// [fieldDataFrom].

// SiteHash is the content hash of a compiled site: lower-case hex SHA-256 of
// its [SiteData] JSON. Opaque to every reader; equal hashes are equal content.
type SiteHash string

// SiteDataVersion names the shape [SiteData] is hashed and stored in. BUMP IT
// when what a site compiles to changes for the same bytes, so a stored site
// piece written by an older build is refused rather than reinterpreted.
const SiteDataVersion = "site/1"

// defaultSiteEntries is how many sites [NewMemorySites] keeps by default, and
// defaultSiteSightEntries how many raw visible-from answers one site keeps.
const (
	defaultSiteEntries      = 16
	defaultSiteSightEntries = 4 * visibleCacheBound
)

// ErrUnknownSite is a site hash the provider does not hold and was given no
// source to compile. A checkpoint naming it cannot be loaded on this process
// until a provider that holds the content is supplied: it fails closed.
var ErrUnknownSite = errors.New("encounter: unknown site")

// SiteData is the persistent representation of a [Site]: the authored field
// and the door geometry, the bytes its [SiteHash] is taken of. A host that
// stores the site piece apart from the checkpoint stores this, keyed by the
// hash, and hands it back through [SiteInput.Data] on a cold process.
type SiteData struct {
	// Version is [SiteDataVersion]; any other is refused.
	Version string `json:"version"`
	// Field is the authored field, exactly as it compiled.
	Field FieldData `json:"field"`
	// Doors is every door's geometry in id order, each written closed with no
	// lock: the state is the game's, carried on [EncounterData.Doors].
	Doors []DoorData `json:"doors,omitempty"`
}

// Site is one compiled site, shared by every encounter built on its content.
// Read-only after [CompileSite]; see this file's laws.
type Site struct {
	hash  SiteHash
	field *field
	// doors is the door geometry, every one closed, in id order — the
	// [SiteData.Doors] the hash was taken of and every instance's doors are
	// checked against.
	doors []DoorData
	sight *siteSight
}

// Hash is the site's content hash.
func (s *Site) Hash() SiteHash { return s.hash }

// Data is the site's persistent form, freshly built: what a host stores as
// the site piece, and what [CompileSite] reads back to the same hash.
func (s *Site) Data() SiteData {
	doors := make([]DoorData, len(s.doors))
	for i, d := range s.doors {
		doors[i] = cloneDoorData(d)
	}
	return SiteData{Version: SiteDataVersion, Field: fieldDataFrom(s.field), Doors: doors}
}

// CompileSiteInput names the authored source of a site: exactly one of Field
// (the construction form, as [SetupInput.Field]) or Data (the stored form).
type CompileSiteInput struct {
	Field *FieldInput
	Data  *SiteData
	// SightEntries bounds the site's raw visible-from answers; zero is the
	// default ([defaultSiteSightEntries]).
	SightEntries int
}

// CompileSiteOutput is the compiled site.
type CompileSiteOutput struct {
	Site *Site
}

// CompileSite validates and compiles an authored site: every rule about what a
// field may be that depends on the field alone — [compileField], the doors'
// geometry ([validateDoorInputs]), the doors a concealment names, the records'
// targets — then measures each door's opening and takes the hash.
//
// From Field the partitions are derived now, so a field that cannot be divided
// is refused where it is authored; from Data they are derived when first
// asked (rpg-project#762 T13), since the bytes came from a field that was.
//
// Errors carry no verb prefix (compileField's rule): the seam that asked wraps
// its own.
func CompileSite(in *CompileSiteInput) (*CompileSiteOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	var source FieldInput
	switch {
	case in.Field != nil && in.Data != nil:
		return nil, fmt.Errorf("a site is compiled from a field or from its data, not both: %w", ErrNoField)
	case in.Field != nil:
		source = *in.Field
	case in.Data != nil:
		if in.Data.Version != SiteDataVersion {
			return nil, fmt.Errorf("site data is version %q and this build reads %q: %w",
				in.Data.Version, SiteDataVersion, ErrInvalidData)
		}
		var err error
		source, err = fieldInputFrom(in.Data.Field)
		if err != nil {
			return nil, err
		}
		source.Doors, err = convertDoorDataToDoorInput(in.Data.Doors)
		if err != nil {
			return nil, err
		}
		for _, d := range in.Data.Doors {
			if DoorStateKind(d.State) != DoorClosed {
				return nil, fmt.Errorf("site door %q is written %q, and a site's doors are geometry, written closed: %w",
					d.ID, d.State, ErrInvalidData)
			}
		}
	default:
		return nil, fmt.Errorf("a site names no source: %w", ErrNoField)
	}

	f, err := compileField(source)
	if err != nil {
		return nil, err
	}
	if err = validateDoorInputs(f, source.Doors); err != nil {
		return nil, err
	}
	if in.Field != nil {
		if err = f.derivePartitions(source.Doors); err != nil {
			return nil, err
		}
	} else {
		f.deferPartitions(source.Doors)
	}
	if err = validateConcealmentDoors(f, source.Doors); err != nil {
		return nil, err
	}
	if err = validateIntelTargets(f); err != nil {
		return nil, err
	}

	closed := closedDoors(source.Doors)
	openings := make(map[DoorID][]DoorEdge, len(closed))
	doors := make([]DoorData, 0, len(closed))
	for _, d := range closed {
		openings[d.id] = f.measureDoorOpening(d)
		doors = append(doors, doorDataFrom(d))
	}
	f.doorOpenings = openings
	f.openingsAreSite = true

	raw, err := json.Marshal(SiteData{Version: SiteDataVersion, Field: fieldDataFrom(f), Doors: doors})
	if err != nil {
		return nil, fmt.Errorf("site data does not encode: %w: %w", ErrInvalidData, err)
	}
	sum := sha256.Sum256(raw)
	bound := in.SightEntries
	if bound <= 0 {
		bound = defaultSiteSightEntries
	}
	return &CompileSiteOutput{Site: &Site{
		hash:  SiteHash(hex.EncodeToString(sum[:])),
		field: f,
		doors: doors,
		sight: &siteSight{bound: bound},
	}}, nil
}

// instanceField is the field one encounter runs on: a shallow copy of the
// site's, sharing every compiled map and slice read-only, with the pointers a
// game writes — the holdings reader, the door records and their blocking
// mask, the sight pass memo — its own and empty.
func (s *Site) instanceField() *field {
	f := *s.field
	f.holdings = nil
	f.sightPass = nil
	f.doorFootprints = nil
	f.blockingDoors, f.blockingMask = nil, nil
	return &f
}

// checkDoors refuses an instance's doors whose geometry is not the site's: a
// checkpoint whose doors stand somewhere its site does not put them was
// written against other content.
func (s *Site) checkDoors(doors []DoorInput) error {
	got := make([]DoorData, 0, len(doors))
	for _, d := range closedDoors(doors) {
		got = append(got, doorDataFrom(d))
	}
	if !reflect.DeepEqual(got, s.doors) {
		return fmt.Errorf("the doors' geometry is not site %s's: %w", s.hash, ErrInvalidData)
	}
	return nil
}

// cloneDoorData deep-copies one door's data, so [Site.Data] never hands out
// the site's own slices.
func cloneDoorData(d DoorData) DoorData {
	out := d
	out.Edges = append([]EdgeData(nil), d.Edges...)
	if d.Placement != nil {
		p := *d.Placement
		out.Placement = &p
	}
	if d.Lock != nil {
		l := *d.Lock
		out.Lock = &l
	}
	return out
}

// siteSight is THE RAW LAYER OF WHAT A CELL SEES (rpg-project#879 S5, F23):
// the visible-from bits by sweep key for every key whose observer has no face
// hidden from it — no unfound concealed door or prop. The key fingerprints
// the origin, the range and the opacity (every door's blocking answer, every
// movable footprint, every sight-blocking entity, every obscuring area), so an
// entry is a pure function of the site and its key, whichever game asked;
// a door opened in one game is a different key, never a changed entry.
// Bounded: at the bound the whole map is dropped and refilled by use.
//
// Entries are shared read-only: a reader never writes into the bits it gets.
type siteSight struct {
	mu      sync.RWMutex
	bound   int
	entries map[sightSweepKey]visibleBits
}

func (s *siteSight) get(key sightSweepKey, words int) (visibleBits, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bits, ok := s.entries[key]
	if !ok || len(bits) != words {
		return nil, false
	}
	return bits, true
}

func (s *siteSight) put(key sightSweepKey, bits visibleBits) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) >= s.bound {
		s.entries = nil
	}
	if s.entries == nil {
		s.entries = make(map[sightSweepKey]visibleBits)
	}
	s.entries[key] = bits
}

func (s *siteSight) len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// SiteProvider resolves a site by its hash, compiling it from a source when
// it is given one. Host-injected ([SetupInput.Sites], [LoadEncounterInput.Sites]);
// nil there is [DefaultSites]. Never (nil, nil): a site it cannot resolve is
// [ErrUnknownSite].
type SiteProvider interface {
	Site(ctx context.Context, in *SiteInput) (*SiteOutput, error)
}

// SiteInput asks for one site: by Hash, with Field or Data as the authored
// source to compile on a miss. With a source and no hash the source's own
// hash is the answer; with both, a source that hashes otherwise is refused.
type SiteInput struct {
	Hash  SiteHash
	Field *FieldInput
	Data  *SiteData
}

// SiteOutput is the resolved site, never nil on success.
type SiteOutput struct {
	Site *Site
}

// MemorySitesConfig bounds a [MemorySites].
type MemorySitesConfig struct {
	// Entries is how many sites are held; zero is [defaultSiteEntries].
	Entries int
	// SightEntries bounds each compiled site's raw sight answers; zero is
	// [defaultSiteSightEntries].
	SightEntries int
}

// MemorySites is THE IN-PROCESS DEFAULT PROVIDER: a bounded map of compiled
// sites keyed by hash. On a hash it holds, the site is returned and marked
// recently used; on a miss with a source, the source is compiled and kept;
// on a miss without one, [ErrUnknownSite]. At the bound the LEAST RECENTLY
// USED site is evicted — an encounter already running on it keeps its pointer
// and runs on; the next load of that hash needs a source again. Safe for
// concurrent use.
type MemorySites struct {
	mu           sync.Mutex
	entries      int
	sightEntries int
	byHash       map[SiteHash]*list.Element
	order        *list.List // front is most recent; values are *Site
	// compiles counts the sites compiled and kept, for the tests that pin
	// "one compile per content".
	compiles int
}

// NewMemorySites is an empty provider under cfg (nil is every default).
func NewMemorySites(cfg *MemorySitesConfig) *MemorySites {
	m := &MemorySites{
		entries:      defaultSiteEntries,
		sightEntries: defaultSiteSightEntries,
		byHash:       make(map[SiteHash]*list.Element),
		order:        list.New(),
	}
	if cfg != nil && cfg.Entries > 0 {
		m.entries = cfg.Entries
	}
	if cfg != nil && cfg.SightEntries > 0 {
		m.sightEntries = cfg.SightEntries
	}
	return m
}

var defaultSites = NewMemorySites(nil)

// DefaultSites is the process-wide [MemorySites] every encounter built or
// loaded without a provider uses, so a host and a test that inject nothing
// keep working within one process.
func DefaultSites() *MemorySites { return defaultSites }

// Site resolves in; see [MemorySites].
func (m *MemorySites) Site(_ context.Context, in *SiteInput) (*SiteOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.Hash != "" {
		if site, ok := m.lookup(in.Hash); ok {
			return &SiteOutput{Site: site}, nil
		}
	}
	if in.Field == nil && in.Data == nil {
		if in.Hash == "" {
			return nil, fmt.Errorf("a site asked for by neither hash nor source: %w", ErrNilInput)
		}
		return nil, fmt.Errorf("site %s: %w", in.Hash, ErrUnknownSite)
	}
	out, err := CompileSite(&CompileSiteInput{Field: in.Field, Data: in.Data, SightEntries: m.sightEntries})
	if err != nil {
		return nil, err
	}
	if in.Hash != "" && out.Site.hash != in.Hash {
		return nil, fmt.Errorf("the source hashes to site %s, not the %s asked for: %w",
			out.Site.hash, in.Hash, ErrInvalidData)
	}
	return &SiteOutput{Site: m.keep(out.Site)}, nil
}

func (m *MemorySites) lookup(hash SiteHash) (*Site, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	el, ok := m.byHash[hash]
	if !ok {
		return nil, false
	}
	m.order.MoveToFront(el)
	return el.Value.(*Site), true
}

// keep stores a freshly compiled site, or returns the one already held under
// its hash — the same content compiled twice is one site.
func (m *MemorySites) keep(site *Site) *Site {
	m.mu.Lock()
	defer m.mu.Unlock()
	if el, ok := m.byHash[site.hash]; ok {
		m.order.MoveToFront(el)
		return el.Value.(*Site)
	}
	m.byHash[site.hash] = m.order.PushFront(site)
	m.compiles++
	for m.order.Len() > m.entries {
		oldest := m.order.Back()
		m.order.Remove(oldest)
		delete(m.byHash, oldest.Value.(*Site).hash)
	}
	return site
}

// resolveSite is the one call both construction seams make: the injected
// provider or the default, and an answer that is never a nil site.
func resolveSite(sites SiteProvider, in *SiteInput) (*Site, error) {
	if sites == nil {
		sites = defaultSites
	}
	out, err := sites.Site(context.Background(), in)
	if err != nil {
		return nil, err
	}
	if out == nil || out.Site == nil {
		return nil, fmt.Errorf("the site provider answered no site: %w", ErrUnknownSite)
	}
	return out.Site, nil
}

// siteHash is the hash ToData writes: the site's, or empty on an Encounter
// built by hand in a test, which no load can resolve.
func (e *Encounter) siteHash() SiteHash {
	if e.site == nil {
		return ""
	}
	return e.site.hash
}

// Site is the compiled site this encounter runs on, shared with every other
// encounter on the same content: what a host stores as the site piece
// ([Site.Data]) beside the checkpoint that names it by [Site.Hash].
func (e *Encounter) Site() *Site { return e.site }
