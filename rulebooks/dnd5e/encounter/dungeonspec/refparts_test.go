// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// refparts_test.go is rpg-toolkit#1536 at the compiler: a placement's ref is
// module:type:id, and the ID IS EVERYTHING AFTER THE SECOND COLON.
//
// The exact-ref props of rpg-project#367 mint four-part refs, and the compiler
// refused them — "place[19].ref \"dnd5e:props:plushie:skeleton-dog\" is not
// module:type:id" — because it counted colons and expected two. Counting was
// the compiler imposing structure on an id whose inner shape belongs to the
// content that mints it. What routes a placement is the TYPE, and the type is
// where it always was.
//
// What survives the change is the refusal for a GAP. An empty part is a typo
// rather than a structure the compiler does not own, and the author has to see
// it on the canvas — not when the run refuses to start.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// aStripPlacing writes one placement into theStrip, on the study's free cell.
func aStripPlacing(ref string) string {
	return theStrip("") + `place:
  - { ref: "` + ref + `", at: [1,0], blocks_movement: false, blocks_los: false }
`
}

// TestAPlacementRefWithPartsInItsID — the four-part props ref compiles, routes
// as props, and reaches the field carrying the ref the author wrote.
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
		{"one part deeper still", "dnd5e:props:plushie:skeleton-dog:chewed"},
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

// TestAPlacementRefWithAGapInIt — an empty part is still refused, at the ref,
// with the ref quoted, whether the gap is at the front, the middle, or the end.
func TestAPlacementRefWithAGapInIt(t *testing.T) {
	gaps := []struct {
		name string
		ref  string
	}{
		{"no id at all", "dnd5e:props:"},
		{"a gap at the front of the id", "dnd5e:props::skeleton-dog"},
		{"a gap in the middle of the id", "dnd5e:props:plushie::skeleton-dog"},
		{"a gap at the end of the id", "dnd5e:props:plushie:"},
		{"no type", "dnd5e::plushie:skeleton-dog"},
	}

	for _, g := range gaps {
		t.Run(g.name, func(t *testing.T) {
			spec, err := dungeonspec.Decode([]byte(aStripPlacing(g.ref)))
			require.NoError(t, err)

			errs := dungeonspec.Validate(spec)
			require.NotEmpty(t, errs, "a gap in a ref is still a defect")

			var found bool
			for _, e := range errs {
				if e.Path == "place[0].ref" && strings.Contains(e.Message, "is not module:type:id") {
					found = true
					assert.Contains(t, e.Message, g.ref, "and the refusal quotes the ref the author wrote")
				}
			}
			assert.True(t, found, "refused at place[0].ref, got %v", errs)
		})
	}
}

// TestARefThisCompilerCannotPlace — routing still reads the TYPE and only the
// type, and a deep id does not smuggle a placement past it. This is the pair to
// the test above: the grammar loosened, the routing did not.
func TestARefThisCompilerCannotPlace(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(aStripPlacing("dnd5e:traps:pit:spiked")))
	require.NoError(t, err)

	errs := dungeonspec.Validate(spec)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "cannot place")
	assert.Contains(t, errs[0].Message, `names type "traps"`)
}
