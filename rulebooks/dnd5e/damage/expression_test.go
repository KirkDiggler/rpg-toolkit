package damage_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/stretchr/testify/require"
)

func TestParseExpression(t *testing.T) {
	tests := []struct {
		input string
		want  damage.Expression
	}{
		{
			input: "1d8+4",
			want: damage.Expression{
				Terms:     []damage.DiceTerm{{Dice: "1d8", Sign: 1}},
				FlatBonus: 4,
				Notation:  "1d8+4",
			},
		},
		{
			input: "2d6-2",
			want: damage.Expression{
				Terms:     []damage.DiceTerm{{Dice: "2d6", Sign: 1}},
				FlatBonus: -2,
				Notation:  "2d6-2",
			},
		},
		{
			input: "1d6+1d4",
			want: damage.Expression{
				Terms:    []damage.DiceTerm{{Dice: "1d6", Sign: 1}, {Dice: "1d4", Sign: 1}},
				Notation: "1d6+1d4",
			},
		},
		{
			input: "1d6-1d4+2",
			want: damage.Expression{
				Terms:     []damage.DiceTerm{{Dice: "1d6", Sign: 1}, {Dice: "1d4", Sign: -1}},
				FlatBonus: 2,
				Notation:  "1d6-1d4+2",
			},
		},
		{
			input: "2d6 + 1d4 - 3",
			want: damage.Expression{
				Terms:     []damage.DiceTerm{{Dice: "2d6", Sign: 1}, {Dice: "1d4", Sign: 1}},
				FlatBonus: -3,
				Notation:  "2d6+1d4-3",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := damage.ParseExpression(test.input)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestParseExpressionRejectsInvalidNotation(t *testing.T) {
	for _, input := range []string{"0d6", "1d0", "1d6*2", "(1d6)", "1d6/2", "1d6++2", "2"} {
		t.Run(input, func(t *testing.T) {
			_, err := damage.ParseExpression(input)
			require.Error(t, err)
		})
	}
}
