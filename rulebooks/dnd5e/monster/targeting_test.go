// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetingSerialization(t *testing.T) {
	monster := New(Config{ID: "goblin", Name: "Goblin", HP: 7})
	monster.SetTargeting(TargetLowestHP)

	require.Equal(t, TargetLowestHP, monster.ToData().Targeting)
}

func TestParseTargetingStrategy_Table(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    TargetingStrategy
		wantErr bool
	}{
		{name: "closest", input: "closest", want: TargetClosest},
		{name: "lowest-health", input: "lowest-health", want: TargetLowestHP},
		{name: "lowest-ac", input: "lowest-ac", want: TargetLowestAC},
		{name: "empty string rejected", input: "", wantErr: true},
		{name: "unknown value rejected", input: "random", wantErr: true},
		{name: "lowest-hp is not the author-facing label", input: "lowest-hp", wantErr: true},
		{name: "unspecified is not an authorable choice", input: "unspecified", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTargetingStrategy(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid targeting strategy")
				if tc.input != "" {
					require.Contains(t, err.Error(), tc.input)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.input, got.String())
		})
	}
}

func TestTargetingStrategy_Ref(t *testing.T) {
	require.Equal(t, "dnd5e:targeting:closest", TargetClosest.Ref())
	require.Equal(t, "dnd5e:targeting:lowest-hp", TargetLowestHP.Ref())
	require.Equal(t, "dnd5e:targeting:lowest-ac", TargetLowestAC.Ref())
	require.Equal(t, "dnd5e:targeting:closest", TargetingUnspecified.Ref())
}

func TestTargetingStrategy_ZeroValueIsUnspecified(t *testing.T) {
	var zero TargetingStrategy
	require.Equal(t, TargetingUnspecified, zero)
	require.NotEqual(t, TargetClosest, zero)
	require.Equal(t, "unspecified", zero.String())
	require.Equal(t, "closest", TargetClosest.String())
}
