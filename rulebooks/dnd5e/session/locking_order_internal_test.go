// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// recordingLocker writes down every guard taken and given back, in order.
// failOn refuses that character's guard from its failAt-th request on.
type recordingLocker struct {
	log      []string
	failOn   string
	failAt   int
	requests map[string]int
}

func (l *recordingLocker) LockSession(_ context.Context, in *LockSessionInput) (*LockSessionOutput, error) {
	l.log = append(l.log, "lock session:"+in.Session)
	return &LockSessionOutput{Release: func() { l.log = append(l.log, "unlock session:"+in.Session) }}, nil
}

func (l *recordingLocker) LockCharacter(_ context.Context, in *LockCharacterInput) (*LockCharacterOutput, error) {
	if l.requests == nil {
		l.requests = map[string]int{}
	}
	l.requests[in.Character]++
	if in.Character == l.failOn && l.requests[in.Character] >= l.failAt {
		l.log = append(l.log, "FAIL "+in.Character)
		return nil, ErrBadSessionLock
	}
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

// TestAFailedRetakeGivesBackEveryGuardItTook: a retake that cannot take a
// guard gives back every guard the pass already took, so the verb is left
// holding none it would release out of order.
func TestAFailedRetakeGivesBackEveryGuardItTook(t *testing.T) {
	locker := &recordingLocker{failOn: "carol", failAt: 2}
	m := &Manager{locker: locker}
	scope := &writeScope{}
	ctx := context.Background()

	_, err := m.acquireCharactersFor(ctx, scope, "carol")
	require.NoError(t, err)
	_, err = m.acquireCharactersFor(ctx, scope, "carol", "bob")
	require.Error(t, err)

	require.Equal(t, []string{"lock carol", "unlock carol", "lock bob", "FAIL carol", "unlock bob"}, locker.log)
	require.Empty(t, scope.heldCharacters)
}
