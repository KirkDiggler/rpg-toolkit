// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// dialect_test.go is WHAT THE AUTHOR IS ALLOWED TO SAY, and what happens when
// they say it wrong.
//
// Decoding and structural validation are the half of this compiler that does
// not care what a cell means. Whether a room becomes a rhombus or a cell set,
// whether the grid is pointy or flat — none of it changes whether `width: 0` is
// a dungeon, or whether a connector may name a room that was never declared.
// So this file holds the rules that are true before geometry, and holds them as
// REFUSALS: every one is a spec somebody could plausibly write, and the error
// says which field offended.
//
// # Loud, always, and never repaired
//
// There is no lenient mode and no defaulting anywhere below. An unknown key is
// a typo or a stale dialect, and silently dropping it is how an author's
// intention disappears between the file and the game — the old stack's decoder
// says the same thing in its own words and uses KnownFields(true) to mean it.
// The same reasoning is why `void`, `orientation`, `start` and a prop's two
// blocking answers are REQUIRED rather than derived from a theme word, a room
// archetype, or an omission: guessing any of them here would be
// rpg-toolkit#1033's forbidden default, relocated from the composition into the
// compiler where it is harder to see. tombsource_test.go lists what that cost
// the dialect it replaces.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

type DialectSuite struct {
	suite.Suite
}

func TestDialectSuite(t *testing.T) {
	suite.Run(t, new(DialectSuite))
}

// TestTheShippingTombDecodes is the acceptance case: everything below is about
// what gets refused, and this is the one that must not be.
func (s *DialectSuite) TestTheShippingTombDecodes() {
	spec, err := dungeonspec.Decode([]byte(tombYAML))
	s.Require().NoError(err)

	s.Equal("reference-tomb", spec.Key)
	s.Equal(8, spec.Height)
	s.Equal("opaque", spec.Void)
	s.Equal("pointy", spec.Orientation)
	s.Require().Len(spec.Rooms, 3)
	s.Equal([]int{6, 10, 12}, []int{spec.Rooms[0].Width, spec.Rooms[1].Width, spec.Rooms[2].Width})

	s.Require().NotNil(spec.Start, "an omitted start and [0,0] are different facts")
	s.Equal([2]int{1, 3}, *spec.Start, "and it says where the party comes in")

	s.Require().Len(spec.Rooms[1].Place, 8, "the hall's six props and two skeletons")
	s.Equal("lowest-health", deref(spec.Rooms[1].Place[6].Targeting))

	captain := spec.Rooms[2].Place[7]
	s.Equal("dnd5e:monsters:skeleton-captain", captain.Ref)
	s.True(captain.Boss, "the boss is a monster with a flag, not a key of its own")

	coffin := spec.Rooms[2].Place[0]
	s.Equal("dnd5e:props:coffin", coffin.Ref)
	s.Require().NotNil(coffin.BlocksLoS)
	s.False(*coffin.BlocksLoS, "authored false — you see over a coffin")
	s.Require().NotNil(coffin.BlocksMovement)
	s.True(*coffin.BlocksMovement, "and do not walk through one")

	s.Require().Len(spec.Connectors, 2)
	s.Nil(spec.Connectors[0].Locked, "entrance to hall stands open")
	s.Require().NotNil(spec.Connectors[1].Locked)
	s.Equal(12, spec.Connectors[1].Locked.DC)
	s.Equal("dex", spec.Connectors[1].Locked.Ability)

	s.Require().NoError(dungeonspec.Validate(spec), "and it survives validation")
}

// TestADungeonMustBeOneDocumentOfKnownKeys — the decoder's own refusals, before
// any rule about dungeons.
func (s *DialectSuite) TestADungeonMustBeOneDocumentOfKnownKeys() {
	for _, tc := range []struct {
		name, yaml, wants string
	}{
		{"empty", "", "empty"},
		{"unknown key", tombWith("height: 8", "height: 8\nlighting: dim"), "lighting"},
		{"a key the old dialect had", tombWith("height: 8", "height: 8\ntheme: crypt"), "theme"},
		{"misspelled key", tombWith("height: 8", "hieght: 8"), "hieght"},
		{"two documents", tombYAML + "\n---\nversion: 1\nkey: other\n", "one document"},
	} {
		s.Run(tc.name, func() {
			_, err := dungeonspec.Decode([]byte(tc.yaml))
			s.Require().Error(err)
			s.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wants))
		})
	}
}

// TestADungeonMustSayWhatItIs is the required-and-never-defaulted set. Each of
// these is a fact about THIS world that no rule could derive
// (rpg-toolkit#1033), and the compiler is not allowed to pick one.
func (s *DialectSuite) TestADungeonMustSayWhatItIs() {
	for _, tc := range []struct {
		name, yaml, wants string
	}{
		{"no version", withoutLine("version: 1"), "version"},
		{"a version this build does not speak", tombWith("version: 1", "version: 2"), "version"},
		{"no key", withoutLine("key: reference-tomb"), "key"},
		{"no height", withoutLine("height: 8"), "height"},
		{"zero height", tombWith("height: 8", "height: 0"), "height"},

		{"no void", withoutLine("void: opaque"), "void"},
		{"a void nobody knows", tombWith("void: opaque", "void: soup"), "void"},

		{"no orientation", withoutLine("orientation: pointy"), "orientation"},
		{"an orientation nobody knows", tombWith("orientation: pointy", "orientation: sideways"), "orientation"},

		{"no start", withoutLine("start: [1, 3]"), "start"},
		{"a start outside the dungeon", tombWith("start: [1, 3]", "start: [28, 3]"), "start"},
		{"a start below the floor", tombWith("start: [1, 3]", "start: [1, 8]"), "start"},
		{"a start on top of something", tombWith("start: [1, 3]", "start: [1, 1]"), "start"},
	} {
		s.Run(tc.name, func() {
			s.Require().Error(s.decodeAndValidate(tc.yaml), "this spec must be refused")
			s.Contains(strings.ToLower(s.decodeAndValidate(tc.yaml).Error()), tc.wants)
		})
	}
}

// TestARoomMustBeARoom — the room list's own rules.
func (s *DialectSuite) TestARoomMustBeARoom() {
	for _, tc := range []struct {
		name, yaml, wants string
	}{
		{"no rooms", roomless, "rooms"},
		{"empty room id", tombWith("  - id: hall", "  - id: \"\""), "id"},
		{"duplicate room id", tombWith("  - id: tomb", "  - id: hall"), "duplicate"},
		{"zero width", tombWith("    width: 10", "    width: 0"), "width"},
		{"negative width", tombWith("    width: 10", "    width: -4"), "width"},
	} {
		s.Run(tc.name, func() {
			err := s.decodeAndValidate(tc.yaml)
			s.Require().Error(err)
			s.Contains(strings.ToLower(err.Error()), tc.wants)
		})
	}
}

// TestAPlacementMustNameSomethingAndStandSomewhere.
//
// The ref's TYPE segment is what routes a placement — props become props,
// monsters become members — so a ref that does not parse is not a thing this
// compiler can place, and one whose type it does not recognise is not either.
// Neither is repaired by guessing.
func (s *DialectSuite) TestAPlacementMustNameSomethingAndStandSomewhere() {
	const pillar = `      - { ref: "dnd5e:props:pillar", at: [2, 2], blocks_movement: true,  blocks_los: true }`
	const solid = `blocks_movement: true, blocks_los: true`

	for _, tc := range []struct {
		name, yaml, wants string
	}{
		{"malformed ref", tombWith(pillar, `      - { ref: "pillar", at: [2, 2], `+solid+` }`), "ref"},
		{"unknown ref type", tombWith(pillar, `      - { ref: "dnd5e:vehicles:cart", at: [2, 2], `+solid+` }`), "vehicles"},
		{"x outside the room", tombWith(pillar, `      - { ref: "dnd5e:props:pillar", at: [10, 2], `+solid+` }`), "at"},
		{"y outside the room", tombWith(pillar, `      - { ref: "dnd5e:props:pillar", at: [2, 8], `+solid+` }`), "at"},
		{"negative cell", tombWith(pillar, `      - { ref: "dnd5e:props:pillar", at: [-1, 2], `+solid+` }`), "at"},
		{"two things on one cell", tombWith(pillar, pillar+"\n"+pillar), "same cell"},

		{"targeting on a prop", tombWith(pillar,
			`      - { ref: "dnd5e:props:pillar", at: [2, 2], `+solid+`, targeting: lowest-health }`), "targeting"},
		{"blocks_los on a monster", tombWith(
			`      - { ref: "dnd5e:monsters:skeleton", at: [5, 3], targeting: lowest-health }`,
			`      - { ref: "dnd5e:monsters:skeleton", at: [5, 3], blocks_los: false }`), "blocks"},
		{"a targeting nobody knows", tombWith("targeting: lowest-health", "targeting: whoever"), "whoever"},
	} {
		s.Run(tc.name, func() {
			err := s.decodeAndValidate(tc.yaml)
			s.Require().Error(err)
			s.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wants))
		})
	}
}

// TestAPropMustSayWhatItBlocks.
//
// The dialect this replaces read a missing flag as "blocks", and
// [encounter.PropInput]'s doc comment names that default as the thing it drops:
// "a blocker nobody declared is a wall nobody drew". Every combination is real
// content — a pillar blocks both, a coffin is seen over but not walked through,
// a curtain is the reverse, candles are neither — so an omission has nothing
// safe to stand for, and the composition refuses a nil pointer one layer down
// anyway. Defaulting it here would put rpg-toolkit#1033's forbidden default
// where the author cannot see it.
func (s *DialectSuite) TestAPropMustSayWhatItBlocks() {
	for _, tc := range []struct {
		name, yaml, wants string
	}{
		{"neither answer", tombWith(
			`      - { ref: "dnd5e:props:candles", at: [10, 5], blocks_movement: false, blocks_los: false }`,
			`      - { ref: "dnd5e:props:candles", at: [10, 5] }`), "blocks movement"},
		{"only movement", tombWith(
			`      - { ref: "dnd5e:props:candles", at: [10, 5], blocks_movement: false, blocks_los: false }`,
			`      - { ref: "dnd5e:props:candles", at: [10, 5], blocks_movement: false }`), "line of sight"},
		{"only sight", tombWith(
			`      - { ref: "dnd5e:props:candles", at: [10, 5], blocks_movement: false, blocks_los: false }`,
			`      - { ref: "dnd5e:props:candles", at: [10, 5], blocks_los: false }`), "blocks movement"},
	} {
		s.Run(tc.name, func() {
			err := s.decodeAndValidate(tc.yaml)
			s.Require().Error(err)
			s.Contains(strings.ToLower(err.Error()), tc.wants)
		})
	}
}

// TestABossIsAMonsterWithAFlag.
//
// Folding `boss:` into `place:` is what makes the first two of these free: a
// boss standing outside its chamber, or on top of an altar, is refused by the
// rules every other placement already goes through. The dialect this replaces
// had those rules written twice, once per list, which is how two answers to
// "is this cell inside the room" eventually disagree.
//
// The third is the one the flag has to carry itself: "the boss" is singular,
// and the key it replaces could not be written twice in one chamber.
func (s *DialectSuite) TestABossIsAMonsterWithAFlag() {
	const captain = `      - { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5], targeting: closest, boss: true }`

	for _, tc := range []struct {
		name, yaml, wants string
	}{
		{"a boss that is a prop", tombWith(captain,
			`      - { ref: "dnd5e:props:altar", at: [7, 5], blocks_movement: true, blocks_los: true, boss: true }`),
			"boss"},
		{"a boss standing outside the room", tombWith(captain,
			`      - { ref: "dnd5e:monsters:skeleton-captain", at: [30, 5], boss: true }`), "at"},
		{"a boss on top of a prop", tombWith(captain,
			`      - { ref: "dnd5e:monsters:skeleton-captain", at: [9, 3], boss: true }`), "same cell"},
		{"two bosses in one chamber", tombWith(captain, captain+
			"\n"+`      - { ref: "dnd5e:monsters:skeleton-captain", at: [7, 2], boss: true }`), "more than one boss"},
	} {
		s.Run(tc.name, func() {
			err := s.decodeAndValidate(tc.yaml)
			s.Require().Error(err)
			s.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wants))
		})
	}
}

// TestAConnectorJoinsTwoDECLAREDNEIGHBOURS.
//
// Neighbours is the load-bearing word and it is a compile-shape rule rather
// than a taste: the chambers are laid out in declaration order, so a connector
// between rooms that are not next to each other names a seam that does not
// exist. Refusing it here says so in the author's vocabulary instead of
// producing a dungeon with a door onto solid rock.
func (s *DialectSuite) TestAConnectorJoinsTwoDeclaredNeighbours() {
	for _, tc := range []struct {
		name, yaml, wants string
	}{
		{"a room that was never declared",
			tombWith("  - { from: entrance, to: hall }", "  - { from: entrance, to: ossuary }"), "ossuary"},
		{"itself",
			tombWith("  - { from: entrance, to: hall }", "  - { from: hall, to: hall }"), "itself"},
		{"rooms that do not touch",
			tombWith("  - { from: entrance, to: hall }", "  - { from: entrance, to: tomb }"), "not next to"},
		{"the same seam twice",
			tombWith("  - { from: entrance, to: hall }",
				"  - { from: entrance, to: hall }\n  - { from: hall, to: entrance }"), "twice"},
		{"a lock with nothing to beat",
			tombWith("locked: { dc: 12, ability: dex }", "locked: { dc: 0, ability: dex }"), "dc"},
		{"a lock with no ability",
			tombWith("locked: { dc: 12, ability: dex }", "locked: { dc: 12, ability: \"\" }"), "ability"},
	} {
		s.Run(tc.name, func() {
			err := s.decodeAndValidate(tc.yaml)
			s.Require().Error(err)
			s.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wants))
		})
	}
}

// TestNothingMayStandInADoorway.
//
// The composition refuses a connection endpoint that sits on a prop
// (validateConnectionInputs), so this would be caught either way — but it would
// be caught in the COMPOSITION's vocabulary, about a cell the author never
// wrote, after a compile they cannot see. Said here it is about the line they
// did write.
func (s *DialectSuite) TestNothingMayStandInADoorway() {
	// The tomb is 8 tall, so its doorways sit at row 4 — which is why no
	// placement in the shipping file uses that row.
	const bones = `      - { ref: "dnd5e:props:bone-pile", at: [8, 6], blocks_movement: false, blocks_los: false }`
	const brazier = `      - { ref: "dnd5e:props:brazier", at: [1, 1], blocks_movement: true,  blocks_los: false }`

	s.Run("in the seam the connector opens through", func() {
		err := s.decodeAndValidate(tombWith(bones,
			`      - { ref: "dnd5e:props:bone-pile", at: [9, 4], blocks_movement: false, blocks_los: false }`))
		s.Require().Error(err)
		s.Contains(strings.ToLower(err.Error()), "doorway")
	})

	s.Run("but an edge with no connector through it is just a wall", func() {
		// The entrance's WEST side opens onto nothing — the tomb's only
		// connector from it runs east — so its column 0 is ordinary floor,
		// doorway row or not.
		s.Require().NoError(s.decodeAndValidate(tombWith(brazier,
			`      - { ref: "dnd5e:props:brazier", at: [0, 4], blocks_movement: true,  blocks_los: false }`)))
	})

	s.Run("and the middle of a chamber is nobody's business", func() {
		s.Require().NoError(s.decodeAndValidate(tombWith(bones,
			`      - { ref: "dnd5e:props:bone-pile", at: [4, 4], blocks_movement: false, blocks_los: false }`)))
	})
}

// ─────────────────────────────────────────────────────────────────────────

func (s *DialectSuite) decodeAndValidate(raw string) error {
	spec, err := dungeonspec.Decode([]byte(raw))
	if err != nil {
		return err
	}
	return dungeonspec.Validate(spec)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// tombWith is the shipping tomb with one line (or block) swapped, so every
// rejection below differs from a VALID spec by exactly the thing it is about.
func tombWith(old, new string) string {
	if !strings.Contains(tombYAML, old) {
		panic("tombWith: anchor not present in the tomb: " + old)
	}
	return strings.Replace(tombYAML, old, new, 1)
}

// roomless is a well-formed spec that declares no chambers — built rather than
// carved out of the tomb, because deleting the tomb's room list by string
// surgery leaves its connectors behind and would be testing two defects at once.
const roomless = `
version: 1
key: hollow
void: opaque
orientation: pointy
height: 8
start: [0, 0]
rooms: []
connectors: []
`

// withoutLine is the shipping tomb with one whole line removed.
func withoutLine(line string) string {
	out := make([]string, 0, 64)
	for _, l := range strings.Split(tombYAML, "\n") {
		if strings.TrimSpace(l) == strings.TrimSpace(line) {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
