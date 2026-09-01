// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSaveErrorAfterWritesPreservesEveryEarlierIdentity pins the defensive
// half of first-admission persistence. Join saves the rest first today, but a
// scope may already name dynamic character writes; a later character-save
// failure must not erase those durable facts from the report.
func TestSaveErrorAfterWritesPreservesEveryEarlierIdentity(t *testing.T) {
	cause := errors.New("rest save refused")
	scope := &writeScope{written: []string{"character:prior-a", "character:prior-b"}}

	err := saveErrorAfterWrites(scope, "character:joining", cause)

	require.ErrorIs(t, err, ErrSaveFailed)
	require.ErrorIs(t, err, cause)
	var saveErr *SaveError
	require.ErrorAs(t, err, &saveErr)
	require.Equal(t, SaveReport{
		Written: []string{"character:prior-a", "character:prior-b"},
		Failed:  []string{"character:joining"},
	}, saveErr.Report)

	// The report owns its list. A later append or mutation on the scope cannot
	// rewrite the account already returned to the host.
	scope.written[0] = "character:mutated"
	require.Equal(t, "character:prior-a", saveErr.Report.Written[0])
}

// TestSaveErrorAfterWritesWrapsBarePostWriteFailures is the generic arm used
// by projection, placement, discovery, and commit preparation. With no failed
// aggregate, it still reports every durable identity and leaves the original
// cause matchable.
func TestSaveErrorAfterWritesWrapsBarePostWriteFailures(t *testing.T) {
	cause := errors.New("projection refused")
	scope := &writeScope{written: []string{"character:joining"}}

	err := saveErrorAfterWrites(scope, "", cause)

	require.ErrorIs(t, err, ErrSaveFailed)
	require.ErrorIs(t, err, cause)
	var saveErr *SaveError
	require.ErrorAs(t, err, &saveErr)
	require.Equal(t, SaveReport{Written: []string{"character:joining"}}, saveErr.Report)
}
