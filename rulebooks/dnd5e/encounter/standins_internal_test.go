// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompileOnlyInputsLeaveNoInterfaceSlotNil checks only that no
// interface-typed field of either compile-only input is left nil, except
// Roller, which is optional and refused loudly at the roll. It cannot see
// whether a stand-in answers what the door actually asks — that invariant is
// held by the behavioural tests in standins_test.go, which construct and
// load from these inputs (TestNewEncounterConstructsFromCompileOnlySetup,
// TestLoadEncounterLoadsFromCompileOnlyLoad), and by the field types, which
// name the wide capability interfaces.
func TestCompileOnlyInputsLeaveNoInterfaceSlotNil(t *testing.T) {
	for name, input := range map[string]reflect.Value{
		"CompileOnlySetup": reflect.ValueOf(*CompileOnlySetup(FieldInput{}, nil)),
		"CompileOnlyLoad":  reflect.ValueOf(*CompileOnlyLoad(EncounterData{})),
	} {
		for i := 0; i < input.NumField(); i++ {
			field := input.Type().Field(i)
			if field.Type.Kind() != reflect.Interface || field.Name == "Roller" {
				continue
			}
			require.False(t, input.Field(i).IsNil(), "%s leaves %s nil", name, field.Name)
		}
	}
}

func TestCompileOnlyStandInAnswers(t *testing.T) {
	asked := []MemberID{"b", "a"}

	t.Run("standing: nobody down, as a list", func(t *testing.T) {
		down, err := nobodyDown{}.Standing(asked)
		require.NoError(t, err)
		require.NotNil(t, down)
		require.Empty(t, down)
	})
	t.Run("participation: an empty ask is answered empty", func(t *testing.T) {
		got, err := nobodyDown{}.Assess(nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got.Members)
		require.False(t, got.PartyDefeated)
		require.False(t, got.KeepTurnOrder)
	})
	t.Run("participation: asking about members refuses", func(t *testing.T) {
		got, err := nobodyDown{}.Assess(asked)
		require.ErrorIs(t, err, ErrRefusingParticipation)
		require.Nil(t, got)
	})
	t.Run("sight: zero for every member", func(t *testing.T) {
		got, err := zeroSight{}.Sight(asked)
		require.NoError(t, err)
		require.Equal(t, map[MemberID]int{"a": 0, "b": 0}, got)
	})
	t.Run("initiative refuses", func(t *testing.T) {
		got, err := refusingInitiative{}.RollInitiative(asked)
		require.ErrorIs(t, err, ErrRefusingInitiative)
		require.Nil(t, got)
	})
	t.Run("check resolver refuses", func(t *testing.T) {
		got, err := RefusingCheckResolver{}.ResolveCheck(&ResolveCheckInput{})
		require.ErrorIs(t, err, ErrRefusingCheckResolver)
		require.Nil(t, got)
	})
	t.Run("driver refuses", func(t *testing.T) {
		_, err := RefusingDriver{}.Act(MonsterView{})
		require.ErrorIs(t, err, ErrRefusingDriver)
	})
	t.Run("witness: nobody, as a list", func(t *testing.T) {
		got, err := NobodyPerceives{}.Perceivers(&PerceiversInput{})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	})
}
