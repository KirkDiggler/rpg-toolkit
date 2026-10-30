// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// v4Castle is the castle as it was authored before version 5, kept as the
// converter's own fixture: the testdata castle beside it is the converted one.
const v4Castle = "testdata/convert-v4/world-builder-v4-castle.yaml"

// TestConvertV4_CastleRoundTripsTheFloor converts the v4 castle in memory and
// decodes it: the union of its regions' cells is the old walkable set exactly,
// the nine rooms are named regions with their labels' words, the implicit
// region holds the rest, and everything is lit at 1 because nothing authored
// a lighting.
func TestConvertV4_CastleRoundTripsTheFloor(t *testing.T) {
	src, err := os.ReadFile(v4Castle)
	require.NoError(t, err)
	var before struct {
		Room struct {
			Room struct {
				WalkableHexes []RoomCell `yaml:"walkableHexes"`
			} `yaml:"room"`
		} `yaml:"room"`
	}
	require.NoError(t, yaml.Unmarshal(src, &before))
	require.Len(t, before.Room.Room.WalkableHexes, 1586)

	converted, notes, err := ConvertV4(src)
	require.NoError(t, err)
	require.Empty(t, notes, "every castle region cell is floor and every label is authored")

	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: converted})
	require.NoError(t, err)
	gp := out.Spec.Room.Gameplay
	require.Equal(t, 5, out.Spec.Version)
	require.ElementsMatch(t, before.Room.Room.WalkableHexes, gp.floorCells(), "the floor is the walkable set")

	names := map[string]string{}
	for _, r := range gp.Regions {
		names[r.ID] = r.Name
		require.Equal(t, 1.0, r.Lighting.Intensity, "region %s", r.ID)
	}
	require.Equal(t, map[string]string{
		"room-1-region":      "The Mad King’s Castle",
		"region-courtyard":   "Courtyard",
		"region-gatehouse":   "Gatehouse",
		"region-great-hall":  "Great hall",
		"region-kitchen":     "Kitchen",
		"region-barracks":    "Barracks",
		"region-chapel":      "Chapel",
		"region-armoury":     "Armoury",
		"region-throne-room": "Throne room",
		"region-cells":       "Cells",
	}, names)
	require.Equal(t, "room-1-region", gp.Regions[0].ID, "the implicit region is written first")
	require.NotEmpty(t, gp.Regions[0].Cells, "the castle has floor in no room: the open ground outside")

	// The presentation that described the old regions is gone with them.
	text := string(converted)
	require.NotContains(t, text, "authoringRegions:")
	require.NotContains(t, text, "walkableHexes:")
	require.NotContains(t, text, "  workspace:")
	require.Contains(t, text, "    mapLabels: []\n", "every castle label was a region's")
}

// TestConvertV4_RefusesAutomaticRegion: an automatic region's cells are a
// witness the converter cannot evaluate, so the file is refused and left for
// the author to convert by hand.
func TestConvertV4_RefusesAutomaticRegion(t *testing.T) {
	src := []byte(`version: 4
key: k
room:
  version: 3
  id: room-1
  name: Room
  workspace: { hexRadius: 6 }
  scene:
    name: Room
    authoringRegions:
      - id: region-a
        boundary: { kind: automatic, witness: { q: 0, r: 0 } }
  room:
    implicitRegionId: room-1-region
    walkableHexes: [{q: 0, r: 0}]
`)
	out, _, err := ConvertV4(src)
	require.ErrorIs(t, err, ErrNotConvertible)
	require.Contains(t, err.Error(), "room.scene.authoringRegions[0] is not an explicit region")
	require.Nil(t, out)
}

// TestConvertV4_PreservesComments: a comment the author wrote before `walls:`
// survives byte-for-byte, and so does every line the conversion does not own.
func TestConvertV4_PreservesComments(t *testing.T) {
	src, err := os.ReadFile(v4Castle)
	require.NoError(t, err)
	const comment = "    # The castle's walls, outer then inner — a line the author wrote.\n"
	src = swapOneIn(t, src, "    implicitRegionId: room-1-region\n    walls:\n",
		"    implicitRegionId: room-1-region\n"+comment+"    walls:\n")

	converted, _, err := ConvertV4(src)
	require.NoError(t, err)
	require.Contains(t, string(converted), "    implicitRegionId: room-1-region\n"+comment+"    walls:\n")
	// The file's head — its whole header comment and the stat blocks — is
	// untouched up to the root version line, which is the only line there
	// that changes.
	head := string(src)[:strings.Index(string(src), "version: 4\n")]
	require.True(t, strings.HasPrefix(string(converted), head+"version: 5\n"))
}

// TestConvertV4_RefusesAVersion5Document: there is nothing to convert.
func TestConvertV4_RefusesAVersion5Document(t *testing.T) {
	src, err := os.ReadFile("testdata/world-builder-v4-castle.yaml")
	require.NoError(t, err)
	_, _, err = ConvertV4(src)
	require.ErrorIs(t, err, ErrNotConvertible)
}

// TestConvertV4Testdata is the conversion itself, run once: with
// CONTENT_CONVERT=write it rewrites, in place, every testdata document that
// still carries a walkable list — the converter's own v4 fixture excepted —
// and logs each file and each note. Without it, it only confirms there is
// nothing left to convert.
func TestConvertV4Testdata(t *testing.T) {
	var pending []string
	require.NoError(t, filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" || strings.HasPrefix(path, filepath.Join("testdata", "convert-v4")) {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(src), "walkableHexes:") {
			pending = append(pending, path)
		}
		return nil
	}))
	sort.Strings(pending)
	if os.Getenv("CONTENT_CONVERT") != "write" {
		require.Empty(t, pending, "documents still carry walkableHexes; run with CONTENT_CONVERT=write")
		return
	}
	for _, path := range pending {
		src, err := os.ReadFile(path)
		require.NoError(t, err)
		out, notes, err := ConvertV4(src)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		for _, n := range notes {
			t.Logf("%s: %s: %s", path, n.Path, n.Message)
		}
		require.NoError(t, os.WriteFile(path, out, 0o600))
		t.Logf("converted %s", path)
	}
}
