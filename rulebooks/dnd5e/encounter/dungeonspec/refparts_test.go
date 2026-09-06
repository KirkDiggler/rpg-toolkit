// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// refparts_test.go is what this compiler owns about a placement's ref
// (rpg-project#367, rpg-toolkit#1536): its TYPE, which is what routes it.
//
// The grammar is core's and so are its tests. A ref is module:type:id, the id
// is everything after the second colon, and how many parts it may carry is
// core's rule to state and core's rule to pin. This package used to mirror
// that check, and the mirror is what these tests were mostly about — counts,
// gaps, a cap — which meant every change to core cascaded into a second set of
// assertions about the same thing, and the two could drift into a ref the
// canvas accepted and the run refused.
//
// What is left is the seam and the routing: a well-formed ref of a placeable
// type compiles and carries its ref through unchanged, a type this compiler
// cannot place is refused by name, and a ref core refuses is refused HERE, in
// core's words, at the path the builder draws on.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// aStripPlacing writes one placement into theStrip, on the study's free cell.
func aStripPlacing(ref string) string {
	return theStrip("") + `place:
  - { ref: "` + ref + `", at: [1,0], blocks_movement: false, blocks_los: false }
`
}

// TestAPlacementRefWithPartsInItsID — an exact-ref prop routes as props and
// reaches the field carrying the ref the author wrote.
//
// The four-part ref is here because it is the one that used to be refused:
// the compiler counted colons and expected two, and "place[19].ref
// \"dnd5e:props:plushie:skeleton-dog\" is not module:type:id" is what Kirk's
// walk hit. Routing reads the type, and the type is where it always was.
//
// The last assertion is the one worth having. Routing on the type while
// TRIMMING the id would satisfy "it compiles" and still hand the run a prop
// nobody can resolve, so the test reads the ref back off the compiled field
// rather than only counting the props.
func TestAPlacementRefWithPartsInItsID(t *testing.T) {
	refs := []struct {
		name string
		ref  string
	}{
		{"the three-part ref that always worked", "dnd5e:props:brazier"},
		{"an exact-ref prop", "dnd5e:props:plushie:skeleton-dog"},
	}

	for _, r := range refs {
		t.Run(r.name, func(t *testing.T) {
			scene := aStripPlacing(r.ref)

			spec, err := dungeonspec.Decode([]byte(scene))
			require.NoError(t, err)
			require.Empty(t, dungeonspec.Validate(spec), "the scene was meant to compile")

			compiled, err := dungeonspec.Load([]byte(scene))
			require.NoError(t, err)
			require.Len(t, compiled.Field.Props, 1, "it routed as a prop, on its type")
			assert.Equal(t, r.ref, compiled.Field.Props[0].Ref,
				"and the ref reaches the field exactly as authored, id parts and all")
			assert.Equal(t, spatial.Position{X: 1, Y: 0}, compiled.Field.Props[0].At)
		})
	}
}

// TestARefThisCompilerCannotPlace — routing reads the TYPE and only the type,
// and a deeper id does not smuggle a placement past it.
//
// This is the question that is genuinely this package's: props and monsters
// are what it knows how to put on a field, and a trap is refused by name
// rather than dropped. Whether "dnd5e:traps:pit:spiked" RESOLVES to anything
// is a different question, and design law C1 says this package does not get
// to ask it.
func TestARefThisCompilerCannotPlace(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(aStripPlacing("dnd5e:traps:pit:spiked")))
	require.NoError(t, err)

	errs := dungeonspec.Validate(spec)
	require.NotEmpty(t, errs)
	assert.Equal(t, "place[0].ref", errs[0].Path)
	assert.Contains(t, errs[0].Message, "cannot place")
	assert.Contains(t, errs[0].Message, `names type "traps"`)
}

// TestARefCoreRefusesIsRefusedOnTheCanvas is the seam, and the only thing this
// file has to say about the grammar.
//
// A ref core will not parse has to become a defect the builder can draw, at
// the placement's own path, in core's words. Nothing here re-states what makes
// a ref malformed — core's tests own that, and duplicating them is what this
// file stopped doing.
//
// The expected text is asked of core rather than written out, so a reworded
// refusal upstream travels through instead of failing here. What is pinned is
// that the words ARRIVE, under the ref the author wrote, at place[0].ref.
func TestARefCoreRefusesIsRefusedOnTheCanvas(t *testing.T) {
	const bad = "dnd5e:props:plushie:"

	_, coreErr := core.ParseString(bad)
	require.Error(t, coreErr, "the fixture has to be a ref core actually refuses")

	spec, err := dungeonspec.Decode([]byte(aStripPlacing(bad)))
	require.NoError(t, err)

	errs := dungeonspec.Validate(spec)
	require.NotEmpty(t, errs, "a ref core refuses is a defect in the file")
	assert.Equal(t, "place[0].ref", errs[0].Path,
		"drawn on the placement the author wrote, not on the file")
	assert.Contains(t, errs[0].Message, bad,
		"the refusal quotes the ref")
	assert.Contains(t, errs[0].Message, coreErr.Error(),
		"and carries core's own words, whatever they are")
}
