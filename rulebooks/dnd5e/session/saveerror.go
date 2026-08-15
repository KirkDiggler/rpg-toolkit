// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "fmt"

// SaveError reports a failed save along with what did and did not land.
//
// It exists because a verb returns no output when it returns an error, so a
// bare sentinel would strand exactly the information S6 promises: a caller who
// cannot see the report cannot tell a total failure from a half one, and that
// is the difference between "retry the verb" and "the world moved but what it
// owes did not". Match the condition with errors.Is(err, ErrSaveFailed);
// recover the detail with errors.As.
type SaveError struct {
	// Report names the aggregates that landed and those that did not.
	Report SaveReport

	// Err is the underlying repository failure, still matchable with errors.Is.
	Err error
}

// Error describes which save failed.
func (e *SaveError) Error() string {
	return fmt.Sprintf("%v (written %v, failed %v)", e.Err, e.Report.Written, e.Report.Failed)
}

// Unwrap exposes both the ErrSaveFailed sentinel and the repository's own
// error, so a caller can match either without this type having to guess which
// one they care about.
func (e *SaveError) Unwrap() []error { return []error{ErrSaveFailed, e.Err} }
