// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package banditcamp_test

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/banditcamp"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/dnd5eresolver"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scripted"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// toolkitPrefix is how a toolkit module import is recognised.
const toolkitPrefix = "github.com/KirkDiggler/rpg-toolkit/"

// InvariantSuite asserts the three standing claims that hold across every path:
// nobody is gated, nothing present is stored, and the kernel does not know what
// game it is in.
type InvariantSuite struct {
	suite.Suite

	ctx    context.Context
	sheets map[journal.EntityID]*character.Character
}

func TestInvariantSuite(t *testing.T) {
	suite.Run(t, new(InvariantSuite))
}

func (s *InvariantSuite) SetupSuite() {
	s.ctx = context.Background()

	crew, err := banditcamp.Crew(s.ctx)
	s.Require().NoError(err)
	s.sheets = crew
}

// rig is one playable camp over a written-down sequence of d20 results.
type rig struct {
	w *world.World
}

func (s *InvariantSuite) rig(rolls ...int) rig {
	s.T().Helper()

	resolver, err := dnd5eresolver.New(dnd5eresolver.Config{
		Sheets: s.sheets,
		Roller: scripted.NewRoller(rolls...),
		Bus:    events.NewEventBus(),
	})
	s.Require().NoError(err)

	built, err := world.New(world.Config{Scenario: banditcamp.Scenario(), Resolver: resolver})
	s.Require().NoError(err)

	return rig{w: built}
}

// ---------------------------------------------------------------------------
// Nobody is gated.
// ---------------------------------------------------------------------------

func (s *InvariantSuite) TestAnyActorMayAttemptAnyPath() {
	actors := []journal.EntityID{banditcamp.Rook, banditcamp.Brann, banditcamp.Sela}
	attempts := []struct {
		verb   world.VerbName
		target journal.EntityID
	}{
		{banditcamp.Sneak, banditcamp.Camp},
		{banditcamp.Assassinate, banditcamp.Leader},
		{banditcamp.Impersonate, banditcamp.Camp},
		{banditcamp.Persuade, banditcamp.Camp},
	}

	// The barbarian sneaks, the rogue preaches, the paladin knifes a man in the
	// dark. Every one of these is judged, and not one of them is refused.
	for _, actor := range actors {
		for _, attempt := range attempts {
			r := s.rig(11)

			result, err := r.w.Act(s.ctx, world.Act{
				Verb: attempt.verb, Actor: actor, Target: attempt.target,
				Bystanders: []journal.EntityID{banditcamp.Lieutenant},
			})
			s.Require().NoErrorf(err, "%s attempting %s", actor, attempt.verb)
			s.Truef(result.Fact.Outcome.Contested, "%s attempting %s was never judged", actor, attempt.verb)
			s.Equalf(actor, result.Fact.Actor, "%s attempting %s lost its attribution", actor, attempt.verb)
		}
	}
}

func (s *InvariantSuite) TestProficiencyOnlyTiltsTheDice() {
	sneak := func(actor journal.EntityID, roll int) journal.Fact {
		r := s.rig(roll)
		result, err := r.w.Act(s.ctx, world.Act{
			Verb: banditcamp.Sneak, Actor: actor, Target: banditcamp.Camp,
		})
		s.Require().NoError(err)

		return result.Fact
	}

	expert := sneak(banditcamp.Rook, 10)
	oaf := sneak(banditcamp.Brann, 10)

	s.Run("the same die reads differently, and only by the sheet", func() {
		// Rook has expertise in Stealth on DEX 16 (+7); Brann has DEX 12 and no
		// proficiency (+1). Six points of sheet, and nothing else.
		s.True(expert.Outcome.Succeeded)
		s.False(oaf.Outcome.Succeeded)
		s.Equal(6, expert.Outcome.Margin-oaf.Outcome.Margin)
	})

	s.Run("the tilt is a tilt, not a gate", func() {
		lucky := sneak(banditcamp.Brann, 20)
		s.True(lucky.Outcome.Succeeded)
		s.Equal(banditcamp.FactInfiltration, lucky.Kind)
		s.False(lucky.Audience.Includes(banditcamp.Camp))
	})

	s.Run("the transcript says what the rulebook did", func() {
		s.Contains(expert.Outcome.Detail, "stealth")
		s.Contains(expert.Outcome.Detail, "d20(10)+7 = 17 vs DC 13")
	})
}

// ---------------------------------------------------------------------------
// Nothing present is stored.
// ---------------------------------------------------------------------------

// snapshot is the observable surface of a derived state, in a form two
// different worlds can be compared on.
type snapshot struct {
	Edges    []string
	Leads    journal.EntityID
	Alerted  bool
	Defeated bool
	Regard   int
	Posture  string
}

func snap(state *graph.State) snapshot {
	current := state.Edges()
	edges := make([]string, 0, len(current))
	for _, e := range current {
		edges = append(edges, string(e.From)+" "+string(e.Rel)+" "+string(e.To))
	}
	slices.Sort(edges)

	return snapshot{
		Edges:    edges,
		Leads:    state.Occupant(banditcamp.Leads, banditcamp.Camp),
		Alerted:  state.Flagged(banditcamp.Alerted, banditcamp.Camp),
		Defeated: state.Flagged(banditcamp.Defeated, banditcamp.Camp),
		Regard:   state.Count(banditcamp.Regard, banditcamp.Camp, banditcamp.Party),
		Posture:  state.Label(banditcamp.Posture, banditcamp.Camp),
	}
}

func (s *InvariantSuite) TestPresentStateIsDerivedByFoldAndNeverStored() {
	r := s.rig(19, 15) // The changeling: the kill lands, the claim lands.

	fresh, err := graph.New(banditcamp.Declaration())
	s.Require().NoError(err)

	start := snap(r.w.View(banditcamp.Camp))

	_, err = r.w.Act(s.ctx, world.Act{
		Verb: banditcamp.Assassinate, Actor: banditcamp.Rook, Target: banditcamp.Leader,
	})
	s.Require().NoError(err)
	afterKill := snap(r.w.View(banditcamp.Camp))

	_, err = r.w.Act(s.ctx, world.Act{
		Verb: banditcamp.Impersonate, Actor: banditcamp.Rook, Target: banditcamp.Camp,
	})
	s.Require().NoError(err)
	afterClaim := snap(r.w.View(banditcamp.Camp))

	s.Run("the run actually moved the world", func() {
		s.Equal(start, afterKill) // The camp saw neither, so its present is unchanged.
		s.NotEqual(start, afterClaim)
		s.Equal(banditcamp.Rook, afterClaim.Leads)
	})

	s.Run("a world that watched it all holds nothing a fresh one does not", func() {
		s.Equal(afterClaim, snap(fresh.StateFor(banditcamp.Camp, r.w.Journal())))
	})

	s.Run("rewinding the journal rewinds the present", func() {
		empty := journal.New()
		s.Equal(start, snap(r.w.Graph().StateFor(banditcamp.Camp, empty)))
		s.Equal(start, snap(fresh.StateFor(banditcamp.Camp, empty)))
	})

	s.Run("replaying a prefix reproduces that moment exactly", func() {
		prefix := journal.New()
		for _, f := range r.w.Journal().All()[:1] {
			_, err := prefix.Append(f)
			s.Require().NoError(err)
		}
		s.Equal(afterKill, snap(r.w.Graph().StateFor(banditcamp.Camp, prefix)))
	})

	s.Run("deriving twice from the same facts agrees", func() {
		s.Equal(afterClaim, snap(r.w.View(banditcamp.Camp)))
	})

	s.Run("and nothing declined to fold along the way", func() {
		s.Empty(r.w.View(banditcamp.Camp).Refusals())
		s.Empty(r.w.Truth().Refusals())
	})
}

// ---------------------------------------------------------------------------
// The kernel does not know what game it is in.
// ---------------------------------------------------------------------------

func (s *InvariantSuite) TestKernelPackagesImportNoRulebook() {
	// The kernel graduated out of this repository into its own module
	// (github.com/KirkDiggler/rpg-toolkit/world, #1333/#1334): journal, graph,
	// quest, goal, and the composer no longer live under examples/world, so
	// this can no longer be checked by parsing local directories — that
	// mechanism only ever proved the invariant for a checkout where the
	// kernel happened to sit monorepo-adjacent, which a real consumer (an
	// rpg-api import, say) is not. The resolved build graph is the honest
	// check now: world's own go.mod requires only testify, so nothing under
	// it can reach a rulebook no matter who is asking.
	for _, pkg := range []string{
		toolkitPrefix + "world",
		toolkitPrefix + "world/journal",
		toolkitPrefix + "world/graph",
		toolkitPrefix + "world/quest",
		toolkitPrefix + "world/goal",
	} {
		for _, dep := range s.worldModuleDeps(pkg) {
			s.NotContainsf(dep, "rulebooks/", "%s depends on a rulebook: %s", pkg, dep)
		}
	}

	// scripted is the one thing left here that the kernel packages used to
	// share a directory with. It still imports nothing at all, toolkit or
	// otherwise, so it cannot smuggle a rulebook into anything that reaches
	// for it.
	s.Empty(s.toolkitImportsOf("../scripted"))
}

// worldModuleDeps returns pkg's full transitive dependency closure, filtered
// to toolkit modules, via the resolved build graph rather than source on
// disk. This is what makes the check honest for the graduated kernel: it
// gives the same answer for this monorepo checkout and for a bare `go get`
// in a repository that has never heard of examples/world.
func (s *InvariantSuite) worldModuleDeps(pkg string) []string {
	s.T().Helper()

	out, err := exec.Command("go", "list", "-f", "{{join .Deps \"\\n\"}}", pkg).Output()
	s.Require().NoError(err)

	var deps []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(line, toolkitPrefix) {
			deps = append(deps, line)
		}
	}

	return deps
}

func (s *InvariantSuite) TestScenariosDoNotDependOnEachOther() {
	// The second-instance law, made mechanical. Everything two scenarios both
	// needed is one layer up — the act loop, the verb vocabulary, the resolver
	// seam, the dice. If one scenario ever imports another, something they
	// share failed to get promoted and is being borrowed sideways instead.
	scenarios := map[string]string{
		"../banditcamp":  "examples/world/hostagecamp",
		"../hostagecamp": "examples/world/banditcamp",
	}

	for dir, forbidden := range scenarios {
		for _, imported := range s.toolkitImportsOf(dir) {
			s.NotContainsf(imported, forbidden, "%s reaches sideways into another scenario", dir)
		}
	}
}

func (s *InvariantSuite) TestOnlyOnePackageTeachesTheWorldADieRoll() {
	// Scenarios import a rulebook for their cast; exactly one package imports
	// one to resolve an attempt. Two would mean two answers to "did that
	// work". The kernel and composer are proven clean of rulebooks by
	// TestKernelPackagesImportNoRulebook (they are a separate module now, and
	// that test asks the build graph, not this directory tree); what is left
	// to check locally is whatever still lives in examples/world.
	adapters := 0
	beneath := []string{"../scripted", "../dnd5eresolver"}
	for _, dir := range beneath {
		for _, imported := range s.toolkitImportsOf(dir) {
			if strings.Contains(imported, "rulebooks/") {
				adapters++

				break
			}
		}
	}
	s.Equal(1, adapters, "the resolver seam has exactly one rulebook-facing implementation here")
}

// toolkitImportsOf returns every toolkit import in a package directory,
// deduplicated and sorted. Test files count: a kernel test reaching for a
// rulebook is the same breach as the code doing it.
func (s *InvariantSuite) toolkitImportsOf(dir string) []string {
	s.T().Helper()

	entries, err := os.ReadDir(dir)
	s.Require().NoError(err)

	var out []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		out = append(out, s.parseToolkitImports(filepath.Join(dir, entry.Name()))...)
	}
	slices.Sort(out)

	return slices.Compact(out)
}

func (s *InvariantSuite) parseToolkitImports(path string) []string {
	s.T().Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	s.Require().NoError(err)

	var out []string
	for _, spec := range file.Imports {
		unquoted, err := strconv.Unquote(spec.Path.Value)
		s.Require().NoError(err)
		if strings.HasPrefix(unquoted, toolkitPrefix) {
			out = append(out, unquoted)
		}
	}

	return out
}
