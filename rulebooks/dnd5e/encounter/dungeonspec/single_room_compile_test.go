package dungeonspec

import (
	"errors"
	"os"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
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
}
