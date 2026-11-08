// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// garrisonDoc is the part of a generated document the garrison writes, read
// back as plain YAML: what the file says, not what the generator meant.
type garrisonDoc struct {
	Definitions map[string]struct {
		Ref   string `yaml:"ref"`
		Table map[string][]struct {
			Hold *struct{} `yaml:"hold"`
		} `yaml:"table"`
	} `yaml:"behaviorDefinitions"`
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
				Faction   string `yaml:"faction"`
				Behaviors []struct {
					ID    string               `yaml:"id"`
					Use   string               `yaml:"use"`
					Items []struct{ Q, R int } `yaml:"items"`
				} `yaml:"behaviors"`
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

	require.Equal(t, "keepgen:behaviors:patrol", d.Definitions["patrol"].Ref)
	post, ok := d.Definitions["post"]
	require.True(t, ok)
	require.NotNil(t, post.Table["hold"][0].Hold)

	decls := d.Room.Room.MonsterDeclarations
	require.Len(t, d.Room.Room.MonsterBindings, 5)
	for i := 0; i < 5; i++ {
		b, ok := d.Room.Room.MonsterBindings[decls[i].ID]
		require.True(t, ok, "monster %d (%s) is bound", i, decls[i].ID)
		require.Equal(t, "garrison", b.Faction)
		require.Len(t, b.Behaviors, 1)
		require.Equal(t, "duty", b.Behaviors[0].ID)
		if i >= 2 {
			require.Equal(t, "post", b.Behaviors[0].Use)
			continue
		}
		require.Equal(t, "patrol", b.Behaviors[0].Use)
		path := b.Behaviors[0].Items
		require.Len(t, path, 3)
		start := decls[i].StartingCell.Location
		require.Equal(t, start, path[2], "the loop returns to the authored start")
		require.NotEqual(t, start, path[0], "the first opportunity starts its next leg, not a synthetic arrival")
	}
}

// TestGarrison_IsNeutralToTheParty: the garrison document declares the
// garrison neutral to the party, the castle's disposition, and the loader
// compiles it so. Undeclared, the pair would default to hostile
// ([encounter.DefaultStance]) and a fight would form at the start.
//
// THE PATROL STILL WALKS. Its row is `when: { enemy: none }`, and an enemy
// is an OPPOSED sighting only: [encounter.projectFacts] (facts.go) skips a
// sighting that is not opposed, and opposed is `StanceBetween == hostile`
// (world.go, Encounter.opposed). A neutral party in sight is no enemy, so
// `enemy: none` holds and the patrol row fires.
func TestGarrison_IsNeutralToTheParty(t *testing.T) {
	doc, _, err := Generate(DefaultPlan(), garrisonOptions(5, 2))
	require.NoError(t, err)
	compiled, err := Prove(doc)
	require.NoError(t, err)
	require.Equal(t, encounter.StanceHostile, encounter.DefaultStance(garrisonFaction, encounter.FactionParty),
		"without the declaration the garrison would fight the party")
	var found []encounter.DispositionInput
	for _, d := range compiled.Field.Dispositions {
		pair := map[encounter.FactionID]bool{d.Between[0]: true, d.Between[1]: true}
		if pair[garrisonFaction] && pair[encounter.FactionParty] {
			found = append(found, d)
		}
	}
	require.Len(t, found, 1)
	require.Equal(t, encounter.StanceNeutral, found[0].Stance)
	require.Empty(t, found[0].Until, "the stance is static")

	off := keepAt(t, 10000)
	require.Empty(t, off.compiled.Field.Dispositions, "no garrison, no disposition")
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
const goldenGarrison = "0682f23aa193e3a8fac4b723d47bf9935542e880c0805bf591c0bf44210757ce"

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
		if !ok || b.Behaviors[0].Use != "patrol" {
			continue
		}
		patrols++
		home, ok := regionOf[spatial.Position{X: float64(m.StartingCell.Location.Q), Y: float64(m.StartingCell.Location.R)}]
		require.True(t, ok, "%s stands on floor", m.ID)
		path := b.Behaviors[0].Items
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

// TestGarrison_PostsListen (rpg-project#766 S4): with the option on, every
// post carries the listening recipe — posted, waking only to a sound within
// six hexes — and the patrols keep the default; the document loads, the
// header reads the option back, and with the option off the bytes are the
// garrison's own.
func TestGarrison_PostsListen(t *testing.T) {
	o := garrisonOptions(5, 2)
	o.PostsListen = true
	doc, _, err := Generate(DefaultPlan(), o)
	require.NoError(t, err)
	compiled, err := Prove(doc)
	require.NoError(t, err)

	posts, patrols := 0, 0
	for _, m := range compiled.Monsters {
		if m.Faction != garrisonFaction {
			continue
		}
		require.NotNil(t, m.Behaviors)
		recipe := m.Behaviors.Instances[0].Instance.Recipe
		if recipe.Ref.ID == "patrol" {
			patrols++
			continue
		}
		posts++
		require.Equal(t, "posted", recipe.Initial, "%s starts posted", m.MemberID)
		require.Equal(t, "heard", string(recipe.States["posted"].Transitions[0].On))
		require.Equal(t, "turn", string(recipe.States["posted"].Transitions[1].On), "fight opportunity is explicit, not world time")
		_, err := encounter.CheckBehaviors(&encounter.CheckBehaviorsInput{Behaviors: *m.Behaviors})
		require.NoError(t, err)
	}
	require.Equal(t, 3, posts, "every post listens")
	require.Equal(t, 2, patrols, "the patrols run the default")

	back, err := OptionsFromHeader(doc)
	require.NoError(t, err)
	require.True(t, back.PostsListen, "the header records the option")

	off, _, err := Generate(DefaultPlan(), garrisonOptions(5, 2))
	require.NoError(t, err)
	require.NotContains(t, string(off), "use: listening-post", "off, posts bind the ordinary post definition")
}
