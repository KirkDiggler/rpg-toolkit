// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// TestAnAuthoredWorldWithAnOpenSecretDoorPreviewsAndStarts is the behavioural
// half of rpg-toolkit#1958 item 8: an authored world loads on the
// composition's own stand-ins, with this package supplying only the four
// capabilities it can answer for real.
//
// The world is the hardest legal one for a compile-only load: a concealment,
// so the load holds a check resolver and a witness, and its door standing
// OPEN, the one state a sight refresh would ask the witness about. Both
// callers of the authored load take it — AtlasOf previews it, StartSession
// proves it and persists it — and the preview is the author's whole truth.
func (s *ConcealSuite) TestAnAuthoredWorldWithAnOpenSecretDoorPreviewsAndStarts() {
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
// the only witness and check resolver this package declares are its real
// seams — a refusing or answering twin of encounter's stand-in is the copy
// item 8 deleted, and this is what keeps it deleted.
func TestTheAuthoredLoadIsTheCompositionsOwn(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}

	realSeams := map[string]string{"Perceivers": "witnessSeam", "ResolveCheck": "checkSeam"}
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
			if want, isSeam := realSeams[fn.Name.Name]; isSeam && fn.Recv != nil {
				if got := receiverName(fn.Recv); got != want {
					t.Errorf("%s: %s.%s — the composition's stand-in is the one answer for a world "+
						"nobody plays; only %s may answer it here",
						fset.Position(fn.Pos()), got, fn.Name.Name, want)
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
