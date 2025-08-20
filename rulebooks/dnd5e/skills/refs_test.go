package skills

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/stretchr/testify/assert"
)

func TestSkillRefs(t *testing.T) {
	tests := []struct {
		name     string
		index    string
		wantType string
		wantVal  string
	}{
		{
			name:     "create athletics ref",
			index:    "athletics",
			wantType: "skill",
			wantVal:  "athletics",
		},
		{
			name:     "create custom skill ref",
			index:    "custom-skill",
			wantType: "skill",
			wantVal:  "custom-skill",
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

func TestSkillConstants(t *testing.T) {
	// Test that constants are properly initialized
	assert.NotNil(t, Athletics)
	assert.Equal(t, "dnd5e", Athletics.Module)
	assert.Equal(t, "skill", Athletics.Type)
	assert.Equal(t, "athletics", Athletics.Value)

	assert.NotNil(t, Stealth)
	assert.Equal(t, "stealth", Stealth.Value)

	// Verify all constants are unique
	refs := []*core.Ref{
		Athletics, Acrobatics, SleightOfHand, Stealth,
		Arcana, History, Investigation, Nature, Religion,
		AnimalHandling, Insight, Medicine, Perception, Survival,
		Deception, Intimidation, Performance, Persuasion,
	}

	seen := make(map[string]bool)
	for _, ref := range refs {
		key := ref.Value
		assert.False(t, seen[key], "duplicate skill ref: %s", key)
		seen[key] = true
	}

	// Should have all 18 skills
	assert.Equal(t, 18, len(refs))
}
