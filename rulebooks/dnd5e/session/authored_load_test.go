// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// TestLoadingAnAuthoredWorldNeedsNothingSessionInvented is the behavioural
// half of rpg-toolkit#1958 item 8: an authored world loads on the
// composition's own stand-ins, with this package supplying only the four
// capabilities it can answer for real.
//
// WHAT IT DOES NOT PROVE. It passes with session's old stand-ins put back as
// well: neither StartSession nor AtlasOf drives a turn, rolls a check or
// refreshes sight, so the refusing witness and driver those stand-ins were and
// encounter's answering ones are never asked on either path, and nothing
// observable differs between them today. What this pins is that a world
// carrying concealed structure — so the load must hold a check resolver and a
// witness — with its secret door standing open loads on what encounter
// supplies, through both callers, and that the preview is the author's whole
// truth. [TestTheAuthoredLoadIsTheCompositionsOwn] is the check that the
// stand-ins are encounter's.
func (s *ConcealSuite) TestLoadingAnAuthoredWorldNeedsNothingSessionInvented() {
	world := concealedWorld(s.T(), encounter.DoorIsOpen())
	s.startWith(world, armedSearcher("alice"), armedFighter("bob"), dullEyed("carol"))

	preview, err := s.mgr.AtlasOf(context.Background(), &session.AtlasOfInput{World: world})
	s.Require().NoError(err, "the preview loads the world nobody has started")
	var regions []string
	for _, r := range preview.Regions {
		regions = append(regions, r.ID)
	}
	s.Contains(regions, "vault", "the author's preview draws the concealed room")
}

// TestTheAuthoredLoadIsTheCompositionsOwn is the structural half: session
// holds no copy of an encounter stand-in.
//
// loadAuthored must start from encounter.CompileOnlyLoad and build no
// LoadEncounterInput of its own, so a capability added to LoadEncounter is
// stood in by the composition and this package's call does not change. And
// every witness, check resolver and turn driver this package declares is one
// of a closed list of real ones — a refusing or answering twin of encounter's
// stand-in is the copy item 8 deleted, and this is what keeps it deleted.
//
// The list is keyed by method name and receiver, so a NEW legitimate
// implementation (a wrapper around witnessSeam, a host-facing driver beside
// Pass) fails here until it is added to the list on purpose — the cost of the
// guard, paid once per new seam.
func TestTheAuthoredLoadIsTheCompositionsOwn(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}

	realSeams := map[string][]string{
		"Perceivers":   {"witnessSeam"},
		"ResolveCheck": {"checkSeam"},
		// Both Act shapes: encounter.Driver (compelledDriver, turnDriverSeam)
		// and this package's TurnDriver (Pass, tableDriver).
		"Act": {"compelledDriver", "turnDriverSeam", "Pass", "tableDriver"},
	}
	foundLoad := false
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parsing %s: %v", path, perr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if allowed, isSeam := realSeams[fn.Name.Name]; isSeam && fn.Recv != nil {
				if got := receiverName(fn.Recv); !slices.Contains(allowed, got) {
					t.Errorf("%s: %s.%s — the composition's stand-in is the one answer for a world "+
						"nobody plays; only %v may answer it here",
						fset.Position(fn.Pos()), got, fn.Name.Name, allowed)
				}
			}
			if fn.Name.Name != "loadAuthored" {
				continue
			}
			foundLoad = true
			callsCompileOnly, literals := false, 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.CallExpr:
					if sel, ok := node.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "CompileOnlyLoad" {
						callsCompileOnly = true
					}
				case *ast.CompositeLit:
					if sel, ok := node.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "LoadEncounterInput" {
						literals++
					}
				}
				return true
			})
			if !callsCompileOnly {
				t.Errorf("%s: loadAuthored does not start from encounter.CompileOnlyLoad",
					fset.Position(fn.Pos()))
			}
			if literals != 0 {
				t.Errorf("%s: loadAuthored builds its own LoadEncounterInput — "+
					"the capabilities it does not answer are encounter's to stand in",
					fset.Position(fn.Pos()))
			}
		}
	}
	if !foundLoad {
		t.Fatal("found no loadAuthored: this test has stopped testing anything")
	}
}

// receiverName is a method receiver's type name, pointer or not.
func receiverName(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}
