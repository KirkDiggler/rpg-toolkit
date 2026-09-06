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
		{"six segments, which is the cap", "dnd5e:props:plushie:skeleton-dog:chewed:left-ear"},
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
// with the ref quoted, and the refusal SAYS WHICH PART IS MISSING.
//
// The message is pinned in full because it is the whole value of this refusal.
// "is not module:type:id" is true of every string below and useless for all of
// them: the author wrote a ref that looks right, and finding the gap means
// counting colons. Naming the part is what makes the defect drawn on the
// canvas point at something (rpg-project#367's audience: streamers, not
// engineers).
//
// A missing type keeps the shape refusal, and that is the boundary being
// pinned: module and type are the SHAPE of a ref, so getting one wrong is not
// a gap in an id.
func TestAPlacementRefWithAGapInIt(t *testing.T) {
	gaps := []struct {
		name string
		ref  string
		says string
	}{
		{"no id at all", "dnd5e:props:", `ref "dnd5e:props:" has no id`},
		{"a gap at the front of the id", "dnd5e:props::skeleton-dog",
			`ref "dnd5e:props::skeleton-dog" has an empty id part 1`},
		{"a gap in the middle of the id", "dnd5e:props:plushie::skeleton-dog",
			`ref "dnd5e:props:plushie::skeleton-dog" has an empty id part 2`},
		{"a gap at the end of the id", "dnd5e:props:plushie:",
			`ref "dnd5e:props:plushie:" has an empty id part 2`},
		{"no type is a shape refusal, not a gap", "dnd5e::plushie:skeleton-dog",
			`ref "dnd5e::plushie:skeleton-dog" is not module:type:id`},
	}

	for _, g := range gaps {
		t.Run(g.name, func(t *testing.T) {
			spec, err := dungeonspec.Decode([]byte(aStripPlacing(g.ref)))
			require.NoError(t, err)

			errs := dungeonspec.Validate(spec)
			require.NotEmpty(t, errs, "a gap in a ref is still a defect")

			var found bool
			for _, e := range errs {
				if e.Path == "place[0].ref" && strings.Contains(e.Message, g.ref) {
					found = true
					assert.Equal(t, g.says, e.Message)
				}
			}
			assert.True(t, found, "refused at place[0].ref, got %v", errs)
		})
	}
}

// TestARefThisCompilerCannotPlace — routing still reads the TYPE and only the
// type, and a deep id does not smuggle a placement past it. This is the pair to
// the test above: the grammar loosened, the routing did not.
// TestARefTooDeepToBeContent — the depth is capped, and the canvas says so.
//
// Six segments compile; seven are refused at the ref with the count and the
// limit. The cap is core's (see maxRefSegments on why it is restated rather
// than imported), and it is enforced here as well so a concatenation runaway
// is caught in the file the author can still edit rather than when the run
// will not start.
func TestARefTooDeepToBeContent(t *testing.T) {
	atTheCap := "dnd5e:props:plushie:skeleton-dog:chewed:left-ear"

	spec, err := dungeonspec.Decode([]byte(aStripPlacing(atTheCap)))
	require.NoError(t, err)
	require.Empty(t, dungeonspec.Validate(spec), "six segments is a ref this compiler places")

	spec, err = dungeonspec.Decode([]byte(aStripPlacing(atTheCap + ":frayed")))
	require.NoError(t, err)

	errs := dungeonspec.Validate(spec)
	require.NotEmpty(t, errs)
	require.Equal(t, "place[0].ref", errs[0].Path)
	assert.Equal(t,
		`ref "dnd5e:props:plushie:skeleton-dog:chewed:left-ear:frayed" has 7 segments; at most 6`,
		errs[0].Message)
}

// TestARefThatIsBothTooDeepAndGappy — a ref can break two rules at once, and
// the two layers have to pick the SAME one.
//
// core asks the cap before it walks the id's parts, so this compiler does too.
// Without that, "dnd5e:props:a:b:c::" is a gap on the canvas and too many
// segments when the run starts: two layers, two reasons, one string, and an
// author sent to fix the wrong end of it.
func TestARefThatIsBothTooDeepAndGappy(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(aStripPlacing("dnd5e:props:a:b:c::")))
	require.NoError(t, err)

	errs := dungeonspec.Validate(spec)
	require.NotEmpty(t, errs)
	require.Equal(t, "place[0].ref", errs[0].Path)
	assert.Equal(t, `ref "dnd5e:props:a:b:c::" has 7 segments; at most 6`, errs[0].Message,
		"the depth is the reason, because it is the reason core would give")
}

func TestARefThisCompilerCannotPlace(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(aStripPlacing("dnd5e:traps:pit:spiked")))
	require.NoError(t, err)

	errs := dungeonspec.Validate(spec)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "cannot place")
	assert.Contains(t, errs[0].Message, `names type "traps"`)
}
