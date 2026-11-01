// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// keepfixture_test.go opens the benchmark keep (rpg-project#740) and the mad
// king's castle as encounters, for the reads that must cost what the cells
// within reach cost rather than what the field costs (rpg-project#753, S2).
//
// THE KEEP IS GENERATED, NOT CHECKED IN: keepgen's embedded plan at a hex
// count and seed makes the same bytes every time, the same document
// `go run ./cmd/keepgen -hexes 10000 -seed 1 -key bench-keep` writes, so a
// fixture file would only be a second copy that could drift.

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
)

// castleFixture is the studio castle every castle route plays.
const castleFixture = "dungeonspec/testdata/world-builder-v4-castle.yaml"

var (
	keepDocsMu sync.Mutex
	keepDocs   = map[string]*keepDoc{}
)

// keepDoc is one site compiled once per test binary.
type keepDoc struct {
	once     sync.Once
	compiled dungeonspec.Compiled
	err      error
}

// compiledSite compiles the keep at hexes (or the castle file when hexes is
// zero) once per binary.
func compiledSite(tb testing.TB, hexes int) dungeonspec.Compiled {
	tb.Helper()
	if hexes == 0 {
		return compiledFile(tb, castleFixture)
	}
	name, read := keepDocument(hexes)
	return compiledOnce(tb, name, read)
}

// compiledFile compiles an authored document once per binary.
func compiledFile(tb testing.TB, path string) dungeonspec.Compiled {
	tb.Helper()
	return compiledOnce(tb, path, func() ([]byte, error) {
		return os.ReadFile(path) //nolint:gosec // a test reading its own fixture
	})
}

func compiledOnce(tb testing.TB, name string, read func() ([]byte, error)) dungeonspec.Compiled {
	tb.Helper()
	compiled, err := compiledOnceErr(name, read)
	require.NoError(tb, err, "%s compiles", name)

	return compiled
}

// compiledOnceErr is [compiledOnce] for a caller with no test to fail: the
// launch a [TestMain] starts ahead of the test that reads it.
func compiledOnceErr(name string, read func() ([]byte, error)) (dungeonspec.Compiled, error) {
	keepDocsMu.Lock()
	d, ok := keepDocs[name]
	if !ok {
		d = &keepDoc{}
		keepDocs[name] = d
	}
	keepDocsMu.Unlock()
	d.once.Do(func() {
		var raw []byte
		if raw, d.err = read(); d.err != nil {
			return
		}
		d.compiled, d.err = dungeonspec.Load(raw)
	})
	if d.err != nil {
		return dungeonspec.Compiled{}, d.err
	}
	compiled := d.compiled
	compiled.Field.Doors = append([]encounter.DoorInput(nil), compiled.Field.Doors...)
	compiled.Field.Concealments = append([]encounter.ConcealmentInput(nil), compiled.Field.Concealments...)

	return compiled, nil
}

// keepDocument is the generated keep's document at hexes.
func keepDocument(hexes int) (string, func() ([]byte, error)) {
	return fmt.Sprintf("keep-%d", hexes), func() ([]byte, error) {
		opts := keepgen.DefaultOptions()
		opts.Hexes, opts.Seed, opts.Key = hexes, 1, "bench-keep"
		raw, _, err := keepgen.Generate(keepgen.DefaultPlan(), opts)
		return raw, err
	}
}

// keepPlayers is the party's member ids, best seat first.
var keepPlayers = []encounter.MemberID{"p0", "p1", "p2", "p3"}

// openSite opens the keep at hexes (the castle at zero) with the party in its
// first four seats and, when monsters is true, every monster where the file
// posts it. Sight is the rulebook's 120 feet.
func openSite(tb testing.TB, hexes int, monsters bool) (*encounter.Encounter, dungeonspec.Compiled) {
	tb.Helper()
	compiled := compiledSite(tb, hexes)

	return openCompiled(tb, compiled, seatedParty(compiled), monsters, findsNothing{}), compiled
}

// seatedParty is the four players in the site's first four seats.
func seatedParty(compiled dungeonspec.Compiled) []encounter.MemberInput {
	out := make([]encounter.MemberInput, 0, len(keepPlayers))
	for i, id := range keepPlayers {
		if i >= len(compiled.PartyStart) {
			break
		}
		out = append(out, encounter.MemberInput{ID: id, Kind: encounter.KindPlayer, Position: compiled.PartyStart[i].At})
	}
	return out
}

// openCompiled opens a compiled site with these players, the file's monsters
// when monsters is true, and this discovery resolver.
func openCompiled(
	tb testing.TB, compiled dungeonspec.Compiled, players []encounter.MemberInput, monsters bool,
	resolver encounter.CheckResolver,
) *encounter.Encounter {
	tb.Helper()
	enc, err := openCompiledErr(compiled, players, monsters, resolver)
	require.NoError(tb, err)

	return enc
}

// openCompiledErr is [openCompiled] for a caller with no test to fail.
func openCompiledErr(
	compiled dungeonspec.Compiled, players []encounter.MemberInput, monsters bool, resolver encounter.CheckResolver,
) (*encounter.Encounter, error) {
	members := append([]encounter.MemberInput(nil), players...)
	if monsters {
		for _, m := range compiled.Monsters {
			members = append(members, encounter.MemberInput{
				ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
				Faction: encounter.FactionID(m.Faction), Table: m.Table, Options: m.Options,
				Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees,
			})
		}
	}
	endings := append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...)
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		Endings: endings,
		Capabilities: encounter.Capabilities{
			Sight:         &sightList{fallback: 24},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Roller:        rollsLowest{},
			CheckResolver: resolver,
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	return enc, err
}

// soldierTemper gives every word in a monster's mix the empty profile — a
// soldier — so a garrison deals without this host defining what a coward
// means. The castle's own host does the same.
func soldierTemper(temper encounter.Temper) encounter.Temper {
	if len(temper.Mix) == 0 {
		return temper
	}
	temper.Profiles = make(map[string]encounter.TemperProfile, len(temper.Mix))
	for word := range temper.Mix {
		temper.Profiles[word] = encounter.TemperProfile{}
	}

	return temper
}

var (
	sharedKeepOnce sync.Once
	sharedKeep     *encounter.Encounter
)

// sharedKeep10k is ONE opened 10k keep for the package's read-only tests:
// opening it is most of what those tests cost under -race, and none of them
// changes the run.
func sharedKeep10k(tb testing.TB) *encounter.Encounter {
	tb.Helper()
	// Two players, not four: every observer sweeps the keep when it opens,
	// and two are all the shared tests ask about.
	sharedKeepOnce.Do(func() {
		compiled := compiledSite(tb, 10000)
		sharedKeep = openCompiled(tb, compiled, seatedParty(compiled)[:2], false, findsNothing{})
	})
	require.NotNil(tb, sharedKeep, "the shared keep opened")

	return sharedKeep
}
