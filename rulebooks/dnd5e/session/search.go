// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// search.go is the Search verb at the seam (rpg-toolkit#1375; ruled on
// rpg-project#350): member and region in, the composition's Search does the
// rest. Everything that keeps a secret secret — the sweep, the rolls, the
// audience-of-one reveal, failed-equals-empty — is the composition's
// (encounter/search.go); this verb adds only what the seam adds to every
// verb, and deliberately nothing more.

// SearchInput declares a search of a region.
type SearchInput struct {
	// Session is the session to act in.
	Session string

	// Member is who searches. Universally attemptable — no class gate, no
	// prerequisite; the barbarian may search. Characters are the only
	// searchers in v1: the check rolls a real sheet, so a member with no
	// loadable character is refused (ErrNoCharacter) before the region is
	// ever looked at.
	//
	// THE HOST MUST BIND Member TO THE AUTHENTICATED CALLER — the acting-as
	// gate every verb carries ([WhereInput]'s law): this package takes IDs,
	// not identities, so a host that wires a client-supplied member through
	// unchecked lets one player spend another's search.
	Member string

	// Region is the region to sweep — the one the searcher stands in, in
	// v1 (presence is the composition's truth, enforced there). Any other
	// region refuses with ErrElsewhere, and a region that does not exist
	// refuses IDENTICALLY: a distinct answer would let a guessed ID probe
	// for hidden rooms.
	Region string
}

// SearchOutput acknowledges that the search happened, and deliberately
// nothing about what it found.
//
// THE ANSWER NEVER LEAKS THE QUESTION (the composition's own law, carried
// across whole): nothing here varies with what the region held or how a roll
// went — no found list, no roll, no count. A find reaches the searcher the
// way all world change reaches anybody: as their own recipient-scoped
// EventDoorRevealed on their stream. What remains are the seam's two
// host-facing reports, which every verb returns and only the host reads.
type SearchOutput struct {
	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// Search sweeps a region for concealed structure as Member.
//
// Load-act-save like every verb, and the act is one call: the composition
// sweeps the region's concealed doors, rolls each unfound one's find check
// through this package's [checkSeam] — the member's real sheet, their best
// listed approach, dnd5e's own check machinery — and writes any success as a
// fact and beat for the searcher alone. A world with no concealment accepts
// the verb and sweeps nothing: refusing it would itself answer the question
// a search asks.
//
// The sheet is staged BEFORE the composition acts, unconditionally — see
// [stagedCheck] for why the refusal for an unrollable searcher must not vary
// with what the region holds.
//
// There is deliberately no downed gate: Attack and Move refuse a downed
// actor because that pair was ruled (rpg-toolkit#959's fork); OpenDoor and
// Unlock never gained one, and a rule the seam invented for Search alone
// would be a rule, which this package does not own.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoCharacter, ErrBadCharacter, ErrClosed, ErrNoMember,
// ErrBadPosition for an unplaced searcher, ErrElsewhere for a region the
// searcher does not stand in, or ErrSaveFailed with a populated report.
func (m *Manager) Search(ctx context.Context, in *SearchInput) (*SearchOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("search: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("search: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if err := m.stageCheck(ctx, scope, "searcher", in.Member); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if _, err := scope.enc.Search(&encounter.SearchInput{
		Member: encounter.MemberID(in.Member),
		Region: encounter.RegionID(in.Region),
	}); err != nil {
		return nil, fmt.Errorf("search: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	return &SearchOutput{Saved: report, Delivery: delivery}, nil
}
