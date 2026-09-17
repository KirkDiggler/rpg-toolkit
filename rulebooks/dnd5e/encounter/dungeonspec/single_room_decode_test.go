package dungeonspec

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

type SingleRoomSourceSuite struct {
	suite.Suite
	raw []byte
}

func (s *SingleRoomSourceSuite) SetupTest() {
	var err error
	s.raw, err = os.ReadFile("testdata/world-builder-v3.yaml")
	s.Require().NoError(err)
}
func (s *SingleRoomSourceSuite) TestDecodePreservesSourceFacts() {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	s.Require().NotNil(out.Spec)
	s.Equal(-2.25, out.Spec.Room.Scene.Items[0].Transform.X)
	s.Equal(1.5, *out.Spec.Room.Scene.Items[0].HeightScale)
	s.False(*out.Spec.Room.Gameplay.PropDeclarations["table"].BlocksLineOfSight)
	s.Equal("table", out.Spec.Room.Scene.Items[1].SupportID)
	s.Equal("furniture", out.Spec.Room.Scene.Items[1].ParentID)
}
func (s *SingleRoomSourceSuite) TestDecodeRoundTripsJSONAndNestedGraph() {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	encoded, err := json.Marshal(out.Spec)
	s.Require().NoError(err)
	again, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: encoded})
	s.Require().NoError(err)
	s.Equal(out.Spec, again.Spec)
	s.False(*again.Spec.Room.Gameplay.PropDeclarations["table"].BlocksLineOfSight)
}
func (s *SingleRoomSourceSuite) TestDecodeRejectsLoadBearingInvalidValues() {
	cases := []struct{ name, old, repl string }{{"missing transform coordinate", "x: -2.25, ", ""}, {"unsupported policy", "standing: centre-covered", "standing: invented"}, {"nonfinite footprint", "width: 1.2", "width: .nan"}, {"missing start coordinate", "partyStart: {q: 0, r: 0}", "partyStart: {r: 0}"}, {"wrong monster kind", "dnd5e:monsters:skeleton", "dnd5e:props:table"}, {"bad color", "#ff9d52", "#gggggg"}, {"bad workspace", "horizontalLimit: 12", "horizontalLimit: -1"}}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			raw := []byte(strings.Replace(string(s.raw), tc.old, tc.repl, 1))
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
			s.Error(err)
		})
	}
}
func (s *SingleRoomSourceSuite) TestDecodeRejectsUnknownAndSecondDocument() {
	for _, raw := range [][]byte{append(append([]byte{}, s.raw...), []byte("\n---\nversion: 3\n")...), append(append([]byte{}, s.raw...), []byte("\nextra: true\n")...)} {
		_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		s.Error(err)
	}
}

// swapOne replaces exactly one occurrence of old, failing when the anchor is
// not unique: a mutation that matches twice would test the wrong spot.
func (s *SingleRoomSourceSuite) swapOne(old, repl string) []byte {
	s.Require().Equal(1, strings.Count(string(s.raw), old), "mutation anchor must appear exactly once: "+old)
	return []byte(strings.Replace(string(s.raw), old, repl, 1))
}

func (s *SingleRoomSourceSuite) itemsBlock() (int, int) {
	start := strings.Index(string(s.raw), "    items:\n")
	end := strings.Index(string(s.raw), "    groups:\n")
	s.Require().Positive(start)
	s.Require().Positive(end)
	s.Require().Less(start, end)
	return start, end
}

func (s *SingleRoomSourceSuite) TestDecodeAcceptsEveryWorkspacePreset() {
	for _, preset := range []string{
		"workspace: {hexRadius: 6, horizontalLimit: 12}",
		"workspace: {hexRadius: 10, horizontalLimit: 20}",
		"workspace: {hexRadius: 14, horizontalLimit: 28}",
	} {
		s.Run(preset, func() {
			out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(
				"workspace: {hexRadius: 6, horizontalLimit: 12}", preset)})
			s.Require().NoError(err)
			s.Require().NotNil(out.Spec)
			s.Equal(3, out.Spec.Version)
		})
	}
}

func (s *SingleRoomSourceSuite) TestDecodePreservesOptionalAbsence() {
	cases := []struct {
		name, old, repl string
		check           func(*SingleRoomDecodeResult) bool
	}{
		{"missing heightScale", "heightScale: 1.5\n        ", "", func(out *SingleRoomDecodeResult) bool {
			return out.Spec.Room.Scene.Items[0].HeightScale == nil
		}},
		{"missing pointLight", "        pointLight: {enabled: true, offset: {x: 0, y: 0.5, z: 0},\n          color: '#ff9d52', intensity: 1.1, range: 2.6}\n", "", func(out *SingleRoomDecodeResult) bool {
			return out.Spec.Room.Scene.Items[1].PointLight == nil
		}},
		{"missing supportId", "        supportId: table\n", "", func(out *SingleRoomDecodeResult) bool {
			return out.Spec.Room.Scene.Items[1].SupportID == ""
		}},
		{"missing parentId", "        parentId: furniture\n        supportId: table", "        supportId: table", func(out *SingleRoomDecodeResult) bool {
			return out.Spec.Room.Scene.Items[1].ParentID == ""
		}},
		{"missing partyStart", "    partyStart: {q: 0, r: 0}\n", "", func(out *SingleRoomDecodeResult) bool {
			return out.Spec.Room.Gameplay.PartyStart == nil
		}},
		{"empty monsters", "    monsters:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', cell: {q: 2, r: 0}}\n", "    monsters: []\n", func(out *SingleRoomDecodeResult) bool {
			return len(out.Spec.Room.Gameplay.Monsters) == 0
		}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(tc.old, tc.repl)})
			s.Require().NoError(err)
			s.Require().NotNil(out.Spec)
			s.True(tc.check(out), tc.name)
		})
	}
}

func (s *SingleRoomSourceSuite) TestDecodeAcceptsNestedGroupsAndTemplateDeclarations() {
	raw := s.swapOne("    groups:\n", "    groups:\n      - {id: nested, kind: group, label: Nested, parentId: furniture, transform: {x: 0, y: 0, z: 0, rotationY: 0}}\n"+
		"      - {id: deeper, kind: group, label: Deeper, parentId: nested, transform: {x: 0, y: 0, z: 0, rotationY: 0}}\n")
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Require().Len(out.Spec.Room.Scene.Groups, 3)
	s.Equal("furniture", out.Spec.Room.Scene.Groups[0].ParentID)
	s.Equal("nested", out.Spec.Room.Scene.Groups[1].ParentID)
	s.Empty(out.Spec.Room.Scene.Groups[2].ParentID)

	raw = s.swapOne("arrangementDeclarations: {}",
		"arrangementDeclarations: {arr: {template: {blocksMovement: false, blocksLineOfSight: false, footprint: {width: 1, depth: 1, offsetX: 0, offsetZ: 0}}}}")
	out, err = DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	decl := out.Spec.Room.Gameplay.ArrangementDeclarations["arr"]["template"]
	s.Require().NotNil(decl.BlocksMovement)
	s.False(*decl.BlocksMovement)
	s.Require().NotNil(decl.BlocksLineOfSight)
	s.False(*decl.BlocksLineOfSight)
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsAuthoredNulls() {
	cases := []struct{ name, old, repl string }{
		{"null heightScale", "heightScale: 1.5", "heightScale: null"},
		{"null pointLight", "        pointLight: {enabled: true, offset: {x: 0, y: 0.5, z: 0},\n          color: '#ff9d52', intensity: 1.1, range: 2.6}", "        pointLight: null"},
		{"null parentId", "        parentId: furniture\n        supportId: table", "        parentId: null\n        supportId: table"},
		{"null supportId", "        supportId: table", "        supportId: null"},
		{"null monster cell", "cell: {q: 2, r: 0}", "cell: null"},
		{"null monster coordinate", "cell: {q: 2, r: 0}", "cell: {q: null, r: 0}"},
		{"null partyStart", "partyStart: {q: 0, r: 0}", "partyStart: null"},
		{"null key", "key: workshop-room", "key: null"},
		{"null walkableHexes", "    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0},\n      {q: 0, r: 1}, {q: -1, r: 1}, {q: -1, r: 0},\n      {q: 0, r: -1}, {q: 1, r: -1}]\n", "    walkableHexes: null\n"},
		{"null monsters", "    monsters:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', cell: {q: 2, r: 0}}\n", "    monsters: null\n"},
		{"null propDeclarations", "    propDeclarations:\n      table:\n        blocksMovement: true\n        blocksLineOfSight: false\n        footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}\n", "    propDeclarations: null\n"},
		{"null declaration", "    propDeclarations:\n      table:\n        blocksMovement: true\n        blocksLineOfSight: false\n        footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}\n", "    propDeclarations:\n      table: null\n"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(tc.old, tc.repl)})
			s.Error(err)
		})
	}
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsTruncatedIntegers() {
	cases := []struct{ name, old, repl string }{
		{"fractional walkable q", "walkableHexes: [{q: 0, r: 0}", "walkableHexes: [{q: 0.5, r: 0}"},
		{"fractional monster q", "cell: {q: 2, r: 0}", "cell: {q: 2.5, r: 0}"},
		{"integral float monster q", "cell: {q: 2, r: 0}", "cell: {q: 2.0, r: 0}"},
		{"float root version", "version: 3\nkey: workshop-room", "version: 3.0\nkey: workshop-room"},
		{"float room version", "  version: 3\n  id: room-1", "  version: 3.0\n  id: room-1"},
		{"float scene version", "    version: 1\n", "    version: 1.0\n"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(tc.old, tc.repl)})
			s.Error(err)
		})
	}
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsMissingRequiredFields() {
	cases := []struct{ name, old, repl string }{
		{"missing footprint offsetX", "offsetX: 0.1, ", ""},
		{"missing footprint width", "footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}", "footprint: {depth: 0.5, offsetX: 0.1, offsetZ: -0.2}"},
		{"missing monster q", "cell: {q: 2, r: 0}", "cell: {r: 0}"},
		{"missing monster ref", "ref: 'dnd5e:monsters:skeleton', ", ""},
		{"missing light enabled", "enabled: true, ", ""},
		{"missing light intensity", "intensity: 1.1, ", ""},
		{"missing light range", ", range: 2.6}", "}"},
		{"missing light color", "color: '#ff9d52', ", ""},
		{"missing light offset", "offset: {x: 0, y: 0.5, z: 0},\n          ", ""},
		{"missing declaration flags", "blocksMovement: true\n        ", ""},
		{"missing template flags", "arrangementDeclarations: {}",
			"arrangementDeclarations: {arr: {template: {footprint: {width: 1, depth: 1, offsetX: 0, offsetZ: 0}}}}"},
		{"missing template footprint", "arrangementDeclarations: {}",
			"arrangementDeclarations: {arr: {template: {blocksMovement: true, blocksLineOfSight: false}}}"},
		{"missing monsters list", "    monsters:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', cell: {q: 2, r: 0}}\n", ""},
		{"missing walkableHexes", "    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0},\n      {q: 0, r: 1}, {q: -1, r: 1}, {q: -1, r: 0},\n      {q: 0, r: -1}, {q: 1, r: -1}]\n", ""},
		{"missing propDeclarations", "    propDeclarations:\n      table:\n        blocksMovement: true\n        blocksLineOfSight: false\n        footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}\n", ""},
		{"missing arrangementDeclarations", "    arrangementDeclarations: {}\n", ""},
		{"missing workspace limit", "workspace: {hexRadius: 6, horizontalLimit: 12}", "workspace: {hexRadius: 6}"},
		{"missing footprintFrame", "    footprintFrame: owner-local-xz}", "  }"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(tc.old, tc.repl)})
			s.Error(err)
		})
	}
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsUnsupportedWorkspaces() {
	for _, preset := range []string{
		"workspace: {hexRadius: 8, horizontalLimit: 16}",
		"workspace: {hexRadius: 6, horizontalLimit: 13}",
		"workspace: {hexRadius: 6, horizontalLimit: 12, extra: 1}",
	} {
		s.Run(preset, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(
				"workspace: {hexRadius: 6, horizontalLimit: 12}", preset)})
			s.Error(err)
		})
	}
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsGraphTargetAndCycleDefects() {
	cases := []struct{ name, old, repl string }{
		{"group self parent", "id: furniture, kind: group, label: Furniture,",
			"id: furniture, kind: group, label: Furniture, parentId: furniture,"},
		{"two-prop support cycle", "heightScale: 1.5", "heightScale: 1.5\n        supportId: candles"},
		{"two-group parent cycle", "      - {id: furniture, kind: group, label: Furniture,\n         transform: {x: -2.175, y: 0.6, z: 1.275, rotationY: 0.37}}",
			"      - {id: furniture, kind: group, label: Furniture, parentId: g2,\n         transform: {x: -2.175, y: 0.6, z: 1.275, rotationY: 0.37}}\n" +
				"      - {id: g2, kind: group, label: G2, parentId: furniture,\n         transform: {x: 0, y: 0, z: 0, rotationY: 0}}"},
		{"prop parent names a prop", "        parentId: furniture\n        supportId: table", "        parentId: table\n        supportId: table"},
		{"support names a group", "        supportId: table", "        supportId: furniture"},
		{"group parent names a prop", "id: furniture, kind: group, label: Furniture,",
			"id: furniture, kind: group, label: Furniture, parentId: table,"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(tc.old, tc.repl)})
			s.Error(err)
		})
	}

	// A three-prop support cycle must be caught at any length, not just the
	// two-prop case.
	raw := s.swapOne("      - id: candles\n", "      - id: candles2\n        kind: prop\n        assetRef: dnd5e:props:candles\n        label: Candles2\n"+
		"        transform: {x: -2.1, y: 1.2, z: 1.25, rotationY: 0.37}\n        parentId: furniture\n        supportId: candles\n      - id: candles\n")
	raw = []byte(strings.Replace(string(raw),
		"transform: {x: -2.25, y: 0, z: 1.3, rotationY: 0.37}\n        heightScale: 1.5\n        parentId: furniture\n",
		"transform: {x: -2.25, y: 0, z: 1.3, rotationY: 0.37}\n        heightScale: 1.5\n        parentId: furniture\n        supportId: candles2\n", 1))
	_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Error(err, "three-prop support cycle")
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsOutOfRangeNumbers() {
	cases := []struct{ name, old, repl string }{
		{"negative light range", "range: 2.6", "range: -1"},
		{"huge light range", "range: 2.6", "range: 25"},
		{"negative intensity", "intensity: 1.1", "intensity: -1"},
		{"huge intensity", "intensity: 1.1", "intensity: 21"},
		{"light offset beyond workspace", "offset: {x: 0, y: 0.5, z: 0}", "offset: {x: 13, y: 0.5, z: 0}"},
		{"transform y too tall", "y: 1.2, ", "y: 9, "},
		{"transform y below floor", "y: 1.2, ", "y: -0.5, "},
		{"transform x beyond workspace", "x: -2.25, ", "x: -12.5, "},
		{"rotationY beyond range", "z: 1.3, rotationY: 0.37}", "z: 1.3, rotationY: 400}"},
		{"heightScale too small", "heightScale: 1.5", "heightScale: 0.2"},
		{"heightScale too large", "heightScale: 1.5", "heightScale: 4.5"},
		{"footprint too narrow", "width: 1.2", "width: 0.05"},
		{"footprint too wide", "width: 1.2", "width: 20"},
		{"footprint offset out of range", "offsetX: 0.1, ", "offsetX: 13, "},
		{"walkable outside workspace", "walkableHexes: [{q: 0, r: 0}", "walkableHexes: [{q: 7, r: 0}"},
		// INT EXTREMES, NOT JUST OUT-OF-RANGE ONES. A floor cell at the edge
		// of the int range once passed the workspace check because the cube
		// metric's signed arithmetic wrapped on it — |MinInt| came back
		// negative, and a negative distance is smaller than every radius.
		// Both extremes are refused, by name.
		{"walkable min-int q", "walkableHexes: [{q: 0, r: 0}", "walkableHexes: [{q: -9223372036854775808, r: 0}"},
		{"walkable max-int q", "walkableHexes: [{q: 0, r: 0}", "walkableHexes: [{q: 9223372036854775807, r: 0}"},
		{"walkable min-int r", "walkableHexes: [{q: 0, r: 0}", "walkableHexes: [{q: 0, r: -9223372036854775808}"},
		{"walkable min-int q and r", "walkableHexes: [{q: 0, r: 0}", "walkableHexes: [{q: -9223372036854775808, r: -9223372036854775808}"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(tc.old, tc.repl)})
			s.Error(err)
		})
	}
}

func (s *SingleRoomSourceSuite) TestCubeDistanceNeverWrapsOnExtremeCells() {
	// The metric itself, not only one decode path through it: every extreme
	// answer must name the cell outside every workspace, never inside one.
	for _, c := range []RoomCell{
		{Q: math.MinInt, R: 0}, {Q: 0, R: math.MinInt},
		{Q: math.MaxInt, R: 0}, {Q: 0, R: math.MaxInt},
		{Q: math.MinInt, R: math.MinInt}, {Q: math.MaxInt, R: math.MinInt},
		{Q: -9223372036854775807, R: 9223372036854775806},
	} {
		s.Greater(cubeDistance(c), 14, "extreme cell %v measured inside the largest workspace", c)
	}
	// LEGAL CELLS IDENTICAL: the whole reference floor measures exactly what
	// the axial cube metric says it does.
	s.Equal(0, cubeDistance(RoomCell{Q: 0, R: 0}))
	s.Equal(2, cubeDistance(RoomCell{Q: 2, R: 0}))
	s.Equal(2, cubeDistance(RoomCell{Q: -1, R: -1}))
	s.Equal(1, cubeDistance(RoomCell{Q: -1, R: 1}))
	s.Equal(1, cubeDistance(RoomCell{Q: 1, R: -1}))
	s.Equal(3, cubeDistance(RoomCell{Q: 3, R: -1}))
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsOversizedScene() {
	start, end := s.itemsBlock()
	itemsBlock := string(s.raw[start:end])
	filler := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "      - {id: filler%03d, kind: prop, assetRef: dnd5e:props:candles, label: Filler, transform: {x: 0, y: 0, z: 0, rotationY: 0}}\n", i)
		}
		return b.String()
	}
	noDeclarations := strings.Replace(string(s.raw),
		"    propDeclarations:\n      table:\n        blocksMovement: true\n        blocksLineOfSight: false\n        footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}\n",
		"    propDeclarations: {}\n", 1)
	s.Require().NotEqual(string(s.raw), noDeclarations, "declaration block must match")

	_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: []byte(strings.Replace(noDeclarations, itemsBlock, "    items:\n"+filler(201), 1))})
	s.Error(err, "201 props exceed the editor cap")

	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: []byte(strings.Replace(noDeclarations, itemsBlock, "    items:\n"+filler(200), 1))})
	s.Require().NoError(err, "200 props sit exactly at the cap")
	s.Len(out.Spec.Room.Scene.Items, 200)
}

func (s *SingleRoomSourceSuite) TestDecodeRoundTripsYAMLWithOptionalAbsence() {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	encoded, err := yaml.Marshal(out.Spec)
	s.Require().NoError(err)
	again, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: encoded})
	s.Require().NoError(err)
	s.Equal(out.Spec, again.Spec)

	empty := s.swapOne("    monsters:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', cell: {q: 2, r: 0}}\n", "    monsters: []\n")
	empty = []byte(strings.Replace(string(empty), "    partyStart: {q: 0, r: 0}\n", "", 1))
	trimmed, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: empty})
	s.Require().NoError(err)
	s.Nil(trimmed.Spec.Room.Gameplay.PartyStart)
	s.Empty(trimmed.Spec.Room.Gameplay.Monsters)
	enc2, err := yaml.Marshal(trimmed.Spec)
	s.Require().NoError(err)
	again2, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: enc2})
	s.Require().NoError(err)
	s.Equal(trimmed.Spec, again2.Spec)
}

func (s *SingleRoomSourceSuite) TestDecodeHonorsAnchorsAndMerges() {
	// An alias is the value it refers to, not a second kind of value.
	raw := s.swapOne("label: Table", "label: &t Table")
	raw = []byte(strings.Replace(string(raw), "label: Candles", "label: *t", 1))
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Equal("Table", out.Spec.Room.Scene.Items[1].Label)

	// A merge key supplies values exactly where the struct decode sees them.
	raw = s.swapOne("    propDeclarations:\n      table:\n        blocksMovement: true\n        blocksLineOfSight: false\n        footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}\n",
		"    propDeclarations:\n      table:\n        <<: {blocksMovement: true, blocksLineOfSight: false, footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}}\n")
	out, err = DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	decl := out.Spec.Room.Gameplay.PropDeclarations["table"]
	s.Require().NotNil(decl.BlocksMovement)
	s.True(*decl.BlocksMovement)
	// A directly authored key still wins over the merged one.
	raw = s.swapOne("    propDeclarations:\n      table:\n        blocksMovement: true\n        blocksLineOfSight: false\n        footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}\n",
		"    propDeclarations:\n      table:\n        <<: {blocksMovement: true, footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}}\n        blocksMovement: false\n        blocksLineOfSight: false\n")
	out, err = DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Require().NotNil(out.Spec.Room.Gameplay.PropDeclarations["table"].BlocksMovement)
	s.False(*out.Spec.Room.Gameplay.PropDeclarations["table"].BlocksMovement)
}

func (s *SingleRoomSourceSuite) TestLoadCompilesSingleRoomV3() {
	compiled, err := Load(s.raw)
	s.Require().NoError(err)
	s.Equal("workshop-room", compiled.Key)
	s.Equal("Workshop", compiled.Name)
	s.NotEmpty(compiled.PartyStart)
}

func TestSingleRoomSourceSuite(t *testing.T) { suite.Run(t, new(SingleRoomSourceSuite)) }
