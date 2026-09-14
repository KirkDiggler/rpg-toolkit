// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMindSerialization(t *testing.T) {
	monster := New(Config{ID: "skeleton", Name: "Skeleton", HP: 13})
	monster.SetMind(MindRetaliator)

	require.Equal(t, MindRetaliator, monster.ToData().Mind)
}

func TestParseMind_Table(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Mind
		wantErr bool
	}{
		{name: "retaliator", input: "retaliator", want: MindRetaliator},
		{name: "berserker", input: "berserker", want: MindBerserker},
		{name: "coward", input: "coward", want: MindCoward},
		{name: "empty string rejected", input: "", wantErr: true},
		{name: "unknown value rejected", input: "genius", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMind(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid mind")
				for _, word := range authorable {
					require.Contains(t, err.Error(), word.String(),
						"a refusal names the whole vocabulary, so an author can see what they meant")
				}
				require.Equal(t, MindUnspecified, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.input, got.String())
		})
	}
}

func TestMind_ZeroValueIsUnspecified(t *testing.T) {
	var zero Mind
	require.Equal(t, MindUnspecified, zero)
	require.Equal(t, "", zero.String())
	require.Equal(t, MindUnspecified, New(Config{ID: "goblin", Name: "Goblin", HP: 7}).Mind())
}
