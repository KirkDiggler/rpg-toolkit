package races

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/stretchr/testify/assert"
)

func TestRaceRefs(t *testing.T) {
	tests := []struct {
		name     string
		index    string
		wantType string
		wantVal  string
	}{
		{
			name:     "create dwarf ref",
			index:    "dwarf",
			wantType: "race",
			wantVal:  "dwarf",
		},
		{
			name:     "create custom race ref",
			index:    "custom-race",
			wantType: "race",
			wantVal:  "custom-race",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := Ref(tt.index)
			assert.NotNil(t, ref)
			assert.Equal(t, "dnd5e", ref.Module)
			assert.Equal(t, tt.wantType, ref.Type)
			assert.Equal(t, tt.wantVal, ref.Value)
		})
	}
}

func TestRaceConstants(t *testing.T) {
	// Test that constants are properly initialized
	assert.NotNil(t, Human)
	assert.Equal(t, "dnd5e", Human.Module)
	assert.Equal(t, "race", Human.Type)
	assert.Equal(t, "human", Human.Value)

	assert.NotNil(t, Dwarf)
	assert.Equal(t, "dwarf", Dwarf.Value)

	assert.NotNil(t, Elf)
	assert.Equal(t, "elf", Elf.Value)

	// Verify all constants are unique
	refs := []*core.Ref{
		Dragonborn, Dwarf, Elf, Gnome, HalfElf,
		Halfling, HalfOrc, Human, Tiefling,
	}

	seen := make(map[string]bool)
	for _, ref := range refs {
		key := ref.Value
		assert.False(t, seen[key], "duplicate race ref: %s", key)
		seen[key] = true
	}
}