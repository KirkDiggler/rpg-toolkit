package damage

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// DiceTerm is one signed dice group in a damage expression.
type DiceTerm struct {
	Dice string `json:"dice"`
	Sign int    `json:"sign"`
}

// Expression is a parsed, normalized signed dice expression.
type Expression struct {
	Terms     []DiceTerm
	FlatBonus int
	Notation  string
}

// ParseExpression parses dice groups joined by plus or minus, with an optional trailing bonus.
func ParseExpression(input string) (Expression, error) {
	notation := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, input)

	if notation == "" {
		return Expression{}, invalidExpression()
	}

	var expression Expression
	position := 0
	dice, next, ok := parseDice(notation, position)
	if !ok {
		return Expression{}, invalidExpression()
	}
	expression.Terms = append(expression.Terms, DiceTerm{Dice: dice, Sign: 1})
	position = next

	for position < len(notation) {
		sign := 1
		switch notation[position] {
		case '+':
			position++
		case '-':
			sign = -1
			position++
		default:
			return Expression{}, invalidExpression()
		}

		if dice, next, ok = parseDice(notation, position); ok {
			expression.Terms = append(expression.Terms, DiceTerm{Dice: dice, Sign: sign})
			position = next
			continue
		}

		bonus, next, ok := parseWholeNumber(notation, position)
		if !ok || next != len(notation) {
			return Expression{}, invalidExpression()
		}
		if sign < 0 {
			bonus = -bonus
		}
		expression.FlatBonus = bonus
		position = next
	}

	expression.Notation = notation
	return expression, expression.Validate()
}

// Validate verifies the structural constraints of an expression.
func (e Expression) Validate() error {
	if len(e.Terms) == 0 {
		return invalidExpression()
	}
	for _, term := range e.Terms {
		if term.Sign != 1 && term.Sign != -1 {
			return invalidExpression()
		}
		if _, next, ok := parseDice(term.Dice, 0); !ok || next != len(term.Dice) {
			return invalidExpression()
		}
	}
	return nil
}

func parseDice(input string, position int) (string, int, bool) {
	start := position
	count, next, ok := parseWholeNumber(input, position)
	if !ok || count <= 0 || next >= len(input) || input[next] != 'd' {
		return "", position, false
	}
	sides, next, ok := parseWholeNumber(input, next+1)
	if !ok || sides <= 0 {
		return "", position, false
	}
	return input[start:next], next, true
}

func parseWholeNumber(input string, position int) (int, int, bool) {
	start := position
	for position < len(input) && input[position] >= '0' && input[position] <= '9' {
		position++
	}
	if start == position {
		return 0, start, false
	}
	value, err := strconv.Atoi(input[start:position])
	if err != nil {
		return 0, start, false
	}
	return value, position, true
}

func invalidExpression() error {
	return rpgerr.New(rpgerr.CodeInvalidArgument, "invalid damage expression")
}
