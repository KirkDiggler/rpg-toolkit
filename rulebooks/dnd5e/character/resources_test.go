package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type ResourcesTestSuite struct {
	suite.Suite
}

func TestResourcesSuite(t *testing.T) {
	suite.Run(t, new(ResourcesTestSuite))
}

func (s *ResourcesTestSuite) TestEvaluateSimpleExpression() {
	testCases := []struct {
		name     string
		expr     string
		expected int
		hasError bool
	}{
		{
			name:     "simple number",
			expr:     "5",
			expected: 5,
		},
		{
			name:     "addition",
			expr:     "3 + 2",
			expected: 5,
		},
		{
			name:     "subtraction",
			expr:     "10 - 3",
			expected: 7,
		},
		{
			name:     "multiplication",
			expr:     "4 * 3",
			expected: 12,
		},
		{
			name:     "division",
			expr:     "15 / 3",
			expected: 5,
		},
		{
			name:     "complex expression",
			expr:     "2 + 3 * 4",
			expected: 14,
		},
		{
			name:     "negative number",
			expr:     "-5",
			expected: -5,
		},
		{
			name:     "min function",
			expr:     "min(5, 3)",
			expected: 3,
		},
		{
			name:     "max function",
			expr:     "max(2, 7)",
			expected: 7,
		},
		{
			name:     "max with multiple args",
			expr:     "max(1, 5, 3)",
			expected: 5,
		},
		{
			name:     "division by zero",
			expr:     "5 / 0",
			hasError: true,
		},
		{
			name:     "consecutive operators",
			expr:     "10 + -3",
			hasError: true,
		},
		{
			name:     "double negative",
			expr:     "5--3",
			hasError: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result, err := evaluateSimpleExpression(tc.expr)
			if tc.hasError {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tc.expected, result)
			}
		})
	}
}

func (s *ResourcesTestSuite) TestEvaluateResourceFormula() {
	abilityScores := shared.AbilityScores{
		abilities.STR: 14, // +2 modifier
		abilities.DEX: 12, // +1 modifier
		abilities.CON: 16, // +3 modifier
		abilities.INT: 10, // +0 modifier
		abilities.WIS: 13, // +1 modifier
		abilities.CHA: 18, // +4 modifier
	}

	testCases := []struct {
		name     string
		formula  string
		level    int
		expected int
	}{
		{
			name:     "just level",
			formula:  "level",
			level:    5,
			expected: 5,
		},
		{
			name:     "ability modifier",
			formula:  "charisma_modifier",
			level:    1,
			expected: 4,
		},
		{
			name:     "level plus modifier",
			formula:  "level + constitution_modifier",
			level:    3,
			expected: 6, // 3 + 3
		},
		{
			name:     "multiplied value",
			formula:  "2 * level",
			level:    4,
			expected: 8,
		},
		{
			name:     "complex formula",
			formula:  "1 + min(level, charisma_modifier)",
			level:    2,
			expected: 3, // 1 + min(2, 4) = 1 + 2
		},
		{
			name:     "max with level",
			formula:  "max(1, wisdom_modifier)",
			level:    1,
			expected: 1, // max(1, 1)
		},
		{
			name:     "short form modifier",
			formula:  "str_modifier + 1",
			level:    1,
			expected: 3, // 2 + 1
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := evaluateResourceFormula(tc.formula, tc.level, abilityScores)
			s.Equal(tc.expected, result)
		})
	}
}

func (s *ResourcesTestSuite) TestInitializeClassResources() {
	abilityScores := shared.AbilityScores{
		abilities.STR: 16, // +3
		abilities.DEX: 14, // +2
		abilities.CON: 15, // +2
		abilities.INT: 10, // +0
		abilities.WIS: 13, // +1
		abilities.CHA: 8,  // -1
	}

	s.Run("barbarian rage", func() {
		barbarianClass := &class.Data{
			Resources: []class.ResourceData{
				{
					Type:       shared.ClassResourceRage,
					Name:       "Rage",
					MaxFormula: "", // Uses table
					UsesPerLevel: map[int]int{
						1: 2, 2: 2, 3: 3, 4: 3, 5: 3, 6: 4,
					},
					Resets: shared.ResetTypeLongRest,
				},
			},
		}

		// Level 1 barbarian
		resources := initializeClassResources(barbarianClass, 1, abilityScores)
		s.Len(resources, 1)
		rage := resources[shared.ClassResourceRage]
		s.Equal("Rage", rage.Name)
		s.Equal(2, rage.Max)
		s.Equal(2, rage.Current)
		s.Equal(shared.ResetTypeLongRest, rage.Resets)

		// Level 6 barbarian
		resources = initializeClassResources(barbarianClass, 6, abilityScores)
		rage = resources[shared.ClassResourceRage]
		s.Equal(4, rage.Max)
	})

	s.Run("monk ki points", func() {
		monkClass := &class.Data{
			Resources: []class.ResourceData{
				{
					Type:       shared.ClassResourceKiPoints,
					Name:       "Ki Points",
					MaxFormula: "level",
					Resets:     shared.ResetTypeShortRest,
				},
			},
		}

		// Level 5 monk
		resources := initializeClassResources(monkClass, 5, abilityScores)
		s.Len(resources, 1)
		ki := resources[shared.ClassResourceKiPoints]
		s.Equal("Ki Points", ki.Name)
		s.Equal(5, ki.Max)
		s.Equal(5, ki.Current)
		s.Equal(shared.ResetTypeShortRest, ki.Resets)
	})

	s.Run("sorcerer sorcery points", func() {
		sorcererClass := &class.Data{
			Resources: []class.ResourceData{
				{
					Type:       shared.ClassResourceSorceryPoints,
					Name:       "Sorcery Points",
					MaxFormula: "level",
					Resets:     shared.ResetTypeLongRest,
				},
			},
		}

		// Level 3 sorcerer
		resources := initializeClassResources(sorcererClass, 3, abilityScores)
		s.Len(resources, 1)
		sp := resources[shared.ClassResourceSorceryPoints]
		s.Equal("Sorcery Points", sp.Name)
		s.Equal(3, sp.Max)
	})

	s.Run("paladin channel divinity", func() {
		paladinClass := &class.Data{
			Resources: []class.ResourceData{
				{
					Type:       shared.ClassResourceChannelDivinity,
					Name:       "Channel Divinity",
					MaxFormula: "1", // Most classes get 1 use
					Resets:     shared.ResetTypeShortRest,
				},
			},
		}

		resources := initializeClassResources(paladinClass, 3, abilityScores)
		s.Len(resources, 1)
		cd := resources[shared.ClassResourceChannelDivinity]
		s.Equal(1, cd.Max)
	})

	s.Run("hypothetical ability-based resource", func() {
		customClass := &class.Data{
			Resources: []class.ResourceData{
				{
					Type:       shared.ClassResourceUnspecified, // Custom resource
					Name:       "Focus Points",
					MaxFormula: "1 + wisdom_modifier",
					Resets:     shared.ResetTypeShortRest,
				},
			},
		}

		resources := initializeClassResources(customClass, 1, abilityScores)
		s.Len(resources, 1)
		fp := resources[shared.ClassResourceUnspecified]
		s.Equal(2, fp.Max) // 1 + 1 (wisdom modifier)
	})
}

func (s *ResourcesTestSuite) TestInitializeSpellSlots() {
	s.Run("wizard spell slots", func() {
		wizardClass := &class.Data{
			Spellcasting: &class.SpellcastingData{
				Ability: "Intelligence",
				SpellSlots: map[int][]int{
					1: {2},             // Level 1: 2 first-level slots
					2: {3},             // Level 2: 3 first-level slots
					3: {4, 2},          // Level 3: 4 first, 2 second
					5: {4, 3, 2},       // Level 5: 4 first, 3 second, 2 third
					9: {4, 3, 3, 3, 1}, // Level 9: includes 5th level slot
				},
			},
		}

		// Level 1 wizard
		slots := initializeSpellSlots(wizardClass, 1)
		s.Len(slots, 1)
		s.Equal(2, slots[1].Max)
		s.Equal(0, slots[1].Used)

		// Level 3 wizard
		slots = initializeSpellSlots(wizardClass, 3)
		s.Len(slots, 2)
		s.Equal(4, slots[1].Max)
		s.Equal(2, slots[2].Max)

		// Level 9 wizard
		slots = initializeSpellSlots(wizardClass, 9)
		s.Len(slots, 5)
		s.Equal(4, slots[1].Max)
		s.Equal(3, slots[2].Max)
		s.Equal(3, slots[3].Max)
		s.Equal(3, slots[4].Max)
		s.Equal(1, slots[5].Max)
	})

	s.Run("non-spellcaster", func() {
		fighterClass := &class.Data{
			// No spellcasting data
		}

		slots := initializeSpellSlots(fighterClass, 5)
		s.Empty(slots)
	})

	s.Run("partial caster", func() {
		rangerClass := &class.Data{
			Spellcasting: &class.SpellcastingData{
				Ability: "Wisdom",
				SpellSlots: map[int][]int{
					2: {2},    // Level 2: 2 first-level slots
					3: {3},    // Level 3: 3 first-level slots
					5: {4, 2}, // Level 5: 4 first, 2 second
				},
			},
		}

		// Level 1 ranger (no spells yet)
		slots := initializeSpellSlots(rangerClass, 1)
		s.Empty(slots)

		// Level 2 ranger
		slots = initializeSpellSlots(rangerClass, 2)
		s.Len(slots, 1)
		s.Equal(2, slots[1].Max)

		// Level 5 ranger
		slots = initializeSpellSlots(rangerClass, 5)
		s.Len(slots, 2)
		s.Equal(4, slots[1].Max)
		s.Equal(2, slots[2].Max)
	})
}
