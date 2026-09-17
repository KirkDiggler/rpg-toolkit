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
	t.Run("empty scene arrays and pointer isolation", func(t *testing.T) {
		fresh, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		require.NoError(t, err)
		spec := fresh.Spec
		spec.Room.Scene.Items = []encounter.RoomSceneItem{}
		spec.Room.Scene.Groups = []encounter.RoomSceneGroup{}
		spec.Room.Gameplay.PropDeclarations = map[string]RoomPropDeclaration{}
		compiled, err := CompileSingleRoom(CompileSingleRoomInput{Spec: spec})
		require.NoError(t, err)
		require.NotNil(t, compiled.Field.RoomScene)
		require.NotNil(t, compiled.Field.RoomScene.Scene.Items)
		require.NotNil(t, compiled.Field.RoomScene.Scene.Groups)
		spec.Room.Scene.Name = "changed"
		require.Equal(t, "Workshop", compiled.Field.RoomScene.Scene.Name)
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
		spec.Room.Scene.Items[0].Transform.X = math.Sqrt(3)
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
				spec.Room.Scene.Items[0].Transform.X = 0
				spec.Room.Scene.Items[0].Transform.Z = 0
			}},
			{"monster under footprint", func(spec *SingleRoomSpec) {
				spec.Room.Scene.Items[0].Transform.X = 2 * math.Sqrt(3)
				spec.Room.Scene.Items[0].Transform.Z = 0
			}},
			{"duplicate monster cell", func(spec *SingleRoomSpec) {
				m := spec.Room.Gameplay.Monsters[0]
				m.ID = "second"
				spec.Room.Gameplay.Monsters = append(spec.Room.Gameplay.Monsters, m)
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
			for i := range spec.Room.Gameplay.Monsters {
				spec.Room.Gameplay.Monsters[i].Cell.Q += shift.Q
				spec.Room.Gameplay.Monsters[i].Cell.R += shift.R
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
			require.Len(t, compiled.Monsters, len(spec.Room.Gameplay.Monsters))
			for i, m := range compiled.Monsters {
				abs := encounter.HexCellAt(compiled.Field.Canvas.Orientation, int(m.At.X), int(m.At.Y))
				require.Equal(t, float64(spec.Room.Gameplay.Monsters[i].Cell.Q), abs.X)
				require.Equal(t, float64(spec.Room.Gameplay.Monsters[i].Cell.R), abs.Y)
				require.Equal(t, spec.Room.Gameplay.Monsters[i].ID, m.ID, "stable monster id")
			}
		}
	})
}

// mustEncode marshals an edited spec back to the YAML public Load accepts.
func mustEncode(t *testing.T, spec *SingleRoomSpec) []byte {
	t.Helper()
	b, err := yaml.Marshal(spec)
	require.NoError(t, err)

	return b
}
