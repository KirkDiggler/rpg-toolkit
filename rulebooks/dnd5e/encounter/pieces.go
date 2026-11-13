// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// pieces.go splits a checkpoint's rest into the pieces a store writes one by
// one, and joins them back (rpg-project#897, leg b). The grouping is
// rpg-project#770's shapes at the grain F26 measured: a head written every
// save, the roster one piece per member, knowledge one piece per observer,
// the journal one piece. A piece is canonical JSON and a mark, a 64-bit
// digest of those bytes; the host keeps the marks it last wrote and hands
// them back, and only the pieces whose mark moved come out. Pure: nothing
// here reads or touches a running encounter, and nothing here knows a store.

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"maps"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// PieceKey names one piece of an encounter's rest. The four fixed keys are
// [PieceHead], [PieceRoster], [PieceKnowledge] and [PieceJournal]; a member's
// piece is [MemberPiece] ("m:<member>") and an observer's is [ObserverPiece]
// ("o:<observer>"). [PieceKey.Shape] says which of #770's shapes a key is.
type PieceKey string

const (
	// PieceHead is the clocks (the world clock, the bubbles, the landed
	// beats), the log head, the site hash, the endings, the retention and the
	// perception subjects' version: everything small and everything that
	// moves on every verb. [SplitPieces] returns it on every save, so a
	// store can treat it as the checkpoint's existence sentinel.
	PieceHead PieceKey = "head"
	// PieceRoster is the roster's shared part: the doors, the sight areas,
	// the interactions, the reserve, the outcome, the pause and every member
	// who ever joined. The members themselves are [MemberPiece]s.
	PieceRoster PieceKey = "roster"
	// PieceKnowledge is knowledge's shared part: discovery and the sneak
	// verdicts. Each observer's holdings and sight sweep are an
	// [ObserverPiece].
	PieceKnowledge PieceKey = "knowledge"
	// PieceJournal is the journal, whole. It is append-only, so its mark
	// moves only when a fact is appended. It always exists: a run with no
	// journal is the JSON null, so a store that lost it is told by name.
	PieceJournal PieceKey = "journal"
)

const (
	memberPiecePrefix   = "m:"
	observerPiecePrefix = "o:"
)

// MemberPiece is the key of one member's piece: "m:" and the member id.
func MemberPiece(id MemberID) PieceKey { return PieceKey(memberPiecePrefix + string(id)) }

// ObserverPiece is the key of one observer's piece: "o:" and the observer as
// perception keys it (a member observes as "member|<id>"). The piece holds
// the observer's holdings and, for a member, its sight sweep key.
func ObserverPiece(observer core.EntityID) PieceKey {
	return PieceKey(observerPiecePrefix + string(observer))
}

// PieceShape is which of rpg-project#770's shapes a piece belongs to. A store
// routes by it: the head and the journal are one value each, the roster and
// knowledge one value per piece of that shape.
type PieceShape string

const (
	// ShapeHead is [PieceHead].
	ShapeHead PieceShape = "head"
	// ShapeRoster is [PieceRoster] and every [MemberPiece].
	ShapeRoster PieceShape = "roster"
	// ShapeKnowledge is [PieceKnowledge] and every [ObserverPiece].
	ShapeKnowledge PieceShape = "knowledge"
	// ShapeJournal is [PieceJournal].
	ShapeJournal PieceShape = "journal"
)

// Shape is the shape this key belongs to, or the empty shape for a key that
// names no piece.
func (k PieceKey) Shape() PieceShape {
	switch {
	case k == PieceHead:
		return ShapeHead
	case k == PieceJournal:
		return ShapeJournal
	case k == PieceRoster, len(k) > len(memberPiecePrefix) && strings.HasPrefix(string(k), memberPiecePrefix):
		return ShapeRoster
	case k == PieceKnowledge, len(k) > len(observerPiecePrefix) && strings.HasPrefix(string(k), observerPiecePrefix):
		return ShapeKnowledge
	}
	return ""
}

// Mark is a piece's 64-bit FNV-1a digest of its bytes. Opaque to the store:
// equal marks mean the piece need not be written again, and nothing else.
type Mark uint64

// markOf is the mark of a piece's bytes. FNV-1a because it is in the
// standard library, unseeded (the same bytes mark the same in every process
// and every build, which a stored mark needs; hash/maphash is seeded per
// process) and costs nothing next to the encode it digests.
func markOf(raw []byte) Mark {
	h := fnv.New64a()
	_, _ = h.Write(raw) // a hash.Hash never errs
	return Mark(h.Sum64())
}

// Piece is one piece of the rest: its canonical JSON and its mark.
type Piece struct {
	// Bytes is the piece's canonical JSON, what the store writes.
	Bytes []byte
	// Mark is [Piece.Bytes]'s digest, what the store keeps beside it.
	Mark Mark
}

// SplitPiecesInput is the input to [SplitPieces].
type SplitPiecesInput struct {
	// Data is the rest to split: [ToCheckpointOutput.Rest]. Its log must be
	// the head alone; a log carrying entries is refused with
	// [ErrRestCarriesRecord], because the record is the store's segments.
	Data *EncounterData
	// Held is the mark of every piece the store holds, as the last landed
	// save's [SplitPiecesOutput.Marks] (or the load) left them. Nil means
	// the store holds nothing it can vouch for (a Launch, or a host that
	// keeps no marks), and every piece comes out.
	Held map[PieceKey]Mark
}

// SplitPiecesOutput is the output of [SplitPieces].
type SplitPiecesOutput struct {
	// Pieces is every piece whose mark differs from Held, a piece Held does
	// not name, and the head always.
	Pieces map[PieceKey]Piece
	// Removed is every key Held names that the rest no longer has (a member
	// who left, an observer whose holdings emptied), sorted.
	Removed []PieceKey
	// Marks is the mark of every piece the rest has, what the host holds
	// once this save lands.
	Marks map[PieceKey]Mark
}

// headPiece is [PieceHead]'s shape: the fields of [EncounterData] under the
// same JSON names.
type headPiece struct {
	Clock              clock.TickData   `json:"clock"`
	Bubbles            []clock.TurnData `json:"bubbles,omitempty"`
	Landed             []WakeBeatData   `json:"landed,omitempty"`
	PerceptionSubjects uint32           `json:"perception_subjects,omitempty"`
	Log                record.LogData   `json:"log"`
	SiteHash           SiteHash         `json:"site_hash"`
	Endings            []EndingData     `json:"endings"`
	Retention          int              `json:"retention,omitempty"`
}

// rosterPiece is [PieceRoster]'s shape.
type rosterPiece struct {
	Outcome      *OutcomeData      `json:"outcome,omitempty"`
	Doors        []DoorData        `json:"doors,omitempty"`
	SightAreas   []SightAreaData   `json:"sight_areas,omitempty"`
	Interactions []InteractionData `json:"interactions,omitempty"`
	Sharing      Sharing           `json:"sharing,omitempty"`
	Reserve      []ReserveData     `json:"reserve,omitempty"`
	EverMembers  []MemberID        `json:"ever_members"`
	Pause        *PauseData        `json:"pause,omitempty"`
}

// knowledgePiece is [PieceKnowledge]'s shape.
type knowledgePiece struct {
	Discovery     map[MemberID]DiscoveryStateData            `json:"discovery,omitempty"`
	SneakVerdicts map[MemberID]map[MemberID]SneakVerdictData `json:"sneak_verdicts,omitempty"`
}

// observerPiece is an [ObserverPiece]'s shape: the observer's holdings and,
// for a member, its sight sweep key. One of the two is always present.
type observerPiece struct {
	Holdings map[intel.Subject]map[intel.Topic]intel.HoldingData `json:"holdings,omitempty"`
	Sweep    string                                              `json:"sweep,omitempty"`
}

// SplitPieces groups a checkpoint's rest into pieces, marks each, and returns
// the pieces the store does not hold as they are now (rpg-project#897 leg b).
// Pure: the rest is only read.
//
// The head comes out on every call, whatever Held says; every other piece
// only when its mark differs from Held's or Held does not name it. A key
// Held names that the rest no longer has comes out in Removed. The joined
// pieces are the rest: [JoinPieces] over every piece the store holds after
// writing these and dropping the removed ones encodes to the rest's bytes.
//
// Errors: [ErrNilInput] for a nil input or nil Data; [ErrRestCarriesRecord]
// when Data's log carries entries or audiences (the record is the store's
// segments, never a piece); an encode failure, wrapped.
func SplitPieces(in *SplitPiecesInput) (*SplitPiecesOutput, error) {
	if in == nil || in.Data == nil {
		return nil, fmt.Errorf("split pieces: %w", ErrNilInput)
	}
	d := in.Data
	if len(d.Log.Entries) > 0 || len(d.Log.Audiences) > 0 {
		return nil, fmt.Errorf("split pieces: %w", ErrRestCarriesRecord)
	}
	out := &SplitPiecesOutput{Pieces: map[PieceKey]Piece{}, Marks: map[PieceKey]Mark{}}
	add := func(key PieceKey, v any) error {
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("split pieces: %s: %w", key, err)
		}
		mark := markOf(raw)
		out.Marks[key] = mark
		if held, ok := in.Held[key]; key == PieceHead || in.Held == nil || !ok || held != mark {
			out.Pieces[key] = Piece{Bytes: raw, Mark: mark}
		}
		return nil
	}
	if err := add(PieceHead, headPiece{
		Clock: d.Clock, Bubbles: d.Bubbles, Landed: d.Landed, PerceptionSubjects: d.PerceptionSubjects,
		Log: d.Log, SiteHash: d.SiteHash, Endings: d.Endings, Retention: d.Retention,
	}); err != nil {
		return nil, err
	}
	if err := add(PieceRoster, rosterPiece{
		Outcome: d.Outcome, Doors: d.Doors, SightAreas: d.SightAreas, Interactions: d.Interactions,
		Sharing: d.Sharing, Reserve: d.Reserve, EverMembers: d.EverMembers, Pause: d.Pause,
	}); err != nil {
		return nil, err
	}
	if err := add(PieceKnowledge, knowledgePiece{Discovery: d.Discovery, SneakVerdicts: d.SneakVerdicts}); err != nil {
		return nil, err
	}
	if err := add(PieceJournal, d.Journal); err != nil {
		return nil, err
	}
	for i := range d.Members {
		if err := add(MemberPiece(d.Members[i].ID), &d.Members[i]); err != nil {
			return nil, err
		}
	}
	observers := map[core.EntityID]*observerPiece{}
	for observer, holdings := range d.Perception.Intel.Holdings {
		observers[observer] = &observerPiece{Holdings: holdings}
	}
	for member, sweep := range d.SightSweeps {
		observer := sightMember(member)
		if observers[observer] == nil {
			observers[observer] = &observerPiece{}
		}
		observers[observer].Sweep = sweep
	}
	for observer, piece := range observers {
		if err := add(ObserverPiece(observer), piece); err != nil {
			return nil, err
		}
	}
	for key := range in.Held {
		if _, ok := out.Marks[key]; !ok {
			out.Removed = append(out.Removed, key)
		}
	}
	slices.Sort(out.Removed)
	return out, nil
}

// JoinPiecesInput is the input to [JoinPieces].
type JoinPiecesInput struct {
	// Pieces is every piece the store holds, by key, as [Piece.Bytes].
	Pieces map[PieceKey][]byte
}

// JoinPieces rebuilds a checkpoint's rest from every piece the store holds,
// the inverse of [SplitPieces]: its JSON is the rest's byte for byte. The
// log is the head's (NextSeq and LastID); the store joins its segments onto
// it with [record.JoinData] before [LoadEncounter]. Members come out in id
// order, as [Encounter.ToData] writes them. Pure.
//
// Errors: [ErrNilInput] for a nil input; [ErrMissingPiece] naming the head,
// the roster, knowledge or the journal when one is absent; [ErrBadPiece]
// naming a piece that does not decode, a key that names no piece, a member
// piece whose id is not its key's, or an observer piece with neither
// holdings nor a sweep (or a sweep on an observer that is not a member);
// [ErrRestCarriesRecord] when the head's log carries entries or audiences.
func JoinPieces(in *JoinPiecesInput) (*EncounterData, error) {
	if in == nil {
		return nil, fmt.Errorf("join pieces: %w", ErrNilInput)
	}
	decode := func(key PieceKey, v any) error {
		raw, ok := in.Pieces[key]
		if !ok {
			return fmt.Errorf("join pieces: %s: %w", key, ErrMissingPiece)
		}
		if err := json.Unmarshal(raw, v); err != nil {
			return fmt.Errorf("join pieces: %s: %w: %w", key, ErrBadPiece, err)
		}
		return nil
	}
	var head headPiece
	var roster rosterPiece
	var know knowledgePiece
	var journal *JournalData
	for _, fixed := range []struct {
		key PieceKey
		v   any
	}{{PieceHead, &head}, {PieceRoster, &roster}, {PieceKnowledge, &know}, {PieceJournal, &journal}} {
		if err := decode(fixed.key, fixed.v); err != nil {
			return nil, err
		}
	}
	if len(head.Log.Entries) > 0 || len(head.Log.Audiences) > 0 {
		return nil, fmt.Errorf("join pieces: %s: %w", PieceHead, ErrRestCarriesRecord)
	}
	out := &EncounterData{
		Clock: head.Clock, Bubbles: head.Bubbles, Landed: head.Landed, PerceptionSubjects: head.PerceptionSubjects,
		Log: head.Log, SiteHash: head.SiteHash, Endings: head.Endings, Retention: head.Retention,
		Outcome: roster.Outcome, Doors: roster.Doors, SightAreas: roster.SightAreas, Interactions: roster.Interactions,
		Sharing: roster.Sharing, Reserve: roster.Reserve, EverMembers: roster.EverMembers, Pause: roster.Pause,
		Discovery: know.Discovery, SneakVerdicts: know.SneakVerdicts,
		Journal: journal,
		Members: []MemberData{},
	}
	for _, key := range slices.Sorted(maps.Keys(in.Pieces)) {
		switch {
		case key == PieceHead, key == PieceRoster, key == PieceKnowledge, key == PieceJournal:
		case key.Shape() == ShapeRoster:
			var m MemberData
			if err := decode(key, &m); err != nil {
				return nil, err
			}
			if MemberPiece(m.ID) != key {
				return nil, fmt.Errorf("join pieces: %s holds member %q: %w", key, m.ID, ErrBadPiece)
			}
			out.Members = append(out.Members, m)
		case key.Shape() == ShapeKnowledge:
			var o observerPiece
			if err := decode(key, &o); err != nil {
				return nil, err
			}
			observer := core.EntityID(strings.TrimPrefix(string(key), observerPiecePrefix))
			if len(o.Holdings) == 0 && o.Sweep == "" {
				return nil, fmt.Errorf("join pieces: %s holds nothing: %w", key, ErrBadPiece)
			}
			if len(o.Holdings) > 0 {
				if out.Perception.Intel.Holdings == nil {
					out.Perception.Intel.Holdings = map[core.EntityID]map[intel.Subject]map[intel.Topic]intel.HoldingData{}
				}
				out.Perception.Intel.Holdings[observer] = o.Holdings
			}
			if o.Sweep != "" {
				member, ok := subjectID(observer, memberSubjectKind)
				if !ok {
					return nil, fmt.Errorf("join pieces: %s sweeps but is no member: %w", key, ErrBadPiece)
				}
				if out.SightSweeps == nil {
					out.SightSweeps = map[MemberID]string{}
				}
				out.SightSweeps[member] = o.Sweep
			}
		default:
			return nil, fmt.Errorf("join pieces: %q names no piece: %w", key, ErrBadPiece)
		}
	}
	// Keys are visited sorted, and "m:<id>" sorts as <id> does, so the
	// members are already in the snapshot's id order.
	return out, nil
}
