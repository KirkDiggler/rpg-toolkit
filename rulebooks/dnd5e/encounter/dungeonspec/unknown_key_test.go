// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// unknown_key_test.go is THE GRADE THE WORLD BUILDER SHOWS (rpg-project#481,
// R2).
//
// The builder asks the engine whether a file plays and draws the engine's own
// paths and sentences on the canvas. Two of the three refusals an author meets
// most were already that; the third — a key they misspelled — was a line
// number and a Go type name, which is nothing a canvas can draw and nothing an
// author can read. Every scene below is the shipping camp with ONE key
// misspelled, and each asserts the whole defect: the path it lands at and the
// sentence it lands with.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// theRootKeys is the sentence's own offer for a key misspelled at the top of
// the file, spelled out once. It is the [dungeonspec.Spec] shape in the order
// an author reads a list, which is alphabetical and not authored order: the
// offer is a vocabulary, not a template.
const theRootKeys = "they are dispositions, doors, endings, exits, factions, intel, key, name, " +
	"orientation, place, regions, scenarios, scenery, start, version, void, walls"

// loadDefects runs one edited camp through Load and returns every defect,
// failing the scene if the file was accepted or refused with anything but a
// [dungeonspec.ValidationError].
func loadDefects(t *testing.T, source string) []dungeonspec.FieldError {
	t.Helper()
	_, err := dungeonspec.Load([]byte(source))
	require.Error(t, err)
	require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
	var verr *dungeonspec.ValidationError
	require.ErrorAs(t, err, &verr, "every refusal is a ValidationError, decode ones included")

	return verr.Errors
}

// requireNoGoInIt is the half of the claim that is about WHO the message is
// for. yaml.v3's own words name a Go type and this package's never do.
func requireNoGoInIt(t *testing.T, defects []dungeonspec.FieldError) {
	t.Helper()
	for _, d := range defects {
		require.NotContains(t, d.Message, "dungeonspec.", "an author is told about their file, not about Go")
		require.NotContains(t, d.Message, "not found in type")
	}
}

// TestTheThreeGradesTheWorldBuilderShows is the design's own probe list
// (rpg-project#481): a placement in a faction nobody declared, a trigger key
// that is not one, and a key on a faction that is not one. All three are a
// path and a sentence, which is the whole contract the builder renders — and
// the third is the one this slice makes true.
func TestTheThreeGradesTheWorldBuilderShows(t *testing.T) {
	scenes := []struct {
		name, old, replacement string
		want                   dungeonspec.FieldError
	}{
		{
			name: "a placement in a faction nobody declared",
			old:  chiefLine, replacement: strings.Replace(chiefLine, "faction: raiders", "faction: raidrs", 1),
			want: dungeonspec.FieldError{
				Path: "place[0].faction",
				Message: `"dnd5e:monsters:skeleton-captain" is in faction "raidrs", and no faction in ` +
					"this dungeon has that id — declare it under `factions:`",
			},
		},
		{
			name: "a trigger key that is not one",
			old:  factionLine,
			replacement: `  - { id: raiders, mind: chief, ` +
				`on: { intimdate_failed: [ { say: "Big talk." } ] } }`,
			want: dungeonspec.FieldError{
				Path: "factions[0].on.intimdate_failed",
				Message: `"intimdate_failed" is not a trigger this build rolls: they are ` +
					"intimidated, intimidate_failed, persuaded, persuade_failed, time",
			},
		},
		{
			name: "a key on a faction that is not one",
			old:  factionLine, replacement: `  - { id: raiders, mind: chief, tempre: coward }`,
			want: dungeonspec.FieldError{
				Path:    "factions[0].tempre",
				Message: `"tempre" is not a key this build reads: they are id, mind, on, temper`,
			},
		},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			defects := loadDefects(t, edited(t, sc.old, sc.replacement))
			require.Contains(t, defects, sc.want)
			requireNoGoInIt(t, defects)
		})
	}
}

// TestAnUnknownKeyIsNamedWhereverItIsWritten walks the offending key down the
// document: the root, a list of mappings, a mapping inside one of those, and
// an entry of a table nested three deep. Every one of them is the author's own
// address for the key, in the spelling the rest of this package's paths use.
func TestAnUnknownKeyIsNamedWhereverItIsWritten(t *testing.T) {
	scenes := []struct {
		name, old, replacement string
		want                   dungeonspec.FieldError
	}{
		{
			name: "at the root",
			old:  "void: opaque", replacement: "void: opaque\nheight: 8",
			want: dungeonspec.FieldError{
				Path:    "height",
				Message: `"height" is not a key this build reads: ` + theRootKeys,
			},
		},
		{
			name: "on a region",
			old:  "  - id: gate\n    name: The Gate", replacement: "  - id: gate\n    naem: The Gate",
			want: dungeonspec.FieldError{
				Path: "regions[0].naem",
				Message: `"naem" is not a key this build reads: ` +
					"they are archetype, cells, concealed, id, lighting, name",
			},
		},
		{
			name: "on a placement, written in flow style",
			old:  chiefLine, replacement: strings.Replace(chiefLine, "faction: raiders", "factoin: raiders", 1),
			want: dungeonspec.FieldError{
				Path:    "place[0].factoin",
				Message: `"factoin" is not a key this build reads`,
			},
		},
		{
			name: "inside a predicate",
			old:  zombie1Line, replacement: strings.Replace(zombie1Line, "down: chief", "dwon: chief", 1),
			want: dungeonspec.FieldError{
				Path:    "place[3].arrives.dwon",
				Message: `"dwon" is not a key this build reads`,
			},
		},
		{
			name: "in one entry of one trigger of one table",
			old:  chiefLine,
			replacement: `  - { id: chief, ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, ` +
				`on: { intimidated: [ { say: "Fine." }, { sez: "Fine." } ] } }`,
			want: dungeonspec.FieldError{
				Path:    "place[0].on.intimidated[1].sez",
				Message: `"sez" is not a key this build reads`,
			},
		},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			defects := loadDefects(t, edited(t, sc.old, sc.replacement))
			require.Contains(t, defects, sc.want)
			requireNoGoInIt(t, defects)
		})
	}
}

// TestEveryUnknownKeyInTheFileIsReported is why the defects are a LIST. An
// author who misspelled two keys is told about two keys: `KnownFields`
// collects them all before it gives up, and fixing one to be told about the
// next is the loop this package exists to spare them.
func TestEveryUnknownKeyInTheFileIsReported(t *testing.T) {
	source := edited(t, "void: opaque", "void: opaque\nheight: 8")
	source = strings.Replace(source, factionLine, `  - { id: raiders, mind: chief, tempre: coward }`, 1)
	defects := loadDefects(t, source)

	require.Equal(t, []dungeonspec.FieldError{
		{Path: "height", Message: `"height" is not a key this build reads: ` + theRootKeys},
		{Path: "factions[0].tempre",
			Message: `"tempre" is not a key this build reads: they are id, mind, on, temper`},
	}, defects, "both of them, in the order they were written")
}

// TestTheKeysOfferedAreTheKeysTheDecoderReads is what makes "they are ..." worth
// printing. The offer is reflected off the shape rather than listed, so this
// authors every key it offers for a faction and requires the decoder to take
// the file — a list that drifted from the shape would be caught here rather
// than by an author following it into a second refusal.
func TestTheKeysOfferedAreTheKeysTheDecoderReads(t *testing.T) {
	_, err := dungeonspec.Decode([]byte(edited(t, factionLine,
		`  - { id: raiders, mind: chief, temper: coward, on: { intimidated: [ { say: "Fine." } ] } }`)))
	require.NoError(t, err, "id, mind, temper and on are exactly what the refusal offers")
}

// TestAShapeReadByHandOffersNoList is the honesty rule. [dungeonspec.PlaceSpec]
// and the other custom unmarshalers read their own keys, and a struct's fields
// are not that grammar — [dungeonspec.AnswerSpec] carries a `Line` no file
// authors. So those refusals name the key and stop, rather than offering a
// list an author could follow into a second refusal.
func TestAShapeReadByHandOffersNoList(t *testing.T) {
	byHand := loadDefects(t, edited(t, chiefLine,
		strings.Replace(chiefLine, "faction: raiders", "factoin: raiders", 1)))
	require.Contains(t, byHand,
		dungeonspec.FieldError{Path: "place[0].factoin", Message: `"factoin" is not a key this build reads`})

	byTheDecoder := loadDefects(t, edited(t, factionLine, `  - { id: raiders, mind: chief, tempre: coward }`))
	require.Contains(t, byTheDecoder, dungeonspec.FieldError{Path: "factions[0].tempre",
		Message: `"tempre" is not a key this build reads: they are id, mind, on, temper`})
}

// TestAKeyTheWalkCannotPlaceKeepsItsLine is the one case a path is not
// offered, and it is offered NOTHING rather than a guess. Flow style can write
// two keys of the same name on one line at two different paths, and the
// decoder reports only the line; drawing the refusal on either would be
// drawing it on a key that may be fine. The sentence is still the author's.
func TestAKeyTheWalkCannotPlaceKeepsItsLine(t *testing.T) {
	source := edited(t, chiefLine,
		`  - { id: chief, ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, `+
			`arrives: { id: chief } }`)
	defects := loadDefects(t, source)

	require.Len(t, defects, 1)
	require.Equal(t, `"id" is not a key this build reads`, defects[0].Message,
		"the sentence is the author's either way")
	requireNoGoInIt(t, defects)

	// And the line it kept is the line the key is on — read back out of the
	// source rather than written down here, so the claim survives the fixture
	// growing a line.
	var number int
	_, err := fmt.Sscanf(defects[0].Path, "line %d", &number)
	require.NoError(t, err, "two `id` keys on one line, so the line is all there is: %q", defects[0].Path)
	lines := strings.Split(source, "\n")
	require.Greater(t, number, 0)
	require.LessOrEqual(t, number, len(lines))
	require.Contains(t, lines[number-1], "arrives: { id: chief }")
}
