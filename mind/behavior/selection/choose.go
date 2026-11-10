// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package selection

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrNoCandidates means no row passed every requirement.
	ErrNoCandidates = errors.New("selection: no eligible candidates")
	// ErrZeroWeight means eligible rows exist but none has a random share.
	ErrZeroWeight = errors.New("selection: eligible candidates have zero total weight")
	// ErrUnknownID means the explicit choice does not name a row in this pool.
	ErrUnknownID = errors.New("selection: unknown candidate")
	// ErrIneligible means the named row did not pass every requirement.
	ErrIneligible = errors.New("selection: candidate is ineligible")
	// ErrNoDie refuses a weighted choice without a supplied roller.
	ErrNoDie = errors.New("selection: no die supplied")
	// ErrBadRoll refuses a face outside the die's one-based inclusive bounds.
	ErrBadRoll = errors.New("selection: die returned an invalid face")
	// ErrNilContext refuses a weighted draw with no context for its supplied die.
	ErrNilContext = errors.New("selection: nil draw context")
)

// Die supplies exactly one one-based integer face for the specified size.
// The caller owns identity, randomness and cancellation; no default is supplied.
type Die interface {
	Roll(context.Context, int) (int, error)
}

// Choice identifies a selected input row and carries detached evidence.
// Index is the original input position, including earlier ineligible rows.
type Choice struct {
	Index int
	Row   Row
}

// ChooseInput names an explicit candidate in a captured evaluation.
type ChooseInput struct {
	Evaluation *Evaluation
	ID         string
}

// Choose selects an eligible ID without randomness or re-evaluation. An eligible
// zero-share row is valid. Errors: ErrNilInput, ErrNoEvaluation, ErrUnknownID,
// ErrIneligible. A choice is not authorization to execute in a changed world.
func Choose(in *ChooseInput) (*Choice, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	e := in.Evaluation
	if e == nil || !e.valid {
		return nil, ErrNoEvaluation
	}
	i, ok := e.byID[in.ID]
	if !ok {
		return nil, fmt.Errorf("%q: %w", in.ID, ErrUnknownID)
	}
	if !e.rows[i].Eligible {
		return nil, fmt.Errorf("%q: %w", in.ID, ErrIneligible)
	}
	return &Choice{Index: i, Row: cloneRow(e.rows[i])}, nil
}

// DrawInput supplies the captured pool and its one source of randomness.
type DrawInput struct {
	Evaluation *Evaluation
	Die        Die
}

// DrawOutput is the actual weighted choice, its face and the die size used.
type DrawOutput struct {
	Choice Choice
	Roll   int
	Total  int
}

// Draw makes one weighted choice in supplied candidate order. It never re-reads
// facts, updates a weight or executes a result. Empty eligibility and zero total
// refuse without touching the die; invalid faces never fall back to a row.
// Errors: ErrNilInput, ErrNoEvaluation, ErrNoCandidates, ErrZeroWeight, ErrNoDie,
// ErrNilContext, ErrBadRoll, or the supplied die's wrapped error.
func Draw(ctx context.Context, in *DrawInput) (*DrawOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	e := in.Evaluation
	if e == nil || !e.valid {
		return nil, ErrNoEvaluation
	}
	if e.eligible == 0 {
		return nil, ErrNoCandidates
	}
	if e.total == 0 {
		return nil, ErrZeroWeight
	}
	if in.Die == nil {
		return nil, ErrNoDie
	}
	if ctx == nil {
		return nil, ErrNilContext
	}
	face, err := in.Die.Roll(ctx, e.total)
	if err != nil {
		return nil, fmt.Errorf("selection draw: %w", err)
	}
	if face < 1 || face > e.total {
		return nil, fmt.Errorf("%d outside 1..%d: %w", face, e.total, ErrBadRoll)
	}
	remaining := face
	for i, row := range e.rows {
		if remaining <= row.Weight {
			return &DrawOutput{Choice: Choice{Index: i, Row: cloneRow(row)}, Roll: face, Total: e.total}, nil
		}
		remaining -= row.Weight
	}
	return nil, ErrBadRoll
}
