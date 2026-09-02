// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"errors"
	"fmt"
)

// SaveError reports an operation that failed once persistence was involved,
// along with what did and did not land.
//
// Most producers are repository failures. A first-admission Join is the narrow
// additional case: its between-runs rest is deliberately saved before
// projection and placement, so a later local refusal has Written entries and
// no Failed aggregate. In both cases a bare error would strand exactly the
// information S6 promises: a caller could not tell a safe retry from durable
// progress. Match the condition with errors.Is(err, ErrSaveFailed); recover the
// detail with errors.As.
type SaveError struct {
	// Report names the aggregates that landed and those that did not.
	Report SaveReport

	// Err is the underlying operation failure, still matchable with errors.Is.
	Err error
}

// Error describes the operation failure and its persistence report.
func (e *SaveError) Error() string {
	return fmt.Sprintf("%v (written %v, failed %v)", e.Err, e.Report.Written, e.Report.Failed)
}

// Unwrap exposes both the ErrSaveFailed sentinel and the underlying operation
// error, so a caller can match either without this type having to guess which
// one they care about.
func (e *SaveError) Unwrap() []error { return []error{ErrSaveFailed, e.Err} }

// saveErrorAfterWrites preserves the durable-write account when a later step
// fails. failed is empty for projection, placement, discovery, and commit
// preparation failures; a character repository refusal supplies the aggregate
// it failed to write.
//
// With no earlier write and no failed aggregate there is no persistence fact
// to add, so the original error passes through unchanged. Existing SaveErrors
// already carry their own complete persistence report and likewise pass
// through rather than being hidden behind a second report.
func saveErrorAfterWrites(scope *writeScope, failed string, err error) error {
	if err == nil {
		return nil
	}
	var saveErr *SaveError
	if errors.As(err, &saveErr) {
		return err
	}

	var written []string
	if scope != nil {
		written = append([]string(nil), scope.written...)
	}
	if len(written) == 0 && failed == "" {
		return err
	}

	report := SaveReport{Written: written}
	if failed != "" {
		report.Failed = []string{failed}
	}
	return &SaveError{Report: report, Err: err}
}
