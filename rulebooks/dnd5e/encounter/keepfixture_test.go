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

// smallKeepHexes is [smallKeepPlan]'s whole floor at scale one.
const smallKeepHexes = 1230

// smallKeepPlan is the benchmark keep's own plan at the smallest size its
// doors allow (rpg-project#909): the same 29 rooms on the same sides, the
// courtyard, both grounds and all 36 doors, so every footprint door, every
// wall a door is cut in and every room kind is the 10k keep's. Every room is
// 5 hexes wide and 3 deep: keepgen cuts a door only in a wall at least 4 long
// and 4 hexes across measure 3.46, while 3 rows measure enough. The hidden
// stash and the tower are 3 wide, since their doors are in the walls they
// share with their neighbours along the side, which keeps the north and
// south sides one length; the larder is 6 deep, so the east side fills the
// west side's rows; the grounds are 3 across. 1,230 hexes against the
// keep's 10,000: fewer cells, the same keep.
func smallKeepPlan() *keepgen.Plan {
	p := keepgen.DefaultPlan()
	for i := range p.Rooms {
		r := &p.Rooms[i]
		r.Width, r.Depth = 5, 3
		switch r.ID {
		case "hidden-stash", "tower":
			r.Width = 3
		case "larder":
			r.Depth = 6
		}
	}
	p.Grounds.West.Width, p.Grounds.South.Depth = 3, 3

	return p
}

// smallKeep compiles [smallKeepPlan] once per binary, the way [garrisonKeep]
// and [openGarrisonKeep] compile the 10k one: seed 1 and the default
// densities. With garrison it is the observers' keep: four monsters per 100
// hexes rather than one, every one given orders, posts listening and one
// walking a patrol. The 10k garrison is 99 of its 100 monsters with 20
// walking, and its open grounds put creatures one cell past sight of a door;
// in 1,230 hexes the denser garrison is what puts them there, and a patrol's
// route rarely fits a room 3 deep, so one walks.
func smallKeep(tb testing.TB, garrison bool) dungeonspec.Compiled {
	tb.Helper()
	name, read := smallKeepDocument(garrison)
	return compiledOnce(tb, name, read)
}

// smallKeepDocument is [smallKeep]'s document, by its compile-once name.
func smallKeepDocument(garrison bool) (string, func() ([]byte, error)) {
	key := "small-keep"
	if garrison {
		key += "-garrison"
	}
	return key, func() ([]byte, error) {
		opts := keepgen.DefaultOptions()
		opts.Hexes, opts.Seed, opts.Key = smallKeepHexes, 1, "small-keep"
		plan := smallKeepPlan()
		if garrison {
			opts.MonsterDensity = 4
			_, report, err := keepgen.Generate(plan, opts)
			if err != nil {
				return nil, err
			}
			opts.Garrison, opts.Patrols, opts.PostsListen = report.Monsters, 1, true
		}
		raw, report, err := keepgen.Generate(plan, opts)
		if err == nil && report.Hexes != smallKeepHexes {
			err = fmt.Errorf("the small keep is %d hexes, not %d", report.Hexes, smallKeepHexes)
		}
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
				Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees, Behaviors: m.Behaviors, Machine: m.Machine, BlockingStates: m.BlockingStates, Posts: m.Posts,
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
