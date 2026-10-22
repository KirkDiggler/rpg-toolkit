// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package castletest plays the mad king's castle, the studio document the
// QA script runs on (rpg-project#605, rpg-project#639), one route at a
// time. It is its own package so its routes, which walk the castle round
// after round, run under a test budget of their own beside dungeonspec's
// compiler tests.
package castletest

// host_test.go is what the castle's host borrows from dungeonspec's tests:
// the least interesting capabilities, the content loader, and the readings
// of a run. They are copies, because test helpers do not cross a package
// boundary and exporting them would put a test-only surface on a published
// module.
//
// ONE LOWERING PER FIXTURE PER PACKAGE. Every route compiles the same file;
// loadContent compiles each path once and hands every caller its own copy of
// the door list, the one part of the compiled content a host rewrites before
// it opens the run. Each route still opens its own Encounter.

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// lowered is one fixture compiled once for the package.
type lowered struct {
	once     sync.Once
	compiled dungeonspec.Compiled
	err      error
}

var (
	loweringsMu sync.Mutex
	lowerings   = map[string]*lowered{}
)

// loadContent is the package's one lowering of the file at path, with a
// door list of the caller's own.
func loadContent(t testing.TB, path string) dungeonspec.Compiled {
	t.Helper()
	loweringsMu.Lock()
	l, ok := lowerings[path]
	if !ok {
		l = &lowered{}
		lowerings[path] = l
	}
	loweringsMu.Unlock()

	l.once.Do(func() {
		raw, err := os.ReadFile(path) //nolint:gosec // a test reading its own fixture
		if err != nil {
			l.err = err
			return
		}
		l.compiled, l.err = dungeonspec.Load(raw)
	})
	require.NoError(t, l.err, "%s compiles", path)

	compiled := l.compiled
	compiled.Field.Doors = slices.Clone(compiled.Field.Doors)

	return compiled
}

// loadSource compiles one authored document held in the test.
func loadSource(t *testing.T, source string) dungeonspec.Compiled {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)

	return compiled
}

// unlimitedSight is far enough that no scene here is bounded by it.
const unlimitedSight = 1_000_000

// everyoneSeesTheWholeMap installs a sight model with no distance term, so
// every sighting is decided by geometry alone.
type everyoneSeesTheWholeMap struct{}

func (everyoneSeesTheWholeMap) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = unlimitedSight
	}

	return out, nil
}

// everyoneStanding reports nobody down.
type everyoneStanding struct{}

func (everyoneStanding) Standing([]encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

func (everyoneStanding) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		assessment.Members = append(assessment.Members, encounter.MemberParticipation{
			Member: id, Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait,
		})
	}
	return assessment, nil
}

// orderAsGiven returns the roster untouched: these scenes assert that a fight
// formed, never the order it formed in.
type orderAsGiven struct{}

func (orderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}

// passDriver ends every unplayed member's turn with no other effect.
type passDriver struct{}

func (passDriver) Act(encounter.MonsterView) (encounter.Decision, error) {
	return encounter.Decision{Intent: encounter.Pass{}}, nil
}

// noAttacksExpected is the Striker of a scene in which nobody swings.
type noAttacksExpected struct{}

func (noAttacksExpected) Strike(
	context.Context, *encounter.Encounter, encounter.MemberID, encounter.MemberID, core.Ref,
) error {
	return errors.New("castletest: this scene never attacks")
}

// quietAnnouncer hears every boundary and does nothing.
type quietAnnouncer struct{}

func (quietAnnouncer) Announce(context.Context, *encounter.Encounter, []encounter.Boundary) error {
	return nil
}

// quietMover hears every step and does nothing.
type quietMover struct{}

func (quietMover) Move(context.Context, *encounter.Encounter, encounter.MoveStep) error {
	return nil
}

// nothingIsEverFound resolves every check as a failure.
type nothingIsEverFound struct{}

func (nothingIsEverFound) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return &encounter.ResolveCheckOutput{Beaten: false}, nil
}

func (nothingIsEverFound) ResolveDiscoveryCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return &encounter.ResolveCheckOutput{Beaten: false}, nil
}

// nobodyPerceivesAnything answers that no member perceives any open
// concealed door.
type nobodyPerceivesAnything struct{}

func (nobodyPerceivesAnything) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return nil, nil
}

// zeroSheets states no speed, no actions and no strategy for anybody.
type zeroSheets struct{}

func (zeroSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{}
	}

	return out, nil
}

// zonesOf is a member's red floor.
func zonesOf(t *testing.T, enc *encounter.Encounter, who encounter.MemberID) []encounter.WatchZone {
	t.Helper()
	out, err := enc.WatchZonesFor(&encounter.WatchZonesInput{Member: who})
	require.NoError(t, err)
	require.NotNil(t, out.Zones, "the zones are never nil")

	return out.Zones
}

// cellSet is a list of cells as a set.
func cellSet(cells []spatial.Position) map[spatial.Position]bool {
	out := make(map[spatial.Position]bool, len(cells))
	for _, c := range cells {
		out[c] = true
	}

	return out
}

// facingOf is a member's facing on the roster.
func facingOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) float64 {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == id {
			return m.Facing
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return 0
}

// positionOf is where a member stands now.
func positionOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return spatial.Position{}
}

// dwellOf is a member's dwell as the run persists it, or nil.
func dwellOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) *encounter.DwellData {
	t.Helper()
	for _, m := range enc.ToData().Members {
		if m.ID == id {
			return m.Dwell
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return nil
}

// coneBearing is the bearing from one cell to another on the castle's
// pointy-top floor, map north zero, clockwise.
func coneBearing(from, to spatial.Position) float64 {
	plane := spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{
		Orientation: spatial.HexOrientationPointyTop, CellWidth: encounter.FeetPerCell,
	})
	east, _ := plane.Bearing(from, to)
	north := east + 90
	for north >= 360 {
		north -= 360
	}
	for north < 0 {
		north += 360
	}

	return north
}

// holding is observer's sight holding of subject, or nil.
func (s *kingScene) holding(observer, subject encounter.MemberID) *perception.Holding {
	s.t.Helper()
	held, err := s.enc.View(&encounter.ViewInput{Member: observer})
	require.NoError(s.t, err)
	for i := range held {
		if held[i].Channel == perception.Sight && encounter.MemberID(held[i].Subject) == subject {
			return &held[i]
		}
	}

	return nil
}

// seesLive reports whether observer holds subject live on sight now.
func (s *kingScene) seesLive(observer, subject encounter.MemberID) bool {
	s.t.Helper()
	h := s.holding(observer, subject)

	return h != nil && len(h.CurrentVia) > 0
}

// seenNow is a member's fog-of-war seen-now cells, as a set.
func (s *kingScene) seenNow(member encounter.MemberID) map[spatial.Position]bool {
	s.t.Helper()
	out, err := s.enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: member})
	require.NoError(s.t, err)

	return cellSet(out.Cells)
}
