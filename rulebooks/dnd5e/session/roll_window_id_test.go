package session

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRollWindowBodyPreservesPresentationIDAndLegacyAbsence(t *testing.T) {
	for _, tc := range []struct{ name, extra, want string }{
		{name: "current", extra: `,"presentation_id":"opaque~roll"`, want: "opaque~roll"},
		{name: "legacy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(`{"audience":"fighter","offer":{"ref":"dnd5e:conditions:inspired","name":"Bardic Inspiration"},"roll":8,"total":12` + tc.extra + `}`)
			body, ok := rollWindowOpenedBody(payload).(RollWindowOpenedBody)
			require.True(t, ok)
			require.Equal(t, tc.want, body.PresentationID)
			require.Equal(t, 8, body.Roll)
			require.Equal(t, 12, body.Total)
		})
	}
}

// TestRollWindowBodyCarriesTheRollBehindTheQuestion is R5 at the decoder. The
// window is where an untrained roll is FIRST seen — the player is being asked
// whether to spend a die on it — and the body had nowhere to put the pair of
// faces until this field existed, so the proto's own calculation field was
// never filled.
func TestRollWindowBodyCarriesTheRollBehindTheQuestion(t *testing.T) {
	payload := []byte(`{"audience":"fighter","offer":{"ref":"dnd5e:conditions:inspired","name":"Bardic Inspiration"},` +
		`"roll":7,"total":9,"calculation":{"components":[` +
		`{"source":{"ref":"dnd5e:skills:persuasion","name":"Persuasion","source_id":"fighter"},` +
		`"dice":{"notation":"2d20","die_size":20,"original_rolls":[7,18],"final_rolls":[7,18],` +
		`"kept_indices":[0],"subtotal":7,"keep":{"rule":"disadvantage","imposed":[` +
		`{"ref":"dnd5e:rules:untrained","name":"Untrained","label":"rule","source_id":"fighter"}]}}},` +
		`{"source":{"ref":"dnd5e:abilities:charisma","name":"Charisma"},"modifier":2}],"total":9}}`)

	body, ok := rollWindowOpenedBody(payload).(RollWindowOpenedBody)
	require.True(t, ok)
	require.NotNil(t, body.Calculation)
	require.Equal(t, 9, body.Calculation.Total)

	keep := body.Calculation.Components[0].Dice.Keep
	require.NotNil(t, keep)
	require.Equal(t, KeepDisadvantage, keep.Rule)
	require.Len(t, keep.Imposed, 1)
	require.Equal(t, "Untrained", keep.Imposed[0].Name)
	require.Equal(t, "fighter", keep.Imposed[0].SourceID)
}

// TestRollWindowRefusesArithmeticThatDisagreesWithIt: the two scalars are what
// the player is shown, and a calculation that could not have produced them
// would render a different roll beside the same question. The body declines to
// type rather than show both.
func TestRollWindowRefusesArithmeticThatDisagreesWithIt(t *testing.T) {
	for _, tc := range []struct{ name, calculation string }{
		{
			name: "the total disagrees",
			calculation: `{"components":[{"source":{"ref":"dnd5e:skills:persuasion","name":"Persuasion","source_id":"fighter"},` +
				`"dice":{"notation":"1d20","die_size":20,"original_rolls":[7],"final_rolls":[7],"subtotal":7}}],"total":7}`,
		},
		{
			name: "the kept face disagrees with the rule",
			calculation: `{"components":[{"source":{"ref":"dnd5e:skills:persuasion","name":"Persuasion","source_id":"fighter"},` +
				`"dice":{"notation":"2d20","die_size":20,"original_rolls":[7,18],"final_rolls":[7,18],` +
				`"kept_indices":[1],"subtotal":18,"keep":{"rule":"disadvantage","imposed":[` +
				`{"ref":"dnd5e:rules:untrained","name":"Untrained","source_id":"fighter"}]}}}],"total":18}`,
		},
		{
			name: "a rule brought by nobody",
			calculation: `{"components":[{"source":{"ref":"dnd5e:skills:persuasion","name":"Persuasion","source_id":"fighter"},` +
				`"dice":{"notation":"2d20","die_size":20,"original_rolls":[7,18],"final_rolls":[7,18],` +
				`"kept_indices":[0],"subtotal":7,"keep":{"rule":"disadvantage","imposed":[` +
				`{"ref":"dnd5e:rules:untrained","name":"Untrained"}]}}},` +
				`{"source":{"ref":"dnd5e:abilities:charisma","name":"Charisma"},"modifier":2}],"total":9}`,
		},
		{
			name: "a rule nobody has heard of",
			calculation: `{"components":[{"source":{"ref":"dnd5e:skills:persuasion","name":"Persuasion","source_id":"fighter"},` +
				`"dice":{"notation":"2d20","die_size":20,"original_rolls":[7,18],"final_rolls":[7,18],` +
				`"kept_indices":[0],"subtotal":7,"keep":{"rule":"lucky","imposed":[` +
				`{"ref":"dnd5e:rules:untrained","name":"Untrained","source_id":"fighter"}]}}},` +
				`{"source":{"ref":"dnd5e:abilities:charisma","name":"Charisma"},"modifier":2}],"total":9}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(`{"audience":"fighter","offer":{"ref":"dnd5e:conditions:inspired","name":"Bardic Inspiration"},` +
				`"roll":7,"total":9,"calculation":` + tc.calculation + `}`)

			require.Nil(t, rollWindowOpenedBody(payload))
		})
	}
}
