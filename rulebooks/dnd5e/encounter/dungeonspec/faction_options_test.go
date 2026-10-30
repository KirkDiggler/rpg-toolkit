// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// faction_options_test.go is A SIDE'S LINES (rpg-project#718, Frank's list
// S4): `options` on a faction, written once and spoken by whichever member is
// addressed, laid under each member's own lines the way `on:` is — nearest
// wins the key wholesale.
//
// Every compile scene asserts what a MEMBER carries, because that is what a
// host hands the encounter; a document that parses but leaves the survivor
// mute would pass a shape test and fail the author.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// bandLines is a faction with two lines — the survivors' yield and refuse.
const bandLines = `factions:
  - id: band
    options:
      - id: yield
        say: Yield and live.
        check: [{ ability: persuasion, dc: 10 }]
        success: [{ say: We yield., fact: band-yielded }]
      - id: refuse
        say: Throw down your arms.
        check: [{ ability: intimidation, dc: 15 }]
        failure: [{ say: We fight on. }]
`

// FactionOptionsSuite is the compile and the grammar of a faction's lines.
type FactionOptionsSuite struct {
	suite.Suite
}

func TestFactionOptionsSuite(t *testing.T) { suite.Run(t, new(FactionOptionsSuite)) }

// v4 is a single room with the band's lines and the given bindings.
func (s *FactionOptionsSuite) v4(root, bindings string) string {
	return v4With(root, "    monsterBindings:\n"+bindings)
}

// lineIDs is the ids of a member's compiled lines, in authored order.
func lineIDs(options []encounter.Option) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		out = append(out, o.ID)
	}

	return out
}

// TestAMemberWithNoLinesSpeaksTheFactions is the point of the key: a member
// that wrote nothing carries every one of its side's lines, rows and all.
func (s *FactionOptionsSuite) TestAMemberWithNoLinesSpeaksTheFactions() {
	compiled := loadSource(s.T(), s.v4(bandLines,
		"      goblin-1: { faction: band }\n      goblin-2: { faction: band }\n"))

	for _, id := range []string{"goblin-1", "goblin-2"} {
		options := monsterNamed(s.T(), compiled, id).Options
		s.Equal([]string{"yield", "refuse"}, lineIDs(options), "%s speaks the band's lines", id)
		s.Require().Len(options[0].Success, 1)
		s.Equal(encounter.FactID("band-yielded"), options[0].Success[0].Fact,
			"%s's yield teaches what the faction wrote", id)
		s.Equal([]encounter.CheckApproach{{Ability: "persuasion", DC: 10}}, options[0].Check)
	}
}

// TestAMembersOwnLinesWinWholesale is the layering: a member that writes one
// line of its own speaks that line alone — no merge by id, exactly `on:`'s
// nearest-wins-the-key.
func (s *FactionOptionsSuite) TestAMembersOwnLinesWinWholesale() {
	compiled := loadSource(s.T(), s.v4(bandLines,
		"      goblin-1: { faction: band }\n"+
			"      goblin-2:\n"+
			"        faction: band\n"+
			"        options:\n"+
			"          - { id: plead, say: Spare me., check: [{ ability: persuasion, dc: 8 }] }\n"))

	s.Equal([]string{"yield", "refuse"}, lineIDs(monsterNamed(s.T(), compiled, "goblin-1").Options))
	s.Equal([]string{"plead"}, lineIDs(monsterNamed(s.T(), compiled, "goblin-2").Options),
		"its own lines replace the faction's entirely")
}

// TestAMemberOfAnotherSideSpeaksNone: the lines are the faction's, and a
// creature on no declared side finds nothing to speak.
func (s *FactionOptionsSuite) TestAMemberOfAnotherSideSpeaksNone() {
	compiled := loadSource(s.T(), s.v4(bandLines, "      goblin-1: { faction: band }\n"))

	s.Empty(monsterNamed(s.T(), compiled, "goblin-2").Options,
		"goblin-2 is on the unauthored `monsters` side, which wrote no lines")
}

// TestTheVersionTwoDialectLaysThemTheSameWay: the faction shape is shared by
// both dialects, so a v2 placement inherits its side's lines by the same rule.
func (s *FactionOptionsSuite) TestTheVersionTwoDialectLaysThemTheSameWay() {
	compiled := loadSource(s.T(), `
version: 2
key: v2-band
orientation: pointy
void: opaque
regions:
  - id: yard
    name: Yard
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0],[2,0],[3,0]]
start: [0, 0]
factions:
  - id: band
    options:
      - { id: yield, say: Yield and live., check: [{ ability: persuasion, dc: 10 }] }
dispositions:
  - { between: [band, party], stance: hostile, until: { down: leader } }
place:
  - { id: leader, ref: "dnd5e:monsters:goblin", faction: band, at: [2, 0] }
  - { id: follower, ref: "dnd5e:monsters:goblin", faction: band, at: [3, 0] }
`)

	s.Equal([]string{"yield"}, lineIDs(monsterNamed(s.T(), compiled, "leader").Options))
	s.Equal([]string{"yield"}, lineIDs(monsterNamed(s.T(), compiled, "follower").Options),
		"the survivor speaks the line the leader did")
}

// TestAFactTheFactionsLineTeachesIsAFactThisFileSets: a `when` pattern that
// only a faction's line can satisfy is legal, because the facts universe
// counts the faction's rows.
func (s *FactionOptionsSuite) TestAFactTheFactionsLineTeachesIsAFactThisFileSets() {
	loadSource(s.T(), s.v4(bandLines,
		"      goblin-1:\n"+
			"        faction: band\n"+
			"        on:\n"+
			"          time:\n"+
			"            - { when: { fact: /band-.*/ }, hold: {} }\n"))
}

// TestABundleRowInAFactionsLineIsResolved: `use:` in a faction's verdict is
// resolved as it is in a binding's, so the member carries the bundle's words.
func (s *FactionOptionsSuite) TestABundleRowInAFactionsLineIsResolved() {
	compiled := loadSource(s.T(), s.v4(`bundles:
  give-up:
    rows:
      - { say: We yield., fact: band-yielded }
factions:
  - id: band
    options:
      - id: yield
        say: Yield and live.
        check: [{ ability: persuasion, dc: 10 }]
        success: [{ use: give-up }]
`, "      goblin-1: { faction: band }\n"))

	options := monsterNamed(s.T(), compiled, "goblin-1").Options
	s.Require().Len(options, 1)
	s.Require().NotEmpty(options[0].Success)
	s.Equal(encounter.FactID("band-yielded"), options[0].Success[0].Rows[0].Fact,
		"the bundle's row is what the line answers")
}

// TestAFactionsLinesAreJudgedByTheOneOptionGrammar is (b): every refusal a
// binding's lines earn, at the faction's own path.
func (s *FactionOptionsSuite) TestAFactionsLinesAreJudgedByTheOneOptionGrammar() {
	cases := []struct {
		name    string
		options string
		path    string
		message string
	}{
		{
			name: "two lines under one id",
			options: "      - { id: yield, say: A., check: [{ ability: persuasion, dc: 10 }] }\n" +
				"      - { id: yield, say: B., check: [{ ability: persuasion, dc: 10 }] }\n",
			path:    "factions[0].options[1].id",
			message: `option "yield" is already declared at options[0]`,
		},
		{
			name:    "no check",
			options: "      - { id: yield, say: A., check: [] }\n",
			path:    "factions[0].options[0].check",
		},
		{
			name:    "no line",
			options: "      - { id: yield, check: [{ ability: persuasion, dc: 10 }] }\n",
			path:    "factions[0].options[0].say",
			message: "the option does not say what the character says",
		},
		{
			name:    "a requires that is neither a fact nor a holding",
			options: "      - { id: yield, say: A., check: [{ ability: persuasion, dc: 10 }], requires: { down: goblin-1 } }\n",
			path:    "factions[0].options[0].requires",
			message: "an option requires { fact: <id> }, { holds: <prop id> }, or { not: } one of them, and `down` is none of them",
		},
		{
			name:    "a verdict written and left empty",
			options: "      - { id: yield, say: A., check: [{ ability: persuasion, dc: 10 }], success: [] }\n",
			path:    "factions[0].options[0].success",
			message: "`success` lists nothing — omit it, or write what is answered",
		},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			errs := refusals(s.T(), s.v4("factions:\n  - id: band\n    options:\n"+c.options,
				"      goblin-1: { faction: band }\n"))
			if c.message == "" {
				requireRefusedAt(s.T(), errs, c.path)
				return
			}
			requireExactDefect(s.T(), errs, c.path, c.message)
		})
	}
}

// TestIdsAreUniqueWithinTheFactionNotAcrossTheFile: a member's own line may
// share an id with its faction's — the member's replaces the faction's list
// wholesale, so the two never stand side by side.
func (s *FactionOptionsSuite) TestIdsAreUniqueWithinTheFactionNotAcrossTheFile() {
	loadSource(s.T(), s.v4(bandLines,
		"      goblin-1:\n"+
			"        faction: band\n"+
			"        options:\n"+
			"          - { id: yield, say: Never., check: [{ ability: persuasion, dc: 20 }] }\n"))
}

// TestANullOptionsIsRefused: `options:` with nothing under it on a faction is
// an author who emptied the list, not one who wrote none.
func (s *FactionOptionsSuite) TestANullOptionsIsRefused() {
	_, err := dungeonspec.Load([]byte(s.v4("factions:\n  - id: band\n    options:\n",
		"      goblin-1: { faction: band }\n")))
	s.Require().Error(err)
	s.Contains(fmt.Sprint(err), "factions[0].options")
}

// TestAnEmptyOptionsListIsRefusedNotReadAsAbsent: `options` is a layered key,
// so `options: []` on a member would otherwise silently speak its faction's
// lines. Refused by name at its own path — on a creature, a prop and a
// faction alike, the sentence an empty verdict earns.
func (s *FactionOptionsSuite) TestAnEmptyOptionsListIsRefusedNotReadAsAbsent() {
	const message = "`options` lists nothing — omit it, or write a line it can be spoken to with"
	cases := []struct {
		name, root, gameplay, path string
	}{
		{
			name: "a creature", root: bandLines,
			gameplay: "    monsterBindings:\n      goblin-1: { faction: band, options: [] }\n",
			path:     "room.room.monsterBindings.goblin-1.options",
		},
		{
			name:     "a prop",
			gameplay: "    propBindings:\n      table: { options: [] }\n",
			path:     "room.room.propBindings.table.options",
		},
		{
			name: "a faction", root: "factions:\n  - { id: band, options: [] }\n",
			gameplay: "    monsterBindings:\n      goblin-1: { faction: band }\n",
			path:     "factions[0].options",
		},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			requireExactDefect(s.T(), refusals(s.T(), v4With(c.root, c.gameplay)), c.path, message)
		})
	}

	s.Run("a v2 placement", func() {
		errs := refusals(s.T(), `
version: 2
key: v2-empty
orientation: pointy
void: opaque
regions:
  - { id: yard, name: Yard, archetype: crypt, lighting: { intensity: 1 }, cells: [[[0,0],[1,0],[2,0]]] }
start: [0, 0]
place:
  - { id: goblin, ref: "dnd5e:monsters:goblin", at: [2, 0], options: [] }
`)
		requireExactDefect(s.T(), errs, "place[0].options", message)
	})
}
