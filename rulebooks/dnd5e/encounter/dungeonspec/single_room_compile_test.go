package dungeonspec

import (
	"errors"
	"math"
	"os"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestSingleRoomCompileSuite is the durable host regression for the complete
// v3 path. Keep this as one named suite: the parent gate invokes it directly.
func TestSingleRoomCompileSuite(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(t, err)
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)

	t.Run("public Load preserves identity and IDs", func(t *testing.T) {
		compiled, err := Load(raw)
		require.NoError(t, err)
		require.Equal(t, "workshop-room", compiled.Key)
		require.Equal(t, "Workshop", compiled.Name)
		require.Len(t, compiled.Monsters, 1)
		require.Equal(t, "skeleton-a", compiled.Monsters[0].ID)
		require.Empty(t, compiled.Monsters[0].Actions)
	})
	t.Run("direct compile and nil are atomic", func(t *testing.T) {
		compiled, err := CompileSingleRoom(CompileSingleRoomInput{Spec: decoded.Spec})
		require.NoError(t, err)
		require.NotEmpty(t, compiled.PartyStart)
		zero, err := CompileSingleRoom(CompileSingleRoomInput{})
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrBadSpec))
		require.Equal(t, Compiled{}, zero)
	})
	// A room that declares no prop places none — and the scene it authored
	// reaches the field in no other way (rpg-project#479): the compiled
	// FieldInput is the geometry and nothing else.
	t.Run("a room without declarations places nothing", func(t *testing.T) {
		fresh, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		require.NoError(t, err)
		spec := fresh.Spec
		spec.Room.Gameplay.PropDeclarations = map[string]RoomPropDeclaration{}
		compiled, err := CompileSingleRoom(CompileSingleRoomInput{Spec: spec})
		require.NoError(t, err)
		require.Empty(t, compiled.Field.Placed)
		require.Equal(t, "Workshop", compiled.Name, "the scene still names the dungeon")
	})
	t.Run("source refusals carry editable paths", func(t *testing.T) {
		fresh, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		require.NoError(t, err)
		spec := fresh.Spec
		spec.Room.Gameplay.PartyStart = nil
		compiled, err := CompileSingleRoom(CompileSingleRoomInput{Spec: spec})
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrBadSpec))
		require.Empty(t, compiled)
		var validation *ValidationError
		require.ErrorAs(t, err, &validation)
		require.Contains(t, validation.Errors[0].Path, "partyStart")
	})
	t.Run("yaml direct input remains supported", func(t *testing.T) {
		encoded, err := yaml.Marshal(decoded.Spec)
		require.NoError(t, err)
		compiled, err := Load(encoded)
		require.NoError(t, err)
		require.NotEmpty(t, compiled.Field.Regions)
	})

	// THE COMPILE-LEVEL FACTS, durable (they arrived as temporary probe
	// cases during milestone 3): the seats the compiler derives avoid every
	// static footprint, by the published stationary-contact query — the
	// same fact the encounter's own placement validation asks.
	t.Run("all derived seats avoid static footprints", func(t *testing.T) {
		fresh, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		require.NoError(t, err)
		spec := fresh.Spec
		setItemTransform(t, spec, "table", "x", math.Sqrt(3))
		compiled, err := Load(mustEncode(t, spec))
		require.NoError(t, err)
		require.NotEmpty(t, compiled.PartyStart)
		emb := spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{
			CellWidth: encounter.FeetPerCell, Orientation: spatial.HexOrientationPointyTop,
		})
		for _, seat := range compiled.PartyStart {
			abs := encounter.HexCellAt(compiled.Field.Canvas.Orientation, int(seat.At.X), int(seat.At.Y))
			centre := emb.CellCentre(abs)
			for _, prop := range compiled.Field.Placed {
				if !prop.BlocksMovement {
					continue
				}
				trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
					Placement: prop.Placement, From: centre, To: centre,
				})
				require.NoError(t, err)
				require.False(t, trace.Contact,
					"derived seat %v stands under static footprint %q", seat.At, prop.ID)
			}
		}
	})

	// Occupied and duplicate placements are refused by name, as source
	// validation errors with an editable path and no partial compile.
	t.Run("occupied and duplicate placements are refused by name", func(t *testing.T) {
		cases := []struct {
			name string
			edit func(*SingleRoomSpec)
		}{
			{"start under footprint", func(spec *SingleRoomSpec) {
				setItemTransform(t, spec, "table", "x", 0)
				setItemTransform(t, spec, "table", "z", 0)
			}},
			{"monster under footprint", func(spec *SingleRoomSpec) {
				setItemTransform(t, spec, "table", "x", 2*math.Sqrt(3))
				setItemTransform(t, spec, "table", "z", 0)
			}},
			{"duplicate monster cell", func(spec *SingleRoomSpec) {
				m := spec.Room.Gameplay.MonsterDeclarations[0]
				m.ID = "second"
				spec.Room.Gameplay.MonsterDeclarations = append(spec.Room.Gameplay.MonsterDeclarations, m)
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				fresh, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
				require.NoError(t, err)
				tc.edit(fresh.Spec)
				compiled, err := Load(mustEncode(t, fresh.Spec))
				require.Error(t, err)
				require.True(t, errors.Is(err, ErrBadSpec))
				var validation *ValidationError
				require.ErrorAs(t, err, &validation)
				require.NotEmpty(t, validation.Errors)
				require.NotEqual(t, "", validation.Errors[0].Path)
				require.Equal(t, Compiled{}, compiled, "no partial compile")
			})
		}
	})

	// Negative and odd-row axial cells lower exactly, at the compile
	// boundary: the authored cells, the start and every monster map back to
	// the axial cells the source named, through the canonical inverse.
	t.Run("negative and odd-row axial lowering", func(t *testing.T) {
		for _, shift := range []RoomCell{{Q: 1, R: 1}, {Q: -2, R: -1}} {
			fresh, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
			require.NoError(t, err)
			spec := fresh.Spec
			for i := range spec.Room.Gameplay.WalkableHexes {
				spec.Room.Gameplay.WalkableHexes[i].Q += shift.Q
				spec.Room.Gameplay.WalkableHexes[i].R += shift.R
			}
			spec.Room.Gameplay.PartyStart.Q += shift.Q
			spec.Room.Gameplay.PartyStart.R += shift.R
			for i := range spec.Room.Gameplay.MonsterDeclarations {
				spec.Room.Gameplay.MonsterDeclarations[i].StartingCell.Location.Q += shift.Q
				spec.Room.Gameplay.MonsterDeclarations[i].StartingCell.Location.R += shift.R
			}
			compiled, err := Load(mustEncode(t, spec))
			require.NoError(t, err)

			floor := map[[2]int]bool{}
			for _, c := range spec.Room.Gameplay.WalkableHexes {
				floor[[2]int{c.Q, c.R}] = true
			}
			got := map[[2]int]bool{}
			for _, r := range compiled.Field.Regions {
				for _, c := range r.Cells {
					abs := encounter.HexCellAt(compiled.Field.Canvas.Orientation, int(c.X), int(c.Y))
					got[[2]int{int(abs.X), int(abs.Y)}] = true
				}
			}
			require.Equal(t, floor, got, "floor frame shifted")

			start := encounter.HexCellAt(compiled.Field.Canvas.Orientation,
				int(compiled.PartyStart[0].At.X), int(compiled.PartyStart[0].At.Y))
			require.Equal(t, float64(spec.Room.Gameplay.PartyStart.Q), start.X)
			require.Equal(t, float64(spec.Room.Gameplay.PartyStart.R), start.Y)
			require.Len(t, compiled.Monsters, len(spec.Room.Gameplay.MonsterDeclarations))
			for i, m := range compiled.Monsters {
				abs := encounter.HexCellAt(compiled.Field.Canvas.Orientation, int(m.At.X), int(m.At.Y))
				require.Equal(t, float64(spec.Room.Gameplay.MonsterDeclarations[i].StartingCell.Location.Q), abs.X)
				require.Equal(t, float64(spec.Room.Gameplay.MonsterDeclarations[i].StartingCell.Location.R), abs.Y)
				require.Equal(t, spec.Room.Gameplay.MonsterDeclarations[i].ID, m.ID, "stable monster id")
			}
		}
	})
}

// TestLoadRoutesVersionThreeAndAboveToTheSingleRoomDecoder is the dispatch half
// of the version seam. Load is the entry point consumers actually use, and a
// version it routes to the v2 decoder is misreported as a malformed v2 dungeon
// (`play: "play" is not a key this build reads`) rather than refused as a
// version this build does not speak.
func TestLoadRoutesVersionThreeAndAboveToTheSingleRoomDecoder(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(t, err)
	v3, err := Load(raw)
	require.NoError(t, err)

	// A v4 root carrying only v3 keys compiles through Load, to the SAME world
	// the v3 document compiles to: the version is not content.
	v4 := swapOneIn(t, raw, "version: 3\nkey: workshop-room", "version: 4\nkey: workshop-room")
	compiled, err := Load(v4)
	require.NoError(t, err)
	require.Equal(t, "Workshop", compiled.Name, "the single-room compiler ran")
	require.Equal(t, v3, compiled, "v4-only-in-version compiled to the v3 world")
	require.Equal(t, "workshop-room", compiled.Key)

	// A version nobody agreed on meets the SINGLE ROOM's refusal by name — not
	// a v2 shape error naming play or room, which is what the == 3 dispatch
	// produced for a well-formed v4 site.
	v5 := swapOneIn(t, raw, "version: 3\nkey: workshop-room", "version: 5\nkey: workshop-room")
	_, err = Load(v5)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported version 5 (want 3 or 4)")
	require.NotContains(t, err.Error(), `"play" is not a key`)
	require.NotContains(t, err.Error(), `"room" is not a key`)
}

// TestSingleRoomValidationPaths locks source attribution for every placement
// failure. In particular, it prevents offset-frame coordinates and the old
// "last monster" report from returning to author-facing diagnostics.
func TestSingleRoomValidationPaths(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(t, err)
	cases := []struct {
		name       string
		edit       func(*SingleRoomSpec)
		path       string
		coordinate string
	}{
		{
			name: "party blocked with monsters present", path: "room.room.partyStart", coordinate: "q=0 r=0",
			edit: func(s *SingleRoomSpec) {
				setItemTransform(t, s, "table", "x", 0)
				setItemTransform(t, s, "table", "z", 0)
			},
		},
		{
			name: "party off floor negative odd", path: "room.room.partyStart", coordinate: "q=-3 r=-1",
			edit: func(s *SingleRoomSpec) { s.Room.Gameplay.PartyStart = &RoomCell{Q: -3, R: -1} },
		},
		{
			name: "first monster fails while later monster is valid", path: "room.room.monsterDeclarations[0].startingCell.location", coordinate: "q=2 r=0",
			edit: func(s *SingleRoomSpec) {
				setItemTransform(t, s, "table", "x", 2*math.Sqrt(3))
				setItemTransform(t, s, "table", "z", 0)
				s.Room.Gameplay.MonsterDeclarations[0].StartingCell.Location = RoomCell{Q: 2, R: 0}
				s.Room.Gameplay.MonsterDeclarations = append(s.Room.Gameplay.MonsterDeclarations, RoomMonsterSource{ID: "skeleton-b", Ref: "dnd5e:monsters:skeleton", StartingCell: RoomStartingCell{Location: RoomCell{Q: 1, R: 0}}})
			},
		},
		{
			name: "later monster fails", path: "room.room.monsterDeclarations[1].startingCell.location", coordinate: "q=2 r=0",
			edit: func(s *SingleRoomSpec) {
				setItemTransform(t, s, "table", "x", 2*math.Sqrt(3))
				setItemTransform(t, s, "table", "z", 0)
				s.Room.Gameplay.MonsterDeclarations[0].StartingCell.Location = RoomCell{Q: 1, R: 0}
				s.Room.Gameplay.MonsterDeclarations = append(s.Room.Gameplay.MonsterDeclarations, RoomMonsterSource{ID: "skeleton-b", Ref: "dnd5e:monsters:skeleton", StartingCell: RoomStartingCell{Location: RoomCell{Q: 2, R: 0}}})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
			require.NoError(t, err)
			tc.edit(decoded.Spec)
			compiled, err := CompileSingleRoom(CompileSingleRoomInput{Spec: decoded.Spec})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrBadSpec))
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Equal(t, Compiled{}, compiled)
			require.NotEmpty(t, validation.Errors)
			require.Equal(t, tc.path, validation.Errors[0].Path)
			require.Contains(t, validation.Errors[0].Message, "at author's axial "+tc.coordinate)
		})
	}
}

// mustEncode marshals an edited spec back to the YAML public Load accepts.
func mustEncode(t *testing.T, spec *SingleRoomSpec) []byte {
	t.Helper()
	b, err := yaml.Marshal(spec)
	require.NoError(t, err)

	return b
}
