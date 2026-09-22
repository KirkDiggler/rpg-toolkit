package dungeonspec

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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

// TestDecodePreservesSourceFacts pins what the decoder MODELS: the gameplay
// grammar, verbatim, and the values the lowering reads out of the
// presentation beside it. The scene's own words — assetRef, label, parentId,
// supportId, heightScale, pointLight — are not facts this package has, by
// design (rpg-project#479); the document still carries them, which
// TestTheDecodedDocumentRoundTripsThroughYAML is the proof of.
func (s *SingleRoomSourceSuite) TestDecodePreservesSourceFacts() {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	s.Require().NotNil(out.Spec)
	s.False(*out.Spec.Room.Gameplay.PropDeclarations["table"].BlocksLineOfSight)
	s.Equal(1.2, out.Spec.Room.Gameplay.PropDeclarations["table"].Footprint.Width)
	s.Equal("room-1-region", out.Spec.Room.Gameplay.ImplicitRegionID)

	read, defects := readRoom(&out.Spec.Room)
	s.Empty(defects)
	s.Equal("Workshop", read.Name)
	s.Equal(6.0, read.WorkspaceHexRadius)
	s.Equal(map[string]bool{"table": true, "candles": true}, read.ItemIDs)
	s.Equal(scenePose{X: -2.25, Z: 1.3, RotationY: 0.37}, read.Poses["table"])
	s.NotContains(read.Poses, "candles", "an undeclared prop's pose is nobody's read")
}
func (s *SingleRoomSourceSuite) TestDecodeRejectsLoadBearingInvalidValues() {
	cases := []struct{ name, old, repl string }{{"missing transform coordinate", "x: -2.25, ", ""}, {"unsupported policy", "standing: centre-covered", "standing: invented"}, {"nonfinite footprint", "width: 1.2", "width: .nan"}, {"missing start coordinate", "partyStart: {q: 0, r: 0}", "partyStart: {r: 0}"}, {"wrong monster kind", "dnd5e:monsters:skeleton", "dnd5e:props:table"}}
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

// TestDecodeAcceptsV4RootAheadOfItsKeys is the version seam: the four
// properties the web's own v4 test asserts for it (singleRoomDungeon.test.ts,
// "accepts the v4 root ahead of its keys, without loosening strictness"),
// mirrored here, plus the room-version split and the version-neutrality the
// whole versioning scheme rests on.
//
// The mutation replaces the ROOT version line only — exactly as the web's
// `.replace('version: 3', 'version: 4')` does — leaving the room draft at 3,
// which is the only combination the web can currently produce.
func (s *SingleRoomSourceSuite) TestDecodeAcceptsV4RootAheadOfItsKeys() {
	asV4 := s.swapOne("version: 3\nkey: workshop-room", "version: 4\nkey: workshop-room")

	// 1. A v4 root carrying only v3 keys decodes. The root version is carried
	// verbatim, and the room's own version is untouched.
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: asV4})
	s.Require().NoError(err)
	s.Require().NotNil(out.Spec)
	s.Equal(4, out.Spec.Version)
	s.Equal(3, out.Spec.Room.Version)

	// 2. The version buys no leniency. KnownFields(true) refuses an unknown
	// root key at 4 exactly as it does at 3.
	//
	// The key this asks about is `sites`, which is the DELIBERATELY DEFERRED
	// word (rpg-project#477, Decision 2: a site's rooms stay flat and nesting
	// belongs to the layer above). It used to be `factions`, and that is
	// exactly the point of the change: a version says what a file MAY
	// contain, so a key arriving inside v4 (rpg-toolkit#1826) stops being
	// unknown while every key nobody has agreed on stays refused.
	_, err = DecodeSingleRoom(SingleRoomDecodeInput{
		Source: append(append([]byte{}, asV4...), []byte("\nsites: []\n")...),
	})
	s.Require().Error(err, "an unknown root key is refused at v4 too")
	s.Contains(err.Error(), "sites", "and the refusal names the key")

	// The source shape walk is version-blind too: a required key missing, or a
	// scalar of the wrong kind, is refused at the same path at 4 as at 3.
	_, err = DecodeSingleRoom(SingleRoomDecodeInput{
		Source: s.swapOne("version: 3\nkey: workshop-room", "version: 4.0\nkey: workshop-room"),
	})
	s.Require().Error(err, "an integral-looking float root version is still not an integer")
	s.Contains(err.Error(), "version: must be an integer")

	// 3. A version nobody agreed on (5, or 2) is refused BY NAME, keeping the
	// offending value and saying what is wanted.
	for _, tc := range []struct{ name, repl, want string }{
		{"version 5", "version: 5\nkey: workshop-room", "unsupported version 5 (want 3 or 4)"},
		{"version 2", "version: 2\nkey: workshop-room", "unsupported version 2 (want 3 or 4)"},
	} {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{
				Source: s.swapOne("version: 3\nkey: workshop-room", tc.repl),
			})
			s.Require().Error(err)
			s.Contains(err.Error(), tc.want)
		})
	}

	// 4. The seam is VERSION-NEUTRAL: the v4 document — which differs from
	// s.raw only in the root version — decodes to the SAME spec once that one
	// digit is normalized, and the v3 decode is round-trip stable. Both
	// comparisons are over the MARSHALED [SingleRoomSpec], not the source
	// document: yaml.Marshal of a decode is not the file's bytes, so what is
	// pinned here is the decoded shape. The seam adds no default, no repair
	// and no second shape.
	//
	// What this does NOT prove is that the v3 decode is what it was before the
	// change: both operands are produced by this build, so a change that moved
	// v3 and v4 alike would pass. The v3 fixture's committed picture is owed by
	// the keys increment (rpg-toolkit#1826), which brings this fixture — and
	// the v4 one beside it — into the golden walk that
	// contentgolden_test.go filters it out of today. Until then the v3 half
	// rests on the diff: for v == 3 the accept predicate is the one it always
	// was, and nothing else on this path reads the root version.
	v3, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	v3Bytes, err := yaml.Marshal(v3.Spec)
	s.Require().NoError(err)
	again, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: v3Bytes})
	s.Require().NoError(err)
	againBytes, err := yaml.Marshal(again.Spec)
	s.Require().NoError(err)
	s.Equal(string(v3Bytes), string(againBytes), "the v3 decode is round-trip stable")

	out.Spec.Version = 3
	v4AsV3Bytes, err := yaml.Marshal(out.Spec)
	s.Require().NoError(err)
	s.Equal(string(v3Bytes), string(v4AsV3Bytes), "v4-only-in-version decoded to the v3 spec")

	// 5. The room's own version is still required to be 3. A v4 root whose
	// room claims 4 is refused, and the refusal names the ROOM's version — not
	// the root's, which is accepted.
	bothV4 := s.swapOne("version: 3\nkey: workshop-room", "version: 4\nkey: workshop-room")
	bothV4 = s.swapOneIn(bothV4, "  version: 3\n  id: room-1", "  version: 4\n  id: room-1")
	_, err = DecodeSingleRoom(SingleRoomDecodeInput{Source: bothV4})
	s.Require().Error(err)
	var defects *ValidationError
	s.Require().ErrorAs(err, &defects)
	s.Contains(defects.Errors,
		FieldError{Path: "room.version", Message: "unsupported version 4 (want 3)"})
	for _, d := range defects.Errors {
		s.NotEqual("version", d.Path, "the accepted root version is not the wrong one")
	}
}

// swapOne replaces exactly one occurrence of old in the fixture, failing when
// the anchor is not unique: a mutation that matches twice would test the wrong
// spot.
func (s *SingleRoomSourceSuite) swapOne(old, repl string) []byte {
	return swapOneIn(s.T(), s.raw, old, repl)
}

// swapOneIn is swapOne over a haystack the caller supplies, so a mutation
// CHAINED onto an already-edited document keeps the same uniqueness guard as
// the first one — and so a plain test function can use it too.
func (s *SingleRoomSourceSuite) swapOneIn(hay []byte, old, repl string) []byte {
	return swapOneIn(s.T(), hay, old, repl)
}

// swapOneIn replaces exactly one occurrence of old in hay, failing the test
// when the anchor is not unique: a mutation that matches twice would test the
// wrong spot.
func swapOneIn(t require.TestingT, hay []byte, old, repl string) []byte {
	require.Equal(t, 1, strings.Count(string(hay), old),
		"mutation anchor must appear exactly once: "+old)
	return []byte(strings.Replace(string(hay), old, repl, 1))
}

func (s *SingleRoomSourceSuite) TestDecodePreservesOptionalAbsence() {
	cases := []struct {
		name, old, repl string
		check           func(*SingleRoomDecodeResult) bool
	}{
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

func (s *SingleRoomSourceSuite) TestDecodeAcceptsTemplateDeclarations() {
	raw := s.swapOne("arrangementDeclarations: {}",
		"arrangementDeclarations: {arr: {template: {blocksMovement: false, blocksLineOfSight: false, footprint: {width: 1, depth: 1, offsetX: 0, offsetZ: 0}}}}")
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	decl := out.Spec.Room.Gameplay.ArrangementDeclarations["arr"]["template"]
	s.Require().NotNil(decl.BlocksMovement)
	s.False(*decl.BlocksMovement)
	s.Require().NotNil(decl.BlocksLineOfSight)
	s.False(*decl.BlocksLineOfSight)
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsAuthoredNulls() {
	cases := []struct{ name, old, repl string }{
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
		{"missing declaration flags", "blocksMovement: true\n        ", ""},
		{"missing template flags", "arrangementDeclarations: {}",
			"arrangementDeclarations: {arr: {template: {footprint: {width: 1, depth: 1, offsetX: 0, offsetZ: 0}}}}"},
		{"missing template footprint", "arrangementDeclarations: {}",
			"arrangementDeclarations: {arr: {template: {blocksMovement: true, blocksLineOfSight: false}}}"},
		{"missing monsters list", "    monsters:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', cell: {q: 2, r: 0}}\n", ""},
		{"missing walkableHexes", "    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0},\n      {q: 0, r: 1}, {q: -1, r: 1}, {q: -1, r: 0},\n      {q: 0, r: -1}, {q: 1, r: -1}]\n", ""},
		{"missing propDeclarations", "    propDeclarations:\n      table:\n        blocksMovement: true\n        blocksLineOfSight: false\n        footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}\n", ""},
		{"missing arrangementDeclarations", "    arrangementDeclarations: {}\n", ""},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(tc.old, tc.repl)})
			s.Error(err)
		})
	}
}

func (s *SingleRoomSourceSuite) TestDecodeRejectsOutOfRangeNumbers() {
	cases := []struct{ name, old, repl string }{
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

// TestDecodeRoundTripsYAMLWithOptionalAbsence pins that an absent optional
// key stays absent through a marshal and back.
//
// The comparison is over the MARSHALED documents rather than the decoded
// values: the presentation is carried as a [yaml.Node], which remembers the
// line and column it was authored at, and a re-emitted document does not
// reproduce the first file's line numbers. What the round trip claims is
// about the document, and that is what it compares.
func (s *SingleRoomSourceSuite) TestDecodeRoundTripsYAMLWithOptionalAbsence() {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	encoded, err := yaml.Marshal(out.Spec)
	s.Require().NoError(err)
	again, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: encoded})
	s.Require().NoError(err)
	againBytes, err := yaml.Marshal(again.Spec)
	s.Require().NoError(err)
	s.Equal(string(encoded), string(againBytes))

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
	enc3, err := yaml.Marshal(again2.Spec)
	s.Require().NoError(err)
	s.Equal(string(enc2), string(enc3))
	s.Nil(again2.Spec.Room.Gameplay.PartyStart)
}

// TestDecodeHonorsAnchorsAndMerges covers the GAMEPLAY half. The same
// question about the presentation — whether the node walk resolves an alias
// the struct decode would have — is
// TestTheLoweringResolvesAliasesInsideTheScene's.
func (s *SingleRoomSourceSuite) TestDecodeHonorsAnchorsAndMerges() {
	// A merge key supplies values exactly where the struct decode sees them.
	raw := s.swapOne("    propDeclarations:\n      table:\n        blocksMovement: true\n        blocksLineOfSight: false\n        footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}\n",
		"    propDeclarations:\n      table:\n        <<: {blocksMovement: true, blocksLineOfSight: false, footprint: {width: 1.2, depth: 0.5, offsetX: 0.1, offsetZ: -0.2}}\n")
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
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
