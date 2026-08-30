// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package quest

import (
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// ErrUnknownJob reports a claim on a job that is not on the board.
var ErrUnknownJob = errors.New("there is no job by that name on the board")

// ErrDuplicateJob reports the same job declared twice.
var ErrDuplicateJob = errors.New("this job is declared twice — each job needs its own id")

// Ledger is every job on the board, including the ones that opened because of
// how an earlier job came out.
//
// It is the quest side of a world in one place, so the composer can hand it a
// journal and a graph after every act and be told what changed.
type Ledger struct {
	boards []*Board
	index  map[string]*Board
}

// LedgerReport is one observation of every job in play.
type LedgerReport struct {
	// Boards is one report per job.
	Boards []BoardReport

	// Events is every transition this observation caused, across all jobs.
	Events []Event

	// Refusals are follow-ups that activated but could not be put on the
	// board. A healthy ledger's is empty; a fold that quietly dropped a job
	// would be the failure worth preventing.
	Refusals []string
}

// NewLedger puts the declared jobs on the board.
//
// Every template is validated up front rather than at first claim, so a content
// package that is missing something is told at startup and not three sessions
// in. Returns [ErrDuplicateJob] or any of the template errors.
func NewLedger(templates ...Template) (*Ledger, error) {
	l := &Ledger{index: make(map[string]*Board, len(templates))}
	for _, t := range templates {
		if _, seen := l.index[t.ID]; seen {
			return nil, fmt.Errorf("%q: %w", t.ID, ErrDuplicateJob)
		}
		board, err := NewBoard(t)
		if err != nil {
			return nil, err
		}
		l.add(board)
	}

	return l, nil
}

func (l *Ledger) add(b *Board) {
	l.boards = append(l.boards, b)
	l.index[b.TemplateID()] = b
}

// Board returns a job by id and whether it is on the board.
func (l *Ledger) Board(templateID string) (*Board, bool) {
	b, ok := l.index[templateID]

	return b, ok
}

// Boards returns every job in play, in the order they opened.
func (l *Ledger) Boards() []*Board {
	return slices.Clone(l.boards)
}

// Claim takes a subject off the named job and mints the claimant's instance.
//
// Returns [ErrUnknownJob] or [ErrBoardExhausted].
func (l *Ledger) Claim(templateID string, by journal.EntityID) (*Instance, []Event, error) {
	board, ok := l.index[templateID]
	if !ok {
		return nil, nil, fmt.Errorf("%q: %w", templateID, ErrUnknownJob)
	}

	return board.Claim(by)
}

// Observe reports every job and puts any newly activated follow-up on the
// board.
//
// A follow-up opened by this observation is not itself observed until the next
// one. That is deliberate: it has just appeared, nobody has claimed it, and
// settling it in the same breath that created it would let a job be finished
// before it was ever offered.
func (l *Ledger) Observe(w *graph.World, log *journal.Journal) LedgerReport {
	var report LedgerReport

	for _, board := range slices.Clone(l.boards) {
		one := board.Observe(w, log)
		report.Boards = append(report.Boards, one)
		report.Events = append(report.Events, one.Events...)

		for _, opened := range one.Opens {
			l.open(opened, &report)
		}
	}

	return report
}

func (l *Ledger) open(t Template, report *LedgerReport) {
	if _, seen := l.index[t.ID]; seen {
		report.Refusals = append(report.Refusals,
			fmt.Sprintf("follow-up %q is already on the board", t.ID))

		return
	}

	board, err := NewBoard(t)
	if err != nil {
		report.Refusals = append(report.Refusals, fmt.Sprintf("follow-up %q: %v", t.ID, err))

		return
	}
	l.add(board)
}
