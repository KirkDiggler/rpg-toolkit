// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package selection

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	// ErrNilInput refuses a missing input value.
	ErrNilInput = errors.New("selection: nil input")
	// ErrNoEvaluation refuses nil or zero-value evaluations; use Evaluate.
	ErrNoEvaluation = errors.New("selection: no evaluated pool")
	// ErrInvalidCandidate refuses invalid identities, verdicts or negative numbers.
	ErrInvalidCandidate = errors.New("selection: invalid candidate")
	// ErrOverflow refuses a share calculation that cannot be represented as int.
	ErrOverflow = errors.New("selection: weight overflow")
)

// Verdict is a caller's answer to a named requirement, not a fact reader.
type Verdict uint8

const (
	// Unknown means the caller has no established answer. It is not Unmet.
	Unknown Verdict = iota
	// Unmet means the caller evaluated the requirement and it did not hold.
	Unmet
	// Met means the caller evaluated the requirement and it held.
	Met
)

// Requirement carries an identified condition's verdict and optional explanation.
// IDs must be nonblank and unique within a candidate's requirements.
type Requirement struct {
	ID      string
	Verdict Verdict
	Reason  string
}

// Factor is one named nonnegative integer multiplier. It has no percentage
// interpretation: callers choose their common share units. IDs are unique
// within a candidate's factors; order is retained for evidence and arithmetic.
type Factor struct {
	ID         string
	Multiplier int
}

// Candidate identifies a row, its nonnegative base weight and supplied evidence.
// An empty requirement list is satisfied; no factors means multiplication by one.
type Candidate struct {
	ID           string
	Weight       int
	Requirements []Requirement
	Factors      []Factor
}

// EvaluateInput is an authored-order candidate list; IDs must be unique/nonblank.
type EvaluateInput struct{ Candidates []Candidate }

// Row is detached evidence for one input candidate. Weight is its effective
// random share, zero when ineligible; BaseWeight and factors retain the inputs.
// Eligible zero-share rows remain explicitly selectable.
type Row struct {
	ID           string
	BaseWeight   int
	Requirements []Requirement
	Factors      []Factor
	Eligible     bool
	Weight       int
}

// Inspection is detached evidence for a whole evaluation. Eligible counts rows
// passing every requirement, including zero-share rows; Total sums their shares.
type Inspection struct {
	Rows     []Row
	Eligible int
	Total    int
}

// Evaluation is a captured pool. Its zero value is invalid; construct it through
// Evaluate. Operations never mutate it or obtain new facts from its caller.
type Evaluation struct {
	rows     []Row
	byID     map[string]int
	eligible int
	total    int
	valid    bool
}

const maxInt = int(^uint(0) >> 1)

// Evaluate validates and captures a candidate list, folds supplied requirement
// verdicts, and computes checked integer shares. It reads no world and rolls
// nothing. A constructed empty evaluation is valid. Malformed input returns
// ErrInvalidCandidate; eligible product/total overflow returns ErrOverflow.
// No partial evaluation is returned on error. Ineligible rows do no arithmetic.
func Evaluate(in *EvaluateInput) (*Evaluation, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	e := &Evaluation{rows: make([]Row, 0, len(in.Candidates)), byID: make(map[string]int, len(in.Candidates)), valid: true}
	for i, c := range in.Candidates {
		if strings.TrimSpace(c.ID) == "" || c.Weight < 0 {
			return nil, fmt.Errorf("candidate %q: %w", c.ID, ErrInvalidCandidate)
		}
		if _, duplicate := e.byID[c.ID]; duplicate {
			return nil, fmt.Errorf("duplicate candidate %q: %w", c.ID, ErrInvalidCandidate)
		}
		eligible, err := validateEvidence(c)
		if err != nil {
			return nil, err
		}
		row := Row{ID: c.ID, BaseWeight: c.Weight, Requirements: slices.Clone(c.Requirements), Factors: slices.Clone(c.Factors), Eligible: eligible}
		if eligible {
			row.Weight = c.Weight
			for _, factor := range c.Factors {
				if row.Weight != 0 && factor.Multiplier > maxInt/row.Weight {
					return nil, fmt.Errorf("candidate %q factor %q: %w", c.ID, factor.ID, ErrOverflow)
				}
				row.Weight *= factor.Multiplier
			}
			if row.Weight > maxInt-e.total {
				return nil, fmt.Errorf("candidate %q total: %w", c.ID, ErrOverflow)
			}
			e.eligible++
			e.total += row.Weight
		}
		e.byID[c.ID] = i
		e.rows = append(e.rows, row)
	}
	return e, nil
}

// validateEvidence checks even gated-out rows; invalid input is not a verdict.
func validateEvidence(c Candidate) (bool, error) {
	eligible := true
	requirements := make(map[string]bool, len(c.Requirements))
	for _, r := range c.Requirements {
		if strings.TrimSpace(r.ID) == "" || requirements[r.ID] || r.Verdict > Met {
			return false, fmt.Errorf("candidate %q requirement %q: %w", c.ID, r.ID, ErrInvalidCandidate)
		}
		requirements[r.ID] = true
		eligible = eligible && r.Verdict == Met
	}
	factors := make(map[string]bool, len(c.Factors))
	for _, f := range c.Factors {
		if strings.TrimSpace(f.ID) == "" || factors[f.ID] || f.Multiplier < 0 {
			return false, fmt.Errorf("candidate %q factor %q: %w", c.ID, f.ID, ErrInvalidCandidate)
		}
		factors[f.ID] = true
	}
	return eligible, nil
}

// Inspect returns a detached account without re-evaluating any requirement or
// weight. Nil/zero-value evaluations return ErrNoEvaluation.
func (e *Evaluation) Inspect() (*Inspection, error) {
	if e == nil || !e.valid {
		return nil, ErrNoEvaluation
	}
	out := &Inspection{Rows: make([]Row, len(e.rows)), Eligible: e.eligible, Total: e.total}
	for i, row := range e.rows {
		out.Rows[i] = cloneRow(row)
	}
	return out, nil
}

func cloneRow(row Row) Row {
	row.Requirements = slices.Clone(row.Requirements)
	row.Factors = slices.Clone(row.Factors)
	return row
}
