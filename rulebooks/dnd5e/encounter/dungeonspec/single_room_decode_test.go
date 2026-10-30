package dungeonspec

import (
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
	s.Equal([]RoomRegion{{
		ID: "room-1-region", Name: "Workshop", Lighting: RoomLighting{Intensity: 1},
		Cells: []RoomCellRow{{R: -1, Q: [][2]int{{0, 1}}}, {R: 0, Q: [][2]int{{-1, 2}}}, {R: 1, Q: [][2]int{{-1, 0}}}},
	}}, out.Spec.Room.Gameplay.Regions)

	read, defects := readRoom(&out.Spec.Room)
	s.Empty(defects)
	s.Equal("Workshop", read.Name)
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

// TestDecodeVersionsAreTheRootsAndRefusedByName is the version seam at 5
// (rpg-project#725): the root version alone describes the site document, the
// room draft's own version stays 3, the version buys no leniency, and a
// version nobody agreed on is refused by name, keeping the offending value.
func (s *SingleRoomSourceSuite) TestDecodeVersionsAreTheRootsAndRefusedByName() {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	s.Equal(5, out.Spec.Version)
	s.Equal(3, out.Spec.Room.Version)

	_, err = DecodeSingleRoom(SingleRoomDecodeInput{Source: append(append([]byte{}, s.raw...), []byte("\nsites: []\n")...)})
	s.Require().Error(err, "an unknown root key is refused at 5")
	s.Contains(err.Error(), "sites")

	_, err = DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne("version: 5\nkey: workshop-room", "version: 5.0\nkey: workshop-room")})
	s.Require().Error(err)
	s.Contains(err.Error(), "version: must be an integer")

	for _, tc := range []struct{ name, repl, want string }{
		{"version 6", "version: 6\nkey: workshop-room", "unsupported version 6 (want 5)"},
		{"version 2", "version: 2\nkey: workshop-room", "unsupported version 2 (want 5)"},
	} {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne("version: 5\nkey: workshop-room", tc.repl)})
			s.Require().Error(err)
			s.Contains(err.Error(), tc.want)
		})
	}

	// The room's own version is still required to be 3, and the refusal
	// names the ROOM's version, not the root's.
	room4 := s.swapOne("  version: 3\n  id: room-1", "  version: 4\n  id: room-1")
	_, err = DecodeSingleRoom(SingleRoomDecodeInput{Source: room4})
	var defects *ValidationError
	s.Require().ErrorAs(err, &defects)
	s.Contains(defects.Errors, FieldError{Path: "room.version", Message: "unsupported version 4 (want 3)"})
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
		{"empty monsters", "    monsterDeclarations:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', startingCell: { location: { q: 2, r: 0 } }}\n", "    monsterDeclarations: []\n", func(out *SingleRoomDecodeResult) bool {
			return len(out.Spec.Room.Gameplay.MonsterDeclarations) == 0
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
		{"null monster cell", "location: { q: 2, r: 0 }", "location: null"},
		{"null monster coordinate", "location: { q: 2, r: 0 }", "location: { q: null, r: 0 }"},
		{"null partyStart", "partyStart: {q: 0, r: 0}", "partyStart: null"},
		{"null key", "key: workshop-room", "key: null"},
		{"null regions", workshopRegions, "    regions: null\n"},
		{"null region cells", workshopCells, "        cells: null\n"},
		{"null monsters", "    monsterDeclarations:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', startingCell: { location: { q: 2, r: 0 } }}\n", "    monsterDeclarations: null\n"},
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
		{"fractional span q", "q: [[0, 1]]", "q: [[0.5, 1]]"},
		{"fractional row r", "- r: -1\n", "- r: -1.5\n"},
		{"fractional monster q", "location: { q: 2, r: 0 }", "location: { q: 2.5, r: 0 }"},
		{"integral float monster q", "location: { q: 2, r: 0 }", "location: { q: 2.0, r: 0 }"},
		{"float root version", "version: 5\nkey: workshop-room", "version: 5.0\nkey: workshop-room"},
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
		{"missing monster q", "location: { q: 2, r: 0 }", "location: { r: 0 }"},
		{"missing monster ref", "ref: 'dnd5e:monsters:skeleton', ", ""},
		{"missing declaration flags", "blocksMovement: true\n        ", ""},
		{"missing template flags", "arrangementDeclarations: {}",
			"arrangementDeclarations: {arr: {template: {footprint: {width: 1, depth: 1, offsetX: 0, offsetZ: 0}}}}"},
		{"missing template footprint", "arrangementDeclarations: {}",
			"arrangementDeclarations: {arr: {template: {blocksMovement: true, blocksLineOfSight: false}}}"},
		{"missing monsters list", "    monsterDeclarations:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', startingCell: { location: { q: 2, r: 0 } }}\n", ""},
		{"missing region lighting", "        lighting: { intensity: 1 }\n", ""},
		{"missing region name", "        name: Workshop\n", ""},
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
		// INT EXTREMES, NOT JUST OUT-OF-RANGE ONES. There is no workspace
		// radius any more (rpg-project#725), and a cell at the edge of the
		// int range would overflow the encounter's integer conversion; the
		// coordinate bound refuses both extremes by name.
		{"region min-int q", "q: [[0, 1]]", "q: [[-9223372036854775808, 1]]"},
		{"region max-int q", "q: [[0, 1]]", "q: [[0, 9223372036854775807]]"},
		{"region min-int r", "- r: -1\n", "- r: -9223372036854775808\n"},
		{"region intensity above 1", "lighting: { intensity: 1 }", "lighting: { intensity: 1.5 }"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.swapOne(tc.old, tc.repl)})
			s.Error(err)
		})
	}
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

	empty := s.swapOne("    monsterDeclarations:\n      - {id: skeleton-a, ref: 'dnd5e:monsters:skeleton', startingCell: { location: { q: 2, r: 0 } }}\n", "    monsterDeclarations: []\n")
	empty = []byte(strings.Replace(string(empty), "    partyStart: {q: 0, r: 0}\n", "", 1))
	trimmed, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: empty})
	s.Require().NoError(err)
	s.Nil(trimmed.Spec.Room.Gameplay.PartyStart)
	s.Empty(trimmed.Spec.Room.Gameplay.MonsterDeclarations)
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

func (s *SingleRoomSourceSuite) TestLoadCompilesSingleRoomV5() {
	compiled, err := Load(s.raw)
	s.Require().NoError(err)
	s.Equal("workshop-room", compiled.Key)
	s.Equal("Workshop", compiled.Name)
	s.NotEmpty(compiled.PartyStart)
}

func TestSingleRoomSourceSuite(t *testing.T) { suite.Run(t, new(SingleRoomSourceSuite)) }

// The workshop fixture's floor as authored: one region, three rows.
const (
	workshopCells = "        cells:\n          - r: -1\n            q: [[0, 1]]\n          - r: 0\n" +
		"            q: [[-1, 2]]\n          - r: 1\n            q: [[-1, 0]]\n"
	workshopRegions = "    regions:\n      - id: room-1-region\n        name: Workshop\n" +
		"        lighting: { intensity: 1 }\n" + workshopCells
)

// decodeDefects decodes src and returns its defects, failing when it decodes.
func decodeDefects(t *testing.T, src []byte) []FieldError {
	t.Helper()
	_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: src})
	var defects *ValidationError
	require.ErrorAs(t, err, &defects)
	return defects.Errors
}

// withRegion appends one more region, written as the author would, after the
// workshop's own.
func withRegion(t *testing.T, region string) []byte {
	return swapOneIn(t, fixtureSource(t), workshopRegions, workshopRegions+region)
}

func TestDecode_RegionsRequired(t *testing.T) {
	raw := fixtureSource(t)
	require.Contains(t, decodeDefects(t, swapOneIn(t, raw, workshopRegions, "")),
		FieldError{Path: "room.room.regions", Message: errRequired})
	require.Contains(t, decodeDefects(t, swapOneIn(t, raw, workshopRegions, "    regions: null\n")),
		FieldError{Path: "room.room.regions", Message: errNotNull})
	require.Contains(t, decodeDefects(t, swapOneIn(t, raw, workshopRegions, "    regions: []\n")),
		FieldError{Path: "room.room.regions", Message: "must not be empty"})
}

func TestDecode_RegionLightingRequiredAndBounded(t *testing.T) {
	raw := fixtureSource(t)
	require.Contains(t, decodeDefects(t, swapOneIn(t, raw, "        lighting: { intensity: 1 }\n", "")),
		FieldError{Path: "room.room.regions[0].lighting", Message: errRequired})
	require.Contains(t, decodeDefects(t, swapOneIn(t, raw, "lighting: { intensity: 1 }", "lighting: {}")),
		FieldError{Path: "room.room.regions[0].lighting.intensity", Message: errRequired})
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].lighting.intensity", Message: "must be between 0 and 1"}},
		decodeDefects(t, swapOneIn(t, raw, "lighting: { intensity: 1 }", "lighting: { intensity: 1.5 }")))
	require.Equal(t, []FieldError{{Path: "room.room.regions[0].lighting.intensity", Message: "must be between 0 and 1"}},
		decodeDefects(t, swapOneIn(t, raw, "lighting: { intensity: 1 }", "lighting: { intensity: -0.1 }")))
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: swapOneIn(t, raw, "lighting: { intensity: 1 }", "lighting: { intensity: 0 }")})
	require.NoError(t, err, "a region in darkness is a region")
	require.Equal(t, 0.0, out.Spec.Room.Gameplay.Regions[0].Lighting.Intensity)
}

// TestDecode_CellInTwoRegions: the later listing is the defect, at the span
// that lists the cell again, naming the region that has it.
func TestDecode_CellInTwoRegions(t *testing.T) {
	defects := decodeDefects(t, withRegion(t, "      - id: region-bench\n        name: Bench\n"+
		"        lighting: { intensity: 1 }\n        cells:\n          - r: 0\n            q: [[2, 4]]\n"))
	require.Equal(t, []FieldError{{Path: "room.room.regions[1].cells[0].q[0]", Message: `already belongs to region "room-1-region"`}}, defects)
}

func TestDecode_RegionIDsAreUniqueAndAuthored(t *testing.T) {
	require.Equal(t, []FieldError{{Path: "room.room.regions[1].id", Message: errDuplicateID}},
		decodeDefects(t, withRegion(t, "      - id: room-1-region\n        name: Again\n        lighting: { intensity: 1 }\n        cells: []\n")))
	require.Equal(t, []FieldError{{Path: "room.room.regions[1].id", Message: errRequired}},
		decodeDefects(t, withRegion(t, "      - id: ''\n        name: Nameless\n        lighting: { intensity: 1 }\n        cells: []\n")))
}

// TestDecode_ImplicitRegionIdMayBeUnlisted: the implicit region is where the
// studio's paint lands; a site whose paint all went into named regions lists
// no region under that id, and that is not a defect. The id itself is still
// required.
func TestDecode_ImplicitRegionIdMayBeUnlisted(t *testing.T) {
	raw := fixtureSource(t)
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: swapOneIn(t, raw,
		"implicitRegionId: room-1-region", "implicitRegionId: room-1-paint")})
	require.NoError(t, err)
	require.Equal(t, "room-1-paint", out.Spec.Room.Gameplay.ImplicitRegionID)
	require.Contains(t, decodeDefects(t, swapOneIn(t, raw, "implicitRegionId: room-1-region", "implicitRegionId: ''")),
		FieldError{Path: "room.room.implicitRegionId", Message: errRequired})
}

// TestDecode_EmptyRegionIsAccepted: a place declared and not yet painted.
func TestDecode_EmptyRegionIsAccepted(t *testing.T) {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: withRegion(t,
		"      - id: region-vault\n        name: Vault\n        lighting: { intensity: 0.2 }\n        cells: []\n")})
	require.NoError(t, err)
	require.Len(t, out.Spec.Room.Gameplay.Regions, 2)
	require.Empty(t, out.Spec.Room.Gameplay.Regions[1].Cells)
}

// TestDecode_AllRegionsEmptyRefused: a site with no floor at all is not a
// site; every region declared and none painted is refused at the list.
func TestDecode_AllRegionsEmptyRefused(t *testing.T) {
	defects := decodeDefects(t, swapOneIn(t, fixtureSource(t), workshopCells, "        cells: []\n"))
	require.Contains(t, defects, FieldError{Path: "room.room.regions", Message: "no region owns a cell"})
}

// TestDecode_Version4Refused: a file of the retired shape is told why, by
// version and by each retired key, never read.
func TestDecode_Version4Refused(t *testing.T) {
	src, err := os.ReadFile(v4Castle)
	require.NoError(t, err)
	defects := decodeDefects(t, src)
	require.Contains(t, defects, FieldError{Path: "version", Message: "versions 3 and 4 are retired; regions are the floor (rpg-project#725)"})
	require.Contains(t, defects, FieldError{Path: "room.workspace",
		Message: "workspace is retired: the canvas is unbounded and the file carries only its content (rpg-project#725)"})
	require.Contains(t, defects, FieldError{Path: "room.room.walkableHexes",
		Message: "walkableHexes is retired: the floor is the union of room.room.regions (rpg-project#725)"})

	// And a v4 root on an otherwise v5 document is refused by the version
	// alone.
	require.Equal(t, []FieldError{{Path: "version", Message: "versions 3 and 4 are retired; regions are the floor (rpg-project#725)"}},
		decodeDefects(t, swapOneIn(t, fixtureSource(t), "version: 5\nkey: workshop-room", "version: 4\nkey: workshop-room")))
}

// TestDecode_WorkspaceRefused: the canvas is unbounded and the file carries
// no grid (R3), so a workspace is an unknown key, named where it sits.
func TestDecode_WorkspaceRefused(t *testing.T) {
	defects := decodeDefects(t, swapOneIn(t, fixtureSource(t), "  scene:\n", "  workspace: {hexRadius: 6}\n  scene:\n"))
	require.Contains(t, defects, FieldError{Path: "room.workspace",
		Message: "workspace is retired: the canvas is unbounded and the file carries only its content (rpg-project#725)"})
}

// TestExpandRows_RefusesNonIntegerPair: a span is two integers and nothing
// else, refused at the span's own path.
func TestExpandRows_RefusesNonIntegerPair(t *testing.T) {
	for name, span := range map[string]string{
		"three numbers": "q: [[0, 1, 2]]",
		"one number":    "q: [[0]]",
		"a fraction":    "q: [[0.5, 1]]",
		"a word":        "q: [[zero, 1]]",
		"a bare number": "q: [0]",
	} {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, decodeDefects(t, swapOneIn(t, fixtureSource(t), "q: [[0, 1]]", span)),
				FieldError{Path: "room.room.regions[0].cells[0].q[0]", Message: "must be a pair of integers"})
		})
	}
}

// TestDecode_ThePlanExampleDecodes is the dialect example the plan states for
// the studio (rpg-project#725 plan, Task 1): a gatehouse at 0.3, two rows, a
// gap in the second.
func TestDecode_ThePlanExampleDecodes(t *testing.T) {
	src := swapOneIn(t, fixtureSource(t), workshopRegions, workshopRegions+`      - id: region-gatehouse
        name: Gatehouse
        lighting: { intensity: 0.3 }
        cells:
          - r: -6
            q: [[-2, 2]]
          - r: -5
            q: [[-3, 3], [6, 9]]
`)
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: src})
	require.NoError(t, err)
	gate := out.Spec.Room.Gameplay.Regions[1]
	require.Equal(t, RoomRegion{ID: "region-gatehouse", Name: "Gatehouse", Lighting: RoomLighting{Intensity: 0.3},
		Cells: []RoomCellRow{{R: -6, Q: [][2]int{{-2, 2}}}, {R: -5, Q: [][2]int{{-3, 3}, {6, 9}}}}}, gate)
	require.Len(t, ExpandRows(gate.Cells, "cells", func(string, string) { t.Fatal("canonical") }), 5+7+4)
}

// TestCompile_EmptyRegionContributesNothing: a region declared and not yet
// painted reaches neither the field nor the atlas.
func TestCompile_EmptyRegionContributesNothing(t *testing.T) {
	compiled, err := Load(withRegion(t, "      - id: region-vault\n        name: Vault\n        lighting: { intensity: 0.2 }\n        cells: []\n"))
	require.NoError(t, err)
	ids := []string{}
	for _, r := range compiled.Field.Regions {
		ids = append(ids, r.ID)
	}
	require.Equal(t, []string{"room-1-region"}, ids)
}
