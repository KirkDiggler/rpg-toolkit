// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// single_room_lowering_test.go is the LOWERING's own suite (rpg-project#479):
// the engine reads the handful of values play depends on out of the World
// Builder's authored presentation, refuses each of those it cannot read at
// its own path, and is indifferent to everything else in there.

// sceneNodeFrom parses one authored YAML mapping into the node a
// [RoomSource] carries. A hand-assembled source authors its presentation the
// way a file does, because this package no longer has a Go shape for it.
func sceneNodeFrom(t require.TestingT, src string) yaml.Node {
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &doc))
	require.Len(t, doc.Content, 1, "one document")

	return *doc.Content[0]
}

// setItemTransform rewrites one authored transform number inside a decoded
// scene node, so a test can move a prop the way the editor would and then
// re-marshal the document for [Load].
func setItemTransform(t *testing.T, spec *SingleRoomSpec, itemID, key string, v float64) {
	t.Helper()
	items := childNode(&spec.Room.Scene, "items")
	require.NotNil(t, items, "the fixture authors a scene item list")
	for _, raw := range items.Content {
		it := resolveNode(raw)
		if id := childNode(it, "id"); id == nil || id.Value != itemID {
			continue
		}
		transform := childNode(it, "transform")
		require.NotNil(t, transform, "item %q authors a transform", itemID)
		for i := 0; i+1 < len(transform.Content); i += 2 {
			if transform.Content[i].Value != key {
				continue
			}
			value := transform.Content[i+1]
			value.Kind, value.Tag, value.Style = yaml.ScalarNode, "!!float", 0
			value.Value = strconv.FormatFloat(v, 'g', -1, 64)

			return
		}
		t.Fatalf("item %q authors no transform.%s", itemID, key)
	}
	t.Fatalf("the scene authors no item %q", itemID)
}

// fixtureSource is the committed workshop room, the one v3 document every
// single-room test starts from.
func fixtureSource(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(t, err)

	return raw
}

// TestThePresentationCarriesWhatTheEngineNeverModels is the whole point of
// the lowering: the World Builder may author anything it likes in the scene,
// the frame and the workspace, and the engine compiles the same world from
// it. A light, a label, a group, a height scale and a word no version of
// either codec has ever heard of all ride through without a murmur — the
// codec that owns the scene is the one that judges it (rpg-project#479, R3).
func TestThePresentationCarriesWhatTheEngineNeverModels(t *testing.T) {
	raw := fixtureSource(t)
	before, err := Load(raw)
	require.NoError(t, err)

	// A key nobody has agreed on, inside each of the three subtrees, plus one
	// on the very item a declaration names.
	odd := swapOneIn(t, raw, "      - id: table\n", "      - id: table\n        foo: bar\n        heightHint: {shape: tall, fudge: 0.25}\n")
	odd = swapOneIn(t, odd, "  workspace: {hexRadius: 6, horizontalLimit: 12}",
		"  workspace: {hexRadius: 6, horizontalLimit: 12, gridLines: true}")
	odd = swapOneIn(t, odd, "    footprintFrame: owner-local-xz}", "    footprintFrame: owner-local-xz, upIsUp: yes}")

	after, err := Load(odd)
	require.NoError(t, err, "the engine does not judge the presentation")
	require.Equal(t, before, after, "and compiles the same world from it")
}

// TestTheLoweringRefusesEveryNumberPlayReads pins the other half: a value the
// engine DOES depend on, missing or unusable, is named at its own source
// path rather than arriving as a silent zero.
func TestTheLoweringRefusesEveryNumberPlayReads(t *testing.T) {
	raw := fixtureSource(t)
	cases := []struct {
		name, old, repl string
		path, message   string
	}{
		{
			// `table` is the declared prop; x is one of the three numbers its
			// placement is built from, and 0 is a real pose.
			name: "declared item without transform.x",
			old:  "transform: {x: -2.25, y: 0, z: 1.3, rotationY: 0.37}",
			repl: "transform: {y: 0, z: 1.3, rotationY: 0.37}",
			path: "room.scene.items[0].transform.x", message: errRequired,
		},
		{
			name: "declared item with a non-finite pose",
			old:  "transform: {x: -2.25, y: 0, z: 1.3, rotationY: 0.37}",
			repl: "transform: {x: .nan, y: 0, z: 1.3, rotationY: 0.37}",
			path: "room.scene.items[0].transform.x", message: "must be finite",
		},
		{
			name: "declared item with a transform that is not a mapping",
			old:  "transform: {x: -2.25, y: 0, z: 1.3, rotationY: 0.37}",
			repl: "transform: overhead",
			path: "room.scene.items[0].transform.rotationY", message: errRequired,
		},
		{
			name: "declared item with no id to join on",
			old:  "      - id: table\n", repl: "      - kind: prop\n",
			path: "room.scene.items[0].id", message: errRequired,
		},
		{
			name: "two items under one id",
			old:  "      - id: candles\n", repl: "      - id: table\n",
			path: "room.scene.items[1].id", message: errDuplicateID,
		},
		{
			name: "frame hexRadius the adapter is not calibrated to",
			old:  "hexRadius: 1,", repl: "hexRadius: 2,",
			path: "room.coordinateFrame.hexRadius", message: "must be 1",
		},
		{
			name: "frame hexRadius unauthored",
			old:  " hexRadius: 1,\n", repl: "\n",
			path: "room.coordinateFrame.hexRadius", message: errRequired,
		},
		{
			name: "workspace hexRadius unauthored",
			old:  "workspace: {hexRadius: 6, horizontalLimit: 12}", repl: "workspace: {horizontalLimit: 12}",
			path: "room.workspace.hexRadius", message: errRequired,
		},
		{
			name: "workspace hexRadius authored as a word",
			old:  "workspace: {hexRadius: 6, horizontalLimit: 12}", repl: "workspace: {hexRadius: wide, horizontalLimit: 12}",
			path: "room.workspace.hexRadius", message: errNotANumber,
		},
		{
			name: "scene name unauthored",
			old:  "    name: Workshop\n", repl: "",
			path: "room.scene.name", message: errRequired,
		},
		{
			// The entries move under a key nothing reads, so `items` is
			// authored, present, and not a list.
			name: "scene items authored as a mapping",
			old:  "    items:\n", repl: "    items: {}\n    spares:\n",
			path: "room.scene.items", message: errNotAList,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: swapOneIn(t, raw, tc.old, tc.repl)})
			require.Error(t, err)
			var defects *ValidationError
			require.ErrorAs(t, err, &defects)
			require.Contains(t, defects.Errors, FieldError{Path: tc.path, Message: tc.message})
		})
	}
}

// TestANonPositiveWorkspaceRadiusIsOneDefect is the second half of the
// missing-radius rule, and the one a flag alone did not cover: an absent
// `hexRadius` is named once and the floor bound is skipped, but an AUTHORED 0
// or -6 read as a usable bound and every walkable cell was then blamed for
// sitting outside it — eight defects for -6 on this fixture, seven for 0.
//
// A floor of radius zero or less is not a floor. It is the same lie as the
// absent number wearing a value, so it is refused the same way: once, at its
// own path, before anything is measured against it.
func TestANonPositiveWorkspaceRadiusIsOneDefect(t *testing.T) {
	raw := fixtureSource(t)
	for _, radius := range []string{"0", "-6"} {
		t.Run("hexRadius "+radius, func(t *testing.T) {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: swapOneIn(t, raw,
				"workspace: {hexRadius: 6, horizontalLimit: 12}",
				"workspace: {hexRadius: "+radius+", horizontalLimit: 12}")})
			require.Error(t, err)
			var defects *ValidationError
			require.ErrorAs(t, err, &defects)
			require.Equal(t, []FieldError{{Path: "room.workspace.hexRadius", Message: "must be positive"}},
				defects.Errors, "one unusable radius, one defect — never one per cell")
		})
	}

	// And the smallest positive radius the metric can answer for is still a
	// floor: the bound is about sign, not about the editor's presets.
	oneCell := swapOneIn(t, raw, "workspace: {hexRadius: 6, horizontalLimit: 12}",
		"workspace: {hexRadius: 0.5, horizontalLimit: 12}")
	oneCell = swapOneIn(t, oneCell, "    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0},\n"+
		"      {q: 0, r: 1}, {q: -1, r: 1}, {q: -1, r: 0},\n"+
		"      {q: 0, r: -1}, {q: 1, r: -1}]\n", "    walkableHexes: [{q: 0, r: 0}]\n")
	oneCell = swapOneIn(t, oneCell, "cell: {q: 2, r: 0}", "cell: {q: 0, r: 0}")
	_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: oneCell})
	require.NoError(t, err, "a positive radius is a floor, however small")
}

// TestAnUndeclaredPropNeedsNoPose is the scoping rule the read set implies:
// the three numbers are required of the props a declaration NAMES, because
// those are the ones a footprint is placed from. A prop that blocks nothing
// is visual dressing, and the engine has no business asking it for a pose.
func TestAnUndeclaredPropNeedsNoPose(t *testing.T) {
	raw := fixtureSource(t)
	// `candles` is authored, undeclared, and loses its whole transform.
	loose := swapOneIn(t, raw, "        transform: {x: -2.1, y: 1.2, z: 1.25, rotationY: 0.37}\n", "")
	compiled, err := Load(loose)
	require.NoError(t, err, "an undeclared prop's pose is nobody's requirement")
	require.Len(t, compiled.Field.Placed, 1, "and the declared prop still places")
	require.Equal(t, "table", compiled.Field.Placed[0].ID)
}

// TestTheLoweringResolvesAliasesInsideTheScene keeps the node walk honest
// about the YAML it is reading: the struct decode honours anchors and merge
// keys, so the presentation walk beside it has to as well, or the same file
// would mean two things to the two halves of one decoder.
func TestTheLoweringResolvesAliasesInsideTheScene(t *testing.T) {
	raw := fixtureSource(t)
	anchored := swapOneIn(t, raw, "transform: {x: -2.25, y: 0, z: 1.3, rotationY: 0.37}",
		"transform: {x: &x -2.25, y: 0, z: 1.3, rotationY: 0.37}")
	anchored = swapOneIn(t, anchored, "transform: {x: -2.1, y: 1.2, z: 1.25, rotationY: 0.37}",
		"transform: {x: *x, y: 1.2, z: 1.25, rotationY: 0.37}")
	// The declaration moves onto the aliased prop, so the alias is a number
	// the placement is actually built from.
	anchored = swapOneIn(t, anchored, "    propDeclarations:\n      table:\n", "    propDeclarations:\n      candles:\n")

	compiled, err := Load(anchored)
	require.NoError(t, err)
	require.Len(t, compiled.Field.Placed, 1)
	require.Equal(t, "candles", compiled.Field.Placed[0].ID)
	require.InDelta(t, -2.25*feetPerSourceUnit, compiled.Field.Placed[0].Placement.Origin.X, 1e-12,
		"the aliased x is the anchored one")

	merged := swapOneIn(t, raw, "transform: {x: -2.25, y: 0, z: 1.3, rotationY: 0.37}",
		"transform: {<<: {x: -2.25, z: 1.3}, y: 0, rotationY: 0.37}")
	compiled, err = Load(merged)
	require.NoError(t, err)
	require.Len(t, compiled.Field.Placed, 1)
	require.InDelta(t, -2.25*feetPerSourceUnit, compiled.Field.Placed[0].Placement.Origin.X, 1e-12,
		"a merged x is the author's x")
}

// TestTheLoweringReadsTheSceneNameForTheDungeonsOwn is the one non-numeric
// value that survived: [Compiled.Name] is what a host's dungeon list shows,
// and it comes from the scene the author named.
func TestTheLoweringReadsTheSceneNameForTheDungeonsOwn(t *testing.T) {
	compiled, err := Load(swapOneIn(t, fixtureSource(t), "    name: Workshop\n", "    name: The Back Workshop\n"))
	require.NoError(t, err)
	require.Equal(t, "The Back Workshop", compiled.Name)
}

// TestCubeDistanceIsNotAppliedWithoutAWorkspace keeps one missing number from
// reporting as a whole floor's worth of defects: the radius is named once,
// and the cells it could not be compared against are not blamed for it.
func TestCubeDistanceIsNotAppliedWithoutAWorkspace(t *testing.T) {
	_, err := DecodeSingleRoom(SingleRoomDecodeInput{
		Source: swapOneIn(t, fixtureSource(t), "workspace: {hexRadius: 6, horizontalLimit: 12}", "workspace: {}"),
	})
	require.Error(t, err)
	var defects *ValidationError
	require.ErrorAs(t, err, &defects)
	require.Len(t, defects.Errors, 1, "one unreadable number, one defect")
	require.Equal(t, "room.workspace.hexRadius", defects.Errors[0].Path)
}

// TestTheWorkspaceRadiusStillBoundsTheFloor is the other half: the number is
// read for a reason, and a cell outside it is still refused.
func TestTheWorkspaceRadiusStillBoundsTheFloor(t *testing.T) {
	raw := fixtureSource(t)
	_, err := DecodeSingleRoom(SingleRoomDecodeInput{
		Source: swapOneIn(t, raw, "walkableHexes: [{q: 0, r: 0}", "walkableHexes: [{q: 7, r: 0}"),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside the workspace floor")

	// A wider workspace than the editor ever offered is not this decoder's
	// business any more, and the same cell is inside it.
	wider := swapOneIn(t, raw, "walkableHexes: [{q: 0, r: 0}", "walkableHexes: [{q: 7, r: 0}")
	wider = swapOneIn(t, wider, "workspace: {hexRadius: 6, horizontalLimit: 12}",
		"workspace: {hexRadius: 9, horizontalLimit: 18}")
	wider = swapOneIn(t, wider, "partyStart: {q: 0, r: 0}", "partyStart: {q: 1, r: 0}")
	_, err = DecodeSingleRoom(SingleRoomDecodeInput{Source: wider})
	require.NoError(t, err, "the editor's preset list left with the presentation")
}

// TestSceneNumberNamesEveryWayItCannotRead covers the reader itself, without
// a document around it: each refusal is a value that would otherwise arrive
// as a zero that means something.
func TestSceneNumberNamesEveryWayItCannotRead(t *testing.T) {
	node := sceneNodeFrom(t, "authored: 2.5\nnull: ~\nword: wide\nnan: .nan\ninf: .inf\n")
	cases := []struct {
		key     string
		want    float64
		ok      bool
		message string
	}{
		{key: "authored", want: 2.5, ok: true},
		{key: "absent", message: errRequired},
		{key: "null", message: errNotNull},
		{key: "word", message: errNotANumber},
		{key: "nan", message: "must be finite"},
		{key: "inf", message: "must be finite"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			var got []FieldError
			add := func(p, m string) { got = append(got, FieldError{Path: p, Message: m}) }
			v, ok := sceneNumber(&node, tc.key, "room.workspace", add)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, tc.want, v)
				require.Empty(t, got)

				return
			}
			require.Equal(t, []FieldError{{Path: "room.workspace." + tc.key, Message: tc.message}}, got)
		})
	}
}

// TestTheLoweringIsIndifferentToTheEditorsOldBounds names, one by one, the
// judgements that left with the presentation. Each of these documents was
// refused before rpg-project#479 and compiles now, because none of them
// changes a gameplay fact.
func TestTheLoweringIsIndifferentToTheEditorsOldBounds(t *testing.T) {
	raw := fixtureSource(t)
	cases := []struct{ name, old, repl string }{
		{"a light nobody can read", "color: '#ff9d52', intensity: 1.1, range: 2.6", "color: puce, intensity: -4, range: 900"},
		{"a prop taller than the editor allows", "heightScale: 1.5", "heightScale: 40"},
		{"an undeclared prop outside the drawing limit", "x: -2.1,", "x: -400,"},
		{"a prop under the floor", "y: 1.2,", "y: -6,"},
		{"a group parented to nothing at all", "parentId: furniture\n        supportId: table", "parentId: nowhere\n        supportId: table"},
		{"a frame that names no plane", "horizontalPlane: world-xz,", "horizontalPlane: mercator,"},
		{"a workspace the editor never offered", "workspace: {hexRadius: 6, horizontalLimit: 12}", "workspace: {hexRadius: 6, horizontalLimit: 999}"},
		{"a scene version from a later editor", "    version: 1\n    id: scene-1", "    version: 7\n    id: scene-1"},
	}
	before, err := Load(raw)
	require.NoError(t, err)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compiled, err := Load(swapOneIn(t, raw, tc.old, tc.repl))
			require.NoError(t, err, "the web's codec owns this word, not this one")
			require.Equal(t, before, compiled, "and the world it compiles is unchanged")
		})
	}
}

// TestADeclarationWithoutAPoseIsNamedAtTheAdapter covers the exported
// [RoomSource.CanonicalPlacedProps] on a hand-assembled source, which is the
// only way to reach these two sentences: a successful decode has already
// refused both at their own paths.
func TestADeclarationWithoutAPoseIsNamedAtTheAdapter(t *testing.T) {
	yes := true
	decl := map[string]RoomPropDeclaration{
		"table": {BlocksMovement: &yes, BlocksLineOfSight: &yes, Footprint: RoomFootprint{Width: 1, Depth: 1}},
	}

	ghost := &RoomSource{
		Scene:    sceneNodeFrom(t, "items:\n  - {id: bench, transform: {x: 1, z: 0, rotationY: 0}}\n"),
		Gameplay: RoomGameplaySource{PropDeclarations: decl},
	}
	_, err := ghost.CanonicalPlacedProps()
	require.ErrorContains(t, err, `prop declaration "table" names no live scene prop`)

	poseless := &RoomSource{
		Scene:    sceneNodeFrom(t, "items:\n  - {id: table, transform: {z: 0, rotationY: 0}}\n"),
		Gameplay: RoomGameplaySource{PropDeclarations: decl},
	}
	_, err = poseless.CanonicalPlacedProps()
	require.ErrorContains(t, err, `prop declaration "table" names a scene prop whose transform is not authored`)
}

// TestTheLoweringSurvivesAnEditedSceneNode is the test helper's own proof:
// moving a prop through the node and re-marshalling the document produces a
// file that compiles to the moved geometry. Every compile test that shifts a
// prop rests on this.
func TestTheLoweringSurvivesAnEditedSceneNode(t *testing.T) {
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: fixtureSource(t)})
	require.NoError(t, err)
	setItemTransform(t, decoded.Spec, "table", "x", math.Sqrt(3))

	encoded, err := yaml.Marshal(decoded.Spec)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "assetRef: dnd5e:props:torture-table",
		"the presentation the engine never modelled is still in the document")

	compiled, err := Load(encoded)
	require.NoError(t, err)
	require.Len(t, compiled.Field.Placed, 1)
	require.InDelta(t, math.Sqrt(3)*feetPerSourceUnit, compiled.Field.Placed[0].Placement.Origin.X, 1e-12)
}

// TestTheDecodedDocumentRoundTripsThroughYAML replaces the JSON round trip
// this suite used to make. The presentation is a [yaml.Node] and has no
// honest JSON shape, so the claim moves to the format the document is
// authored in — and it is the stronger claim, because it covers the subtree
// the engine does not model.
func TestTheDecodedDocumentRoundTripsThroughYAML(t *testing.T) {
	raw := fixtureSource(t)
	first, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)
	once, err := yaml.Marshal(first.Spec)
	require.NoError(t, err)

	second, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: once})
	require.NoError(t, err)
	twice, err := yaml.Marshal(second.Spec)
	require.NoError(t, err)
	require.Equal(t, string(once), string(twice), "the decode is round-trip stable")

	// The author's own words are still in there, including the ones no type
	// in this module names.
	for _, word := range []string{"assetRef", "pointLight", "heightScale", "groups", "horizontalLimit", "footprintFrame"} {
		require.True(t, strings.Contains(string(once), word), "the re-emitted document still carries %q", word)
	}
}
