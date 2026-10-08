// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// recordingLocker writes down every guard taken and given back, in order.
type recordingLocker struct{ log []string }

func (l *recordingLocker) LockSession(_ context.Context, in *LockSessionInput) (*LockSessionOutput, error) {
	l.log = append(l.log, "lock session:"+in.Session)
	return &LockSessionOutput{Release: func() { l.log = append(l.log, "unlock session:"+in.Session) }}, nil
}

func (l *recordingLocker) LockCharacter(_ context.Context, in *LockCharacterInput) (*LockCharacterOutput, error) {
	l.log = append(l.log, "lock "+in.Character)
	return &LockCharacterOutput{Release: func() { l.log = append(l.log, "unlock "+in.Character) }}, nil
}

// TestCharacterGuardsKeepIDOrderAcrossAVerb: a verb holding carol's guard
// that comes to need bob's (a Join whose commit closes the run) gives carol's
// back, takes bob's, then carol's again — never waiting on a lower id while
// holding a higher one — and never asks for a guard it holds twice.
func TestCharacterGuardsKeepIDOrderAcrossAVerb(t *testing.T) {
	locker := &recordingLocker{}
	m := &Manager{locker: locker}
	scope := &writeScope{}
	ctx := context.Background()

	releaseCarol, err := m.acquireCharactersFor(ctx, scope, "carol")
	require.NoError(t, err)
	releaseBoth, err := m.acquireCharactersFor(ctx, scope, "carol", "bob")
	require.NoError(t, err)

	require.Equal(t, []string{"lock carol", "unlock carol", "lock bob", "lock carol"}, locker.log)

	releaseBoth()
	require.Equal(t, "unlock bob", locker.log[len(locker.log)-1], "the second call gives back only what it asked for")
	releaseCarol()
	require.Equal(t, "unlock carol", locker.log[len(locker.log)-1], "the first call's guard, retaken, is still the first call's to give back")
	require.Empty(t, scope.heldCharacters)
}
