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

// reportUnrecorded turns a failure AFTER persistence into one complete account
// a caller can repair from. Recording may itself reach nested consequences — a
// downed member can dissolve a fight, whose boundary announcement can save a
// changed sheet — so the error can already contain a SaveError. The outer
// report merges that account rather than hiding it behind the encounter that
// was not recorded.
//
// Written follows the actual first-seen path: writes already held by the scope,
// then writes reported by the nested failure. Failed keeps the nested order and
// adds the encounter last. Each list deduplicates independently; an aggregate
// can truthfully occur in both when an earlier save landed and a newer save of
// the same aggregate failed. Each returned list is a fresh copy when non-empty
// and nil when that list has no identities.
//
// errors.As deliberately selects the first reachable SaveError. Session
// producers create one report leaf; if this helper is called again, that first
// SaveError is the already-complete outer report made by the earlier call. This
// composes those producer shapes without speculatively folding arbitrary error
// trees.
//
// A bare post-write error keeps the historical scope-writes plus failed-world
// shape. With neither an earlier write nor an inner SaveError there is no
// persistence fact to add, so the original error passes through unchanged. A
// nil scope has no outer persistence fact and therefore also returns the
// original error unchanged.
func reportUnrecorded(scope *writeScope, err error) error {
	if err == nil {
		return nil
	}
	if scope == nil {
		return err
	}

	var inner *SaveError
	hasInner := errors.As(err, &inner) && inner != nil
	if len(scope.written) == 0 && !hasInner {
		return err
	}

	var innerWritten, innerFailed []string
	if hasInner {
		innerWritten = inner.Report.Written
		innerFailed = inner.Report.Failed
	}

	return &SaveError{
		Report: SaveReport{
			Written: mergeReportIdentities(scope.written, innerWritten),
			Failed: mergeReportIdentities(
				innerFailed,
				[]string{"encounter:" + scope.encounter},
			),
		},
		// Keep the whole original chain, including the nested SaveError and
		// every repository cause it exposes through errors.Is.
		Err: err,
	}
}

// mergeReportIdentities returns the first occurrence of each aggregate across
// the lists, in encounter order. It always copies identities into owned
// storage; callers invoke it separately for Written and Failed so identities
// are never deduplicated across those two independent facts.
func mergeReportIdentities(lists ...[]string) []string {
	total := 0
	for _, list := range lists {
		total += len(list)
	}
	if total == 0 {
		return nil
	}

	merged := make([]string, 0, total)
	seen := make(map[string]struct{}, total)
	for _, list := range lists {
		for _, identity := range list {
			if _, ok := seen[identity]; ok {
				continue
			}
			seen[identity] = struct{}{}
			merged = append(merged, identity)
		}
	}
	return merged
}
