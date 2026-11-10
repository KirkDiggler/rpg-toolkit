// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// ToCheckpointInput is the input to [Encounter.ToCheckpoint].
type ToCheckpointInput struct {
	// Since is the first story seq the store does not hold yet: the NextSeq
	// of the record it last saved, 1 (or 0) for a store that holds none. The
	// store, not the encounter, knows it; the session keeps it beside its
	// checkpoint fence (rpg-project#897). Since above the story's NextSeq is
	// refused: the store cannot hold beats the encounter never wrote.
	Since uint64
}

// ToCheckpointOutput is the output of [Encounter.ToCheckpoint].
type ToCheckpointOutput struct {
	// Rest is [Encounter.ToData] with the story log replaced by its head:
	// NextSeq and LastID, no entries and no audience table.
	//
	// A Rest alone decodes as a legal TRIMMED-EMPTY log (NextSeq >= 2, no
	// entries), so a reader handed a rest without its segments loads an
	// encounter with a silently empty record. The host must therefore keep
	// the rest where no whole-checkpoint reader looks, and refuse by name a
	// stored rest whose segments are missing (the api leg's job).
	Rest EncounterData
	// Appended is the story's segment from Since: the retained entries at or
	// after Since with their own audience table, [record.Log.DataFrom]. It
	// holds no entries for a verb that appended no beat. Older entries are
	// never copied.
	Appended record.LogData
	// Floor is the story's floor after this save's trim: the oldest seq it
	// retains, its NextSeq when it retains none, zero (or 1) for a log that
	// has never lost a beat. The store drops every segment wholly below it
	// and keeps it beside the rest, and the load passes it to
	// [record.JoinData], which drops the straddling segment's older entries
	// and tolerates the seqs the store no longer holds. Under
	// [RetentionUnbounded] nothing is ever trimmed, so it never rises above
	// the first seq.
	Floor uint64
}

// ToCheckpoint is [Encounter.ToData] for a store that APPENDS the record
// rather than rewriting it (rpg-project#897 leg c): the rest of the world
// whole, the story as one segment from Since, and the floor below which the
// store may forget. It is the storage boundary exactly as ToData is,
// enforcing retention first. A store that held the record through Since-1
// appends Appended, drops every stored segment wholly below Floor, keeps
// Floor beside the rest, and on load joins what it holds:
//
//	record.JoinData(&record.JoinDataInput{Head: rest.Log, Floor: floor, Segments: stored})
//
// which is exactly ToData's Log at this save. The encounter keeps no record
// of what any store holds: Since is an argument, never a field.
//
// THE ORDER IS THE CONTRACT. Since is valid from 0 through the story's
// NextSeq, and it is checked FIRST, before retention runs, so a refused call
// changes nothing: the live log is not trimmed and the floor does not move.
// Retention never moves NextSeq, so the check means the same on either side
// of the trim. Appended is taken AFTER the trim: a Since below this save's
// floor yields the segment from the floor, never an entry the trim dropped.
func (e *Encounter) ToCheckpoint(in *ToCheckpointInput) (*ToCheckpointOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	next, err := e.story.NextSeq()
	if err != nil {
		return nil, fmt.Errorf("to checkpoint: next seq: %w", err)
	}
	if in.Since > next {
		return nil, fmt.Errorf("to checkpoint: since %d is past the story's next seq %d: %w",
			in.Since, next, ErrBadSince)
	}
	if err := e.enforceRetention(); err != nil {
		return nil, fmt.Errorf("to checkpoint: retention: %w", err)
	}

	head, err := e.story.DataFrom(&record.DataFromInput{Since: next})
	if err != nil {
		return nil, fmt.Errorf("to checkpoint: head: %w", err)
	}
	appended, err := e.story.DataFrom(&record.DataFromInput{Since: in.Since})
	if err != nil {
		return nil, fmt.Errorf("to checkpoint: segment: %w", err)
	}
	return &ToCheckpointOutput{
		Rest:     e.snapshot(head),
		Appended: appended,
		Floor:    e.logFloor,
	}, nil
}
