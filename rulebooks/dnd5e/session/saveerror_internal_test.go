// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReportUnrecordedMergesInnerReportInFirstSeenOrder pins the account at
// the recording boundary. A nested persistence failure has already named what
// it wrote and what it could not; the outer boundary adds its earlier writes
// and the world whose new story did not land without hiding either inner list.
func TestReportUnrecordedMergesInnerReportInFirstSeenOrder(t *testing.T) {
	cause := errors.New("boundary character save refused")
	inner := &SaveError{
		Report: SaveReport{
			Written: []string{"character:boundary-written", "character:shared"},
			Failed:  []string{"character:boundary-failed"},
		},
		Err: cause,
	}
	scope := &writeScope{
		encounter: "world",
		written:   []string{"character:earlier", "character:shared"},
	}

	err := reportUnrecorded(scope, fmt.Errorf("record consequences: %w", inner))

	var saveErr *SaveError
	require.ErrorAs(t, err, &saveErr)
	require.Equal(t, SaveReport{
		Written: []string{"character:earlier", "character:shared", "character:boundary-written"},
		Failed:  []string{"character:boundary-failed", "encounter:world"},
	}, saveErr.Report)
}

// TestReportUnrecordedDeduplicatesEachListIndependently keeps report identity
// separate from write count. Repeated saves retain their newer writes but name
// an aggregate once per list; an earlier successful save and a newer failed
// save of that same aggregate remain truthful in both lists.
func TestReportUnrecordedDeduplicatesEachListIndependently(t *testing.T) {
	inner := &SaveError{
		Report: SaveReport{
			Written: []string{"character:shared", "character:inner", "character:inner"},
			Failed:  []string{"character:shared", "character:failed", "character:failed", "encounter:world"},
		},
		Err: errors.New("newer write refused"),
	}
	scope := &writeScope{
		encounter: "world",
		written:   []string{"character:outer", "character:shared", "character:outer"},
	}

	err := reportUnrecorded(scope, inner)

	var saveErr *SaveError
	require.ErrorAs(t, err, &saveErr)
	require.Equal(t,
		[]string{"character:outer", "character:shared", "character:inner"},
		saveErr.Report.Written,
	)
	require.Equal(t,
		[]string{"character:shared", "character:failed", "encounter:world"},
		saveErr.Report.Failed,
	)
	require.Contains(t, saveErr.Report.Written, "character:shared")
	require.Contains(t, saveErr.Report.Failed, "character:shared")
}

// TestReportUnrecordedOwnsItsReportSlices proves the returned account cannot
// be rewritten through the mutable scope, the nested report, or the caller's
// own mutation of the other returned list.
func TestReportUnrecordedOwnsItsReportSlices(t *testing.T) {
	innerWritten := []string{"character:inner-written"}
	innerFailed := []string{"character:inner-failed"}
	inner := &SaveError{
		Report: SaveReport{Written: innerWritten, Failed: innerFailed},
		Err:    errors.New("nested save refused"),
	}
	scopeWritten := []string{"character:scope-written"}
	scope := &writeScope{encounter: "world", written: scopeWritten}

	err := reportUnrecorded(scope, inner)
	var saveErr *SaveError
	require.ErrorAs(t, err, &saveErr)

	scopeWritten[0] = "character:mutated-scope"
	innerWritten[0] = "character:mutated-inner-written"
	innerFailed[0] = "character:mutated-inner-failed"
	require.Equal(t,
		[]string{"character:scope-written", "character:inner-written"},
		saveErr.Report.Written,
	)
	require.Equal(t,
		[]string{"character:inner-failed", "encounter:world"},
		saveErr.Report.Failed,
	)

	saveErr.Report.Written[0] = "character:mutated-returned-written"
	require.Equal(t, "character:mutated-scope", scopeWritten[0],
		"the returned Written list must not share scope storage")
	require.Equal(t, "character:mutated-inner-written", innerWritten[0],
		"the returned Written list must not share the nested report")
	require.Equal(t, "character:inner-failed", saveErr.Report.Failed[0],
		"the returned Written and Failed lists must not share storage")

	saveErr.Report.Failed[0] = "character:mutated-returned-failed"
	require.Equal(t, "character:mutated-inner-failed", innerFailed[0],
		"the returned Failed list must not share the nested report")
}

// TestReportUnrecordedPreservesEveryCause keeps the host's own failure chain
// reachable through both persistence-report layers.
func TestReportUnrecordedPreservesEveryCause(t *testing.T) {
	first := errors.New("first repository cause")
	second := errors.New("second repository cause")
	inner := &SaveError{
		Report: SaveReport{Failed: []string{"character:alice"}},
		Err:    fmt.Errorf("saving boundary sheet: %w", errors.Join(first, second)),
	}

	err := reportUnrecorded(
		&writeScope{encounter: "world", written: []string{"character:alice"}},
		fmt.Errorf("notice down: %w", inner),
	)

	require.ErrorIs(t, err, ErrSaveFailed)
	require.ErrorIs(t, err, first)
	require.ErrorIs(t, err, second)
	require.Contains(t, err.Error(), "notice down")
	require.Contains(t, err.Error(), "saving boundary sheet")
}

// TestReportUnrecordedAddsTheWorldWhenOnlyTheNestedWritePersisted covers the
// reachable shape where recording itself produced the first persistence fact.
// Returning the inner SaveError unchanged would omit the unrecorded world.
func TestReportUnrecordedAddsTheWorldWhenOnlyTheNestedWritePersisted(t *testing.T) {
	inner := &SaveError{
		Report: SaveReport{
			Written: []string{"character:inner"},
			Failed:  []string{"character:failed"},
		},
		Err: errors.New("nested save refused"),
	}

	err := reportUnrecorded(&writeScope{encounter: "world"}, inner)

	var saveErr *SaveError
	require.ErrorAs(t, err, &saveErr)
	require.Equal(t, SaveReport{
		Written: []string{"character:inner"},
		Failed:  []string{"character:failed", "encounter:world"},
	}, saveErr.Report)
}

// TestReportUnrecordedBareErrorCompatibility preserves the old distinction:
// a failure after an earlier durable write needs a report, while a failure
// with no persistence fact remains the original bare error.
func TestReportUnrecordedBareErrorCompatibility(t *testing.T) {
	t.Run("post-write", func(t *testing.T) {
		cause := errors.New("story refused")
		err := reportUnrecorded(
			&writeScope{encounter: "world", written: []string{"character:alice"}},
			cause,
		)

		var saveErr *SaveError
		require.ErrorAs(t, err, &saveErr)
		require.Equal(t, SaveReport{
			Written: []string{"character:alice"},
			Failed:  []string{"encounter:world"},
		}, saveErr.Report)
		require.ErrorIs(t, err, cause)
	})

	t.Run("no-write", func(t *testing.T) {
		cause := errors.New("story refused before persistence")
		err := reportUnrecorded(&writeScope{encounter: "world"}, cause)

		require.Equal(t, cause, err)
		require.NotErrorIs(t, err, ErrSaveFailed)
		var saveErr *SaveError
		require.NotErrorAs(t, err, &saveErr)
	})
}
