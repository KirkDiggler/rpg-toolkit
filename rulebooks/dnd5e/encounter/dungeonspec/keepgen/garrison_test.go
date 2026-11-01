// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// garrisonDoc is the part of a generated document the garrison writes, read
// back as plain YAML: what the file says, not what the generator meant.
type garrisonDoc struct {
	Tables map[string]struct {
		Time []struct {
			When   map[string]string `yaml:"when"`
			Hold   *struct{}         `yaml:"hold"`
			Toward *struct {
				Path []struct{ Q, R int } `yaml:"path"`
			} `yaml:"toward"`
		} `yaml:"time"`
	} `yaml:"tables"`
	Factions []struct {
		ID     string         `yaml:"id"`
		Temper map[string]int `yaml:"temper"`
	} `yaml:"factions"`
	Room struct {
		Room struct {
			MonsterDeclarations []struct {
				ID           string `yaml:"id"`
				StartingCell struct {
					Location struct{ Q, R int } `yaml:"location"`
				} `yaml:"startingCell"`
			} `yaml:"monsterDeclarations"`
			MonsterBindings map[string]struct {
				Faction string `yaml:"faction"`
				Table   string `yaml:"table"`
			} `yaml:"monsterBindings"`
		} `yaml:"room"`
	} `yaml:"room"`
}

func garrisonOptions(garrison, patrols int) Options {
	o := DefaultOptions()
	o.Garrison, o.Patrols = garrison, patrols
	o.Key = "bench-keep-garrison"
	o.CommandLine = "keepgen"
	return o
}

func readGarrison(t *testing.T, doc []byte) garrisonDoc {
	t.Helper()
	var d garrisonDoc
	require.NoError(t, yaml.Unmarshal(doc, &d))
	return d
}

// TestGarrison_EmitsBindings: garrison 5, patrols 2 is one faction with the
// soldier/coward mix, five bindings on the first five declared monsters, the
// first two on their own patrol tables and the other three on the post.
func TestGarrison_EmitsBindings(t *testing.T) {
	doc, _, err := Generate(DefaultPlan(), garrisonOptions(5, 2))
	require.NoError(t, err)
	_, err = Prove(doc)
	require.NoError(t, err)
	d := readGarrison(t, doc)

	require.Len(t, d.Factions, 1)
	require.Equal(t, "garrison", d.Factions[0].ID)
	require.Equal(t, map[string]int{"soldier": 2, "coward": 1}, d.Factions[0].Temper)

	require.Len(t, d.Tables, 3, "post and two patrols")
	post, ok := d.Tables["post"]
	require.True(t, ok)
	require.Len(t, post.Time, 1)
	require.NotNil(t, post.Time[0].Hold)

	decls := d.Room.Room.MonsterDeclarations
	require.Len(t, d.Room.Room.MonsterBindings, 5)
	for i := 0; i < 5; i++ {
		b, ok := d.Room.Room.MonsterBindings[decls[i].ID]
		require.True(t, ok, "monster %d (%s) is bound", i, decls[i].ID)
		require.Equal(t, "garrison", b.Faction)
		if i >= 2 {
			require.Equal(t, "post", b.Table)
			continue
		}
		name := fmt.Sprintf("patrol-%d", i)
		require.Equal(t, name, b.Table)
		tbl, ok := d.Tables[name]
		require.True(t, ok)
		require.Len(t, tbl.Time, 1)
		require.Equal(t, map[string]string{"enemy": "none"}, tbl.Time[0].When)
		require.NotNil(t, tbl.Time[0].Toward)
		path := tbl.Time[0].Toward.Path
		// A three-cell loop: the route closes itself, last leg back to the
		// first waypoint (encounter patrol.go), so the first is not repeated.
		require.Len(t, path, 3)
		start := decls[i].StartingCell.Location
		require.Equal(t, start, path[0], "the patrol starts on its own cell")
	}
}

// TestGarrison_OffIsGolden: zero garrison writes today's bytes.
func TestGarrison_OffIsGolden(t *testing.T) {
	o := DefaultOptions()
	o.CommandLine = "keepgen"
	o.Garrison, o.Patrols = 0, 0
	doc, _, err := Generate(DefaultPlan(), o)
	require.NoError(t, err)
	sum := sha256.Sum256(doc)
	require.Equal(t, goldenKeep, hex.EncodeToString(sum[:]))
}

// goldenGarrison is the SHA-256 of the 10,000-hex keep at seed 1 and the
// default densities, garrison 99, patrols 20, key bench-keep-garrison,
// command line "keepgen".
const goldenGarrison = "24aa1bad43a150f300976856098311fee620b61cf11a9a7bc3fc55980aaebe51"

// TestGenerate_GoldenGarrison pins the garrison document's bytes.
func TestGenerate_GoldenGarrison(t *testing.T) {
	doc, _, err := Generate(DefaultPlan(), garrisonOptions(99, 20))
	require.NoError(t, err)
	sum := sha256.Sum256(doc)
	require.Equal(t, goldenGarrison, hex.EncodeToString(sum[:]))
}

// TestGarrison_RouteCellsAreFloor: every cell every leg of every patrol walks
// — the engine's own [encounter.LegLine] — is compiled floor, in the region
// its monster starts in.
func TestGarrison_RouteCellsAreFloor(t *testing.T) {
	doc, _, err := Generate(DefaultPlan(), garrisonOptions(99, 20))
	require.NoError(t, err)
	compiled, err := Prove(doc)
	require.NoError(t, err)
	// The compiled field's region cells are odd-r offset ([floorCells]);
	// a route's waypoints and legs are the room's axial cells.
	regionOf := map[spatial.Position]string{}
	for c, id := range floorCells(compiled) {
		a := c.axial()
		regionOf[spatial.Position{X: float64(a.Q), Y: float64(a.R)}] = id
	}
	// And no leg walks under a prop, blocking or not.
	o := garrisonOptions(99, 20)
	l, err := layoutOf(DefaultPlan(), o.Hexes)
	require.NoError(t, err)
	st, err := placeAll(DefaultPlan(), l, o)
	require.NoError(t, err)
	underProp := map[spatial.Position]bool{}
	for _, pr := range st.props {
		for _, c := range pr.covered {
			a := c.axial()
			underProp[spatial.Position{X: float64(a.Q), Y: float64(a.R)}] = true
		}
	}
	d := readGarrison(t, doc)
	patrols := 0
	for _, m := range d.Room.Room.MonsterDeclarations {
		b, ok := d.Room.Room.MonsterBindings[m.ID]
		if !ok || b.Table == "post" {
			continue
		}
		patrols++
		home, ok := regionOf[spatial.Position{X: float64(m.StartingCell.Location.Q), Y: float64(m.StartingCell.Location.R)}]
		require.True(t, ok, "%s stands on floor", m.ID)
		path := d.Tables[b.Table].Time[0].Toward.Path
		wps := make([]spatial.Position, len(path))
		for i, c := range path {
			wps[i] = spatial.Position{X: float64(c.Q), Y: float64(c.R)}
		}
		_, retraced := encounter.RetracedCell(wps)
		require.False(t, retraced, "%s's route does not retrace itself", m.ID)
		for i := range wps {
			for _, c := range encounter.LegLine(wps[i], wps[(i+1)%len(wps)]) {
				require.Equal(t, home, regionOf[c], "%s leg %d cell %v is floor in its own region", m.ID, i+1, c)
				require.False(t, underProp[c], "%s leg %d cell %v is under no prop", m.ID, i+1, c)
			}
		}
	}
	require.Equal(t, 20, patrols)
}

// TestGarrison_Refuses: more patrols than garrison, more garrison than
// monsters placed, and a negative count are bad options.
func TestGarrison_Refuses(t *testing.T) {
	for name, o := range map[string]Options{
		"patrols over garrison":  garrisonOptions(3, 4),
		"garrison over monsters": garrisonOptions(100, 0),
		"negative garrison":      garrisonOptions(-1, 0),
		"negative patrols":       garrisonOptions(2, -1),
	} {
		t.Run(name, func(t *testing.T) {
			doc, _, err := Generate(DefaultPlan(), o)
			require.ErrorIs(t, err, ErrBadOptions)
			require.Nil(t, doc)
		})
	}
}

// TestOptionsFromHeader_Garrison: the header records the garrison and gives
// it back, so the file regenerates byte for byte; without one it is absent.
func TestOptionsFromHeader_Garrison(t *testing.T) {
	o := garrisonOptions(7, 3)
	o.CommandLine = "keepgen -garrison 7 -patrols 3 -key bench-keep-garrison -out k.yaml"
	doc, _, err := Generate(DefaultPlan(), o)
	require.NoError(t, err)
	require.Contains(t, string(doc), " garrison=7 patrols=3\n")
	got, err := OptionsFromHeader(doc)
	require.NoError(t, err)
	require.Equal(t, o, got)
	again, _, err := Generate(DefaultPlan(), got)
	require.NoError(t, err)
	require.Equal(t, doc, again)

	g := keepAt(t, 10000)
	require.NotContains(t, string(g.doc), "garrison=")
	got, err = OptionsFromHeader(g.doc)
	require.NoError(t, err)
	require.Zero(t, got.Garrison)
	require.Zero(t, got.Patrols)
}
