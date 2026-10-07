// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// SettlementInput bounds a [Encounter.Settlement] read to what one verb
// settled.
type SettlementInput struct {
	// FromSeq is the INCLUSIVE lower bound: the sequence the verb's first
	// beat took ([Encounter.NextStorySeq] read before the verb ran) — the same
	// bound [StoryInput.AfterSeq] carries. Zero reads the whole retained
	// record.
	FromSeq uint64
}

// FightEnded is one fight that ended: who was in it and why it ended.
type FightEnded struct {
	// Seq is the sequence of the beat that told the ending.
	Seq uint64
	// Members are the fight's members when it ended, in turn order.
	Members []MemberID
	// Cause is why it ended ([DissolveByDecision], [DissolveByDefeat],
	// [DissolveByStance]).
	Cause DissolveKind
}

// MemberFell is one member falling: the composition noticing a member down.
type MemberFell struct {
	// Seq is the sequence of the beat that told the fall. Falls are listed in
	// the order they happened, which is this order.
	Seq uint64
	// Member is who fell.
	Member MemberID
	// Kind is the fallen member's kind as the roster held it at the fall,
	// carried on the beat. Empty only for a fall told before beats carried
	// it whose member has since exited: the kind is then unknown, never
	// guessed.
	Kind MemberKind
}

// SettlementOutput is what a verb settled, as typed facts.
type SettlementOutput struct {
	// FightsEnded is every fight that ended at or after FromSeq, in order.
	FightsEnded []FightEnded
	// Falls is every fall at or after FromSeq, in order. A member who fell,
	// recovered and fell again inside the bound falls twice.
	Falls []MemberFell
}

// Settlement reports what a verb settled — the fights that ended and the
// members that fell — as typed facts read from this encounter's own record
// (rpg-project#539, "What a verb settles").
//
// ACROSS EVERY AUDIENCE. A settlement is a fact about the world, not about
// who saw it: a fight that ended where no player could see it ended all the
// same, and the reader is never handed a per-viewer story to stitch one
// together from. The beats are this module's own, so this module is the only
// one that decodes them; the reader decodes nothing.
//
// Errors: ErrNilInput; ErrTrimmed when a non-zero FromSeq names a sequence
// already trimmed from the retained record (a short answer would be
// indistinguishable from a complete one); ErrInvalidData for a fight or fall
// beat that does not say what it must. A fall whose member has since exited
// is still a fall.
func (e *Encounter) Settlement(in *SettlementInput) (*SettlementOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("settlement: %w", ErrNilInput)
	}
	if in.FromSeq > 0 && in.FromSeq < e.logFloor {
		return nil, fmt.Errorf("settlement: seq %d below retained floor %d: %w", in.FromSeq, e.logFloor, ErrTrimmed)
	}
	entries, err := e.story.All(&record.AllInput{FromSeq: in.FromSeq})
	if err != nil {
		return nil, fmt.Errorf("settlement: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })

	out := &SettlementOutput{}
	for _, entry := range entries {
		var peek struct {
			Beat string `json:"beat"`
		}
		if json.Unmarshal(entry.Payload, &peek) != nil {
			// Every beat this module writes is a JSON object; a payload that
			// is not one is not a fight or a fall this module told.
			continue
		}
		switch peek.Beat {
		case BeatFightEnded:
			var beat struct {
				Members []MemberID `json:"members"`
				Cause   string     `json:"cause"`
			}
			if err := json.Unmarshal(entry.Payload, &beat); err != nil {
				return nil, fmt.Errorf("settlement: fight ended at %d: %w: %v", entry.Seq, ErrInvalidData, err)
			}
			cause := DissolveKind(beat.Cause)
			if !validDissolveKind(cause) {
				return nil, fmt.Errorf("settlement: fight ended at %d: cause %q: %w", entry.Seq, beat.Cause, ErrInvalidData)
			}
			out.FightsEnded = append(out.FightsEnded, FightEnded{
				Seq: entry.Seq, Members: append([]MemberID(nil), beat.Members...), Cause: cause,
			})
		case string(OutcomeDown):
			var beat struct {
				Member MemberID   `json:"member"`
				Kind   MemberKind `json:"kind"`
			}
			if err := json.Unmarshal(entry.Payload, &beat); err != nil {
				return nil, fmt.Errorf("settlement: fall at %d: %w: %v", entry.Seq, ErrInvalidData, err)
			}
			if beat.Member == "" {
				return nil, fmt.Errorf("settlement: fall at %d: %w: %w", entry.Seq, ErrInvalidData, ErrNoMember)
			}
			// The beat carries the kind the roster held at the fall. A beat
			// written before it did falls back to the roster; a fall is a
			// true historical fact either way, and is never refused for a
			// member who has since exited.
			if beat.Kind == "" {
				if m, ok := e.members[beat.Member]; ok {
					beat.Kind = m.Kind
				}
			}
			out.Falls = append(out.Falls, MemberFell{Seq: entry.Seq, Member: beat.Member, Kind: beat.Kind})
		}
	}
	return out, nil
}

func validDissolveKind(kind DissolveKind) bool {
	switch kind {
	case DissolveByDecision, DissolveByDefeat, DissolveByStance:
		return true
	default:
		return false
	}
}
