// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/sound"
)

// group_test.go is THE NESTED TABLE (rpg-project#670, R12–R15): a row whose
// answer is a table. The outer die picks the group by its one load, the inner
// die picks inside, and the record carries the path and every level's
// verdicts. It also pins [behavior.Name], the literal-or-pattern value a
// `heard` kind and a `fact` take (R16, rpg-project#684).

type GroupSuite struct {
	suite.Suite
}

func TestGroupSuite(t *testing.T) { suite.Run(t, new(GroupSuite)) }

// queueDie answers the faces it was given, in order, and records every die
// size it was asked for, so a test can read each level's die.
type queueDie struct {
	faces []int
	sizes []int
}

func (q *queueDie) Roll(_ context.Context, size int) (int, error) {
	q.sizes = append(q.sizes, size)
	face := 1
	if len(q.faces) > 0 {
		face, q.faces = q.faces[0], q.faces[1:]
	}
	if face > size {
		return size, nil
	}

	return face, nil
}

var lost = []behavior.When{{Enemy: behavior.EnemyLost}}

// guesser is the two-row left/right branch of the design: a group whose rows
// are both eligible on a lost trail.
func guesser(weight int) behavior.Answer {
	return behavior.Answer{Weight: weight, From: "guesser", Rows: []behavior.Answer{
		{Weight: 1, When: lost, Search: &behavior.Search{Distance: 6, Heading: behavior.HeadingLeft}},
		{Weight: 1, When: lost, Search: &behavior.Search{Distance: 6, Heading: behavior.HeadingRight}},
	}}
}

// heardRow is a one-row `heard` answer eligible beside the guesser.
var heardRow = behavior.Answer{Weight: 1,
	When:   []behavior.When{{Heard: &behavior.HeardWhen{Kind: behavior.Name{Literal: "clang"}, Within: 2}}},
	Toward: &behavior.Selector{Word: behavior.SelectorSound}}

// lostAndHearing is a creature on a lost trail that just heard a clang.
var lostAndHearing = behavior.Facts{CanMove: true, EnemyLost: true, Now: 5,
	Sounds: []sound.Sound{{Kind: "clang", At: 5}}}

func (s *GroupSuite) pick(table []behavior.Answer, temper behavior.Temper, facts behavior.Facts,
	die behavior.Die) *behavior.PickOutput {
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: behavior.Table{behavior.KeyTime: table},
		Temper: temper, Facts: facts, Die: die})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	return out
}

// TestAGroupLoadsOnceAgainstASiblingRow is N3, the reason for the shape: a
// two-row branch eligible beside a one-row `heard` takes ONE share, not two.
// The die is 200 (100 + 100), not 300, so the faces split 50/50 between the
// group and the row; flattened, the branch would have taken 2:1.
func (s *GroupSuite) TestAGroupLoadsOnceAgainstASiblingRow() {
	table := []behavior.Answer{guesser(1), heardRow}

	die := &queueDie{faces: []int{100, 1}}
	out := s.pick(table, behavior.Temper{}, lostAndHearing, die)
	s.Equal(200, out.Of, "the group is one candidate of load 100 beside the row's 100")
	s.Equal([]int{0, 0}, out.Path, "face 100 is the last face of the group's share; inside, face 1 is left")
	s.Equal(behavior.HeadingLeft, out.Answer.Search.Heading)
	s.Equal(0, out.Entry, "Entry is the top of the path")
	s.Equal([]int{200, 200}, die.sizes, "the outer die over 200, the inner over the two rows' 100 each")

	die = &queueDie{faces: []int{101}}
	out = s.pick(table, behavior.Temper{}, lostAndHearing, die)
	s.Equal([]int{1}, out.Path, "face 101 is the first face of the heard row's share")
	s.NotNil(out.Answer.Toward)
	s.Equal([]int{200}, die.sizes, "a word row is not descended: one die")

	s.Require().Len(out.Candidates, 2, "the top level's candidates: the group and the row")
	s.Equal(behavior.Candidate{Entry: 0, Weight: 1, Percent: 100, Loaded: 100}, out.Candidates[0])
}

// TestTheInnerDieIsTheBranch is N4: inside the group, the die rolls over the
// group's own candidates, and the group's verdict carries that die.
func (s *GroupSuite) TestTheInnerDieIsTheBranch() {
	out := s.pick([]behavior.Answer{guesser(1), heardRow}, behavior.Temper{}, lostAndHearing,
		&queueDie{faces: []int{1, 101}})
	s.Equal([]int{0, 1}, out.Path)
	s.Equal(behavior.HeadingRight, out.Answer.Search.Heading)

	group := out.Rows[0]
	s.Equal(101, group.Roll, "the inner face is on the group's verdict")
	s.Equal(200, group.Of, "and so is the inner die")
	s.Equal([]behavior.RowVerdict{
		{Entry: 0, Held: true, Affordable: true, Weight: 1, Percent: 100, Loaded: 100},
		{Entry: 1, Held: true, Affordable: true, Weight: 1, Percent: 100, Loaded: 100},
	}, group.Rows, "a group's verdict holds its rows' verdicts")
	s.Zero(out.Rows[1].Roll, "a word row carries no inner die")
	s.Nil(out.Rows[1].Rows)
}

// TestAnEmptyGroupIsAbsent is R15: a group none of whose rows is a candidate
// is not a candidate, and neither is a group whose own `when` fails. Both say
// why on the verdict, with every inner row's verdict beneath.
func (s *GroupSuite) TestAnEmptyGroupIsAbsent() {
	notLost := behavior.Facts{CanMove: true, EnemyRemembered: true}
	hold := behavior.Answer{Weight: 1, Hold: true}

	out := s.pick([]behavior.Answer{guesser(5), hold}, behavior.Temper{}, notLost, &queueDie{})
	s.Equal([]int{1}, out.Path, "the group is absent, so the hold is the only candidate")
	s.Equal(100, out.Of, "the absent group loaded nothing")
	s.Equal(behavior.RowVerdict{Entry: 0, Held: true, Affordable: false, Rows: []behavior.RowVerdict{
		{Entry: 0, Held: false, Affordable: false},
		{Entry: 1, Held: false, Affordable: false},
	}}, out.Rows[0], "its own when held; nothing inside was a candidate")

	gated := guesser(5)
	gated.When = []behavior.When{{Fact: behavior.Name{Literal: "alarm-raised"}}}
	out = s.pick([]behavior.Answer{gated, hold}, behavior.Temper{}, lostAndHearing, &queueDie{})
	s.Equal([]int{1}, out.Path, "the group's own when gates the whole group")
	s.False(out.Rows[0].Held)
	s.True(out.Rows[0].Affordable, "its rows were candidates; its gate was not")

	only := s.pick([]behavior.Answer{guesser(5)}, behavior.Temper{}, notLost, &queueDie{})
	s.Equal(-1, only.Entry, "a time table whose one group is empty is a hold, not a pick into nothing")
	s.Empty(only.Path)
	s.True(only.Answer.Hold)
}

// TestADieOfOneAtEveryLevelReproducesTheFlatPick is the castle's promise: with
// the route host's die of 1, every level picks its first candidate, so a table
// regrouped in authored order picks what the flat table picked, under every
// band.
func (s *GroupSuite) TestADieOfOneAtEveryLevelReproducesTheFlatPick() {
	seen := []behavior.When{{Enemy: behavior.EnemySeen}}
	remembered := []behavior.When{{Enemy: behavior.EnemyRemembered}}
	none := []behavior.When{{Enemy: behavior.EnemyNone}}
	enemy := &behavior.Selector{Word: behavior.SelectorEnemy}
	chase := behavior.Answer{Weight: 1, When: seen, Toward: enemy}
	follow := behavior.Answer{Weight: 1, When: remembered, Toward: enemy}
	search := behavior.Answer{Weight: 1, When: lost, Search: &behavior.Search{Distance: 6, Heading: behavior.HeadingWent}}
	listen := heardRow
	home := behavior.Answer{Weight: 1, When: none, Toward: &behavior.Selector{At: &someCell}}

	flat := []behavior.Answer{chase, follow, search, listen, home}
	grouped := []behavior.Answer{
		chase,
		{Weight: 1, From: "pursuer", Rows: []behavior.Answer{follow, search}},
		{Weight: 1, From: "alarm-reactor", Rows: []behavior.Answer{
			{Weight: 1, Rows: []behavior.Answer{listen}},
		}},
		home,
	}
	clang := []sound.Sound{{Kind: "clang", At: 5}}
	for name, facts := range map[string]behavior.Facts{
		"seen":              {CanMove: true, EnemySeen: true},
		"remembered":        {CanMove: true, EnemyRemembered: true},
		"lost":              {CanMove: true, EnemyLost: true},
		"lost and hearing":  {CanMove: true, EnemyLost: true, Now: 5, Sounds: clang},
		"none":              {CanMove: true},
		"none and hearing":  {CanMove: true, Now: 5, Sounds: clang},
		"cannot move, held": {EnemyLost: true},
	} {
		s.Run(name, func() {
			want := s.pick(flat, behavior.Temper{}, facts, &queueDie{})
			got := s.pick(grouped, behavior.Temper{}, facts, &queueDie{})
			s.Equal(want.Answer, got.Answer)
		})
	}
}

// TestNestingTwoDeepPicksByPath is R14: a group inside a group descends one
// die per level, and the path names the row at every level.
func (s *GroupSuite) TestNestingTwoDeepPicksByPath() {
	hold := behavior.Answer{Weight: 1, Hold: true}
	inner := behavior.Answer{Weight: 1, From: "guesser", Rows: guesser(1).Rows}
	outer := behavior.Answer{Weight: 3, From: "pursuer", Rows: []behavior.Answer{hold, inner}}
	table := []behavior.Answer{hold, outer}
	facts := behavior.Facts{CanMove: true, EnemyLost: true}

	die := &queueDie{faces: []int{101, 101, 101}}
	out := s.pick(table, behavior.Temper{}, facts, die)
	s.Equal([]int{1, 1, 1}, out.Path, "outer group, then its group, then right")
	s.Equal(1, out.Entry)
	s.Equal(behavior.HeadingRight, out.Answer.Search.Heading)
	s.Equal([]int{400, 200, 200}, die.sizes, "hold 100 + group 300; hold 100 + group 100; left 100 + right 100")

	s.Equal(101, out.Rows[1].Roll)
	s.Equal(200, out.Rows[1].Of)
	s.Equal(101, out.Rows[1].Rows[1].Roll, "the second level's die sits on the inner group's verdict")
	s.Equal(200, out.Rows[1].Rows[1].Of)
	s.Zero(out.Rows[1].Rows[0].Roll)

	out = s.pick(table, behavior.Temper{}, facts, &queueDie{faces: []int{400, 1}})
	s.Equal([]int{1, 0}, out.Path, "a path stops at the first word row")
	s.True(out.Answer.Hold)
}

// TestAsLoadsTheGroupByItsWord is R13: a group has no word of its own, so it
// loads at weight × the percent of the word its `as` names, and at 100 when
// `as` is empty. A coward's pursuer `as: toward` loads at 50.
func (s *GroupSuite) TestAsLoadsTheGroupByItsWord() {
	coward := behavior.Temper{Word: "coward", Profile: cowardProfile}
	for _, tc := range []struct {
		as      string
		percent int
	}{{"", 100}, {"toward", 50}, {"attack", 50}, {"away", 300}, {"flee", 300}, {"hold", 100}} {
		s.Run("as "+tc.as, func() {
			group := guesser(2)
			group.As = tc.as
			out := s.pick([]behavior.Answer{group}, coward, behavior.Facts{CanMove: true, EnemyLost: true},
				&queueDie{})
			s.Equal(behavior.Candidate{Entry: 0, Weight: 2, Percent: tc.percent, Loaded: 2 * tc.percent},
				out.Candidates[0])
			s.Equal(50, out.Rows[0].Rows[0].Percent, "inside, each search row is loaded by its own word")
		})
	}

	group := guesser(2)
	group.As = "toward"
	soldier := s.pick([]behavior.Answer{group}, behavior.Temper{}, behavior.Facts{CanMove: true, EnemyLost: true},
		&queueDie{})
	s.Equal(200, soldier.Of, "a soldier's `as` is 100 whatever the word")
}

// TestAGroupIsATableAndCarriesNoWord is the group arm's door: a group row
// carries no word, no search, no line and no fact; it holds rows; it counts
// `as` one of the five temperament words or nothing; it nests no deeper than
// the guard; and a fault inside names the row it is in.
func (s *GroupSuite) TestAGroupIsATableAndCarriesNoWord() {
	enemy := &behavior.Selector{Word: behavior.SelectorEnemy}
	rows := guesser(1).Rows
	carrying := "a group carries %s; a group's answer is its rows, so the word goes on a row inside it: " +
		"behavior: group cannot be read"
	for _, tc := range []struct {
		name string
		row  behavior.Answer
		says string
	}{
		{"hold", behavior.Answer{Weight: 1, Rows: rows, Hold: true}, strings.Replace(carrying, "%s", "`hold`", 1)},
		{"attack", behavior.Answer{Weight: 1, Rows: rows, Attack: enemy}, strings.Replace(carrying, "%s", "`attack`", 1)},
		{"toward", behavior.Answer{Weight: 1, Rows: rows, Toward: enemy}, strings.Replace(carrying, "%s", "`toward`", 1)},
		{"away", behavior.Answer{Weight: 1, Rows: rows, Away: enemy}, strings.Replace(carrying, "%s", "`away`", 1)},
		{"flee", behavior.Answer{Weight: 1, Rows: rows, Flee: true}, strings.Replace(carrying, "%s", "`flee`", 1)},
		{"search", behavior.Answer{Weight: 1, Rows: rows,
			Search: &behavior.Search{Distance: 6, Heading: behavior.HeadingWent}},
			strings.Replace(carrying, "%s", "`search`", 1)},
		{"say and fact", behavior.Answer{Weight: 1, Rows: rows, Say: "halt", Fact: "alarm"},
			strings.Replace(carrying, "%s", "`say` and `fact`", 1)},
		{"no rows", behavior.Answer{Weight: 1, Rows: []behavior.Answer{}},
			"a group holds no rows; a group is a table, and a table with no rows can never be picked: " +
				"behavior: group cannot be read"},
		{"as with no rows", behavior.Answer{Weight: 1, As: "toward"},
			"a group holds no rows; a group is a table, and a table with no rows can never be picked: " +
				"behavior: group cannot be read"},
		{"as a word that is not a temperament's", behavior.Answer{Weight: 1, Rows: rows, As: "search"},
			"a group counts `as` \"search\"; it counts as one of attack, toward, away, flee or hold, " +
				"or as nothing: behavior: group cannot be read"},
		{"a fault inside", behavior.Answer{Weight: 1, Rows: []behavior.Answer{
			rows[0], {Weight: 1, When: lost, Search: &behavior.Search{Distance: 0, Heading: behavior.HeadingLeft}},
		}}, "row 2 of the group: `search` distance 0: a search walks at least one cell: " +
			"behavior: search cannot be read"},
	} {
		s.Run(tc.name, func() {
			err := tc.row.Validate()
			s.Require().Error(err)
			s.EqualError(err, tc.says)
		})
	}

	// THE DEPTH GUARD: a group nested MaxGroupDepth deep is read; one deeper
	// is refused.
	nest := func(depth int) behavior.Answer {
		row := behavior.Answer{Weight: 1, Hold: true}
		for i := 0; i < depth; i++ {
			row = behavior.Answer{Weight: 1, Rows: []behavior.Answer{row}}
		}

		return row
	}
	deepest := nest(behavior.MaxGroupDepth)
	s.NoError(deepest.Validate())
	deep := nest(behavior.MaxGroupDepth + 1)
	err := deep.Validate()
	s.Require().ErrorIs(err, behavior.ErrBadGroup)
	s.Contains(err.Error(), "groups nest at most 8 deep")

	read := guesser(1)
	s.NoError(read.Validate(), "a group of word rows is read")
}

// TestPickNeverDescendsPastTheGuard: Pick never validates, so a group built
// in Go past the depth guard (or a slice made to hold itself) is absent rather
// than recursed into forever.
func (s *GroupSuite) TestPickNeverDescendsPastTheGuard() {
	rows := []behavior.Answer{{Weight: 1}}
	rows[0].Rows = rows // a group that holds itself
	out := s.pick([]behavior.Answer{rows[0], {Weight: 1, Hold: true}}, behavior.Temper{}, canAct, &queueDie{})
	s.Equal([]int{1}, out.Path, "the self-holding group is absent")
}

// TestAGroupRowSurvivesTheMarshalledRow is the persisted tree: rows, `as`,
// `from` and params ride the row's JSON and come back unchanged, so a table
// saved and loaded never becomes a wordless row.
func (s *GroupSuite) TestAGroupRowSurvivesTheMarshalledRow() {
	group := guesser(2)
	group.As = "toward"
	group.Params = map[string]string{"distance": "8"}
	group.When = []behavior.When{{Not: &behavior.When{Fact: behavior.Name{Pattern: "alarm-.*"}}}}

	raw, err := json.Marshal(group)
	s.Require().NoError(err)
	var back behavior.Answer
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Equal(group, back)

	plain, err := json.Marshal(behavior.Answer{Weight: 1, Hold: true})
	s.Require().NoError(err)
	for _, field := range []string{"Rows", "As", "From", "Params"} {
		s.NotContains(string(plain), `"`+field+`"`, "a word row marshals as it did before groups existed")
	}
}

// --- Name ----------------------------------------------------------------------

// TestALiteralNameIsTheStringItWas is the golden's promise: a literal `fact`
// and a literal `heard` kind marshal exactly as the plain strings did, and a
// bare string decodes as a literal.
func (s *GroupSuite) TestALiteralNameIsTheStringItWas() {
	raw, err := json.Marshal(behavior.When{Fact: behavior.Name{Literal: "alarm-raised"}})
	s.Require().NoError(err)
	s.JSONEq(`{"Enemy":"","Deed":"","Within":0,"Fact":"alarm-raised"}`, string(raw))

	raw, err = json.Marshal(behavior.HeardWhen{Kind: behavior.Name{Literal: "clang"}, Within: 2})
	s.Require().NoError(err)
	s.JSONEq(`{"Kind":"clang","Within":2}`, string(raw))

	var back behavior.When
	s.Require().NoError(json.Unmarshal([]byte(`{"Fact":"alarm-raised"}`), &back))
	s.Equal(behavior.Name{Literal: "alarm-raised"}, back.Fact)

	raw, err = json.Marshal(behavior.When{Enemy: behavior.EnemySeen})
	s.Require().NoError(err)
	s.NotContains(string(raw), "Fact", "an unset name is omitted, as the empty string was")

	lit := behavior.Name{Literal: "clang"}
	s.True(lit.Matches("clang"))
	s.False(lit.Matches("clanging"), "a literal is the whole name")
	s.False(lit.Matches("bell"))
}

// TestAPatternMatchesASetOfAuthoredNames is R16: a pattern written `/…/`
// matches every authored name it covers, WHOLE, and nothing else.
func (s *GroupSuite) TestAPatternMatchesASetOfAuthoredNames() {
	var w behavior.When
	s.Require().NoError(json.Unmarshal([]byte(`{"Heard":{"Kind":"/clang|bell/","Within":3}}`), &w))
	s.Equal(behavior.Name{Pattern: "clang|bell"}, w.Heard.Kind)
	s.Equal("/clang|bell/", w.Heard.Kind.String(), "printed as written")

	raw, err := json.Marshal(w)
	s.Require().NoError(err)
	s.Contains(string(raw), `"Kind":"/clang|bell/"`, "marshalled back the way it was written")

	for name, want := range map[string]bool{"clang": true, "bell": true, "clanging": false, "voice": false,
		"bell-tower": false} {
		s.Equal(want, w.Heard.Kind.Matches(name), name)
	}

	table := []behavior.Answer{
		{Weight: 1, When: []behavior.When{w}, Toward: &behavior.Selector{Word: behavior.SelectorSound}},
		{Weight: 1, Hold: true},
	}
	bell := behavior.Facts{CanMove: true, Now: 4, Sounds: []sound.Sound{{Kind: "bell", At: 3}}}
	s.Equal([]int{0}, s.pick(table, behavior.Temper{}, bell, &queueDie{}).Path, "a bell is heard by the pattern")

	facts := []behavior.Answer{
		{Weight: 1, When: []behavior.When{{Fact: behavior.Name{Pattern: "guards-.*"}}}, Hold: true},
		{Weight: 1, Toward: &behavior.Selector{At: &someCell}},
	}
	s.Equal([]int{0}, s.pick(facts, behavior.Temper{}, behavior.Facts{CanMove: true,
		Known: []string{"guards-turned"}}, &queueDie{}).Path)
	s.Equal([]int{1}, s.pick(facts, behavior.Temper{}, behavior.Facts{CanMove: true,
		Known: []string{"old-guards-turned"}}, &queueDie{}).Path, "the pattern matches the whole name")
}

// TestANameIsOneOrTheOtherAndAPatternMustCompile is the Name's door.
func (s *GroupSuite) TestANameIsOneOrTheOtherAndAPatternMustCompile() {
	for _, tc := range []struct {
		name string
		when behavior.When
		says string
	}{
		{"a bad pattern", behavior.When{Heard: &behavior.HeardWhen{Kind: behavior.Name{Pattern: "clang("}, Within: 2}},
			"`heard` kind /clang(/ is not a pattern: error parsing regexp: missing closing ): `clang(`: " +
				"behavior: when cannot be read"},
		{"both forms", behavior.When{Fact: behavior.Name{Literal: "alarm", Pattern: "alarm-.*"}},
			"`fact` names both \"alarm\" and /alarm-.*/; a name is a literal or a pattern, not both: " +
				"behavior: when cannot be read"},
		{"a bad pattern inside a not", behavior.When{Not: &behavior.When{Fact: behavior.Name{Pattern: "["}}},
			"inside `not`: `fact` /[/ is not a pattern: error parsing regexp: missing closing ]: `[`: " +
				"behavior: when cannot be read"},
	} {
		s.Run(tc.name, func() {
			err := (&behavior.Answer{Weight: 1, Hold: true, When: []behavior.When{tc.when}}).Validate()
			s.Require().ErrorIs(err, behavior.ErrBadWhen)
			s.EqualError(err, tc.says)
		})
	}

	var w behavior.When
	s.Error(json.Unmarshal([]byte(`{"Fact":"//"}`), &w), "a pattern between empty slashes is refused")

	s.False(behavior.Name{Pattern: "clang("}.Matches("clang("), "a pattern that does not compile matches nothing")
}

// TestParseNameIsTheWrittenForm: the one reading of `/…/` against a bare
// string, which a blob that stores the name as a string reads back through.
func (s *GroupSuite) TestParseNameIsTheWrittenForm() {
	for written, want := range map[string]behavior.Name{
		"clang":        {Literal: "clang"},
		"/clang|bell/": {Pattern: "clang|bell"},
		"":             {},
		"/":            {Literal: "/"},
	} {
		got, err := behavior.ParseName(written)
		s.Require().NoError(err, written)
		s.Equal(want, got, written)
		s.Equal(written, got.String(), "the written form round-trips")
	}
	_, err := behavior.ParseName("//")
	s.Error(err)
}
