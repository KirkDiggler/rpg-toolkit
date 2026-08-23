// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// oneplace_internal_test.go asks the SOURCE a question no behavioural test can
// ask: is this decided in one place?
//
// Duplication is invisible to behaviour. Two implementations of one rule agree
// on every fixture written while they still agree, which is why this package
// has grown the same defect three times over: the walk resolved in the session
// and again in the pump (#1059); the ReachedPosition ending decided in
// firedReachedPosition and again inside Join (#1059, deleted by #1106); region
// ownership answered by roomAt, by ownedByAnyRoom and by an inline copy in
// outcome validation (#1108, this slice). Every one was found by reading, and
// every one had a green suite while it stood.
//
// So these two tests read. Each names one syntactic move a second
// implementation of its rule cannot avoid making, and asserts that move happens
// in exactly one function body. Neither checks style, and neither counts lines.
//
// The technique is the house's, not a new one: rulebooks/dnd5e/session's
// boundary_test.go parses its own package to prove no inner type crosses the
// seam, on the same reasoning — "the guarantee is mechanical rather than
// aspirational ... instead of trusting that reviewers will spot a leaked type in
// a diff two years from now".

// TestRegionOwnershipIsAskedInOneFunction.
//
// Since regions replaced rooms (rpg-project#256) there is no arithmetic left
// in asking which region owns a cell: the answer is a map, built once at
// construction, and READING THAT MAP is the one move a second implementation
// cannot avoid. So: every function that indexes the owner map is named here,
// and there are exactly two — the builder, which reads it to refuse a cell
// painted twice (W2), and the lookup, which reads it to answer.
//
// This used to pin spatial.Position.Subtract, the move the room chain's mask
// made (footprintHolds, and regionAt before it). That move is gone with the
// origin it subtracted; the question it guarded is the same one.
//
// If a legitimate third reader ever appears, the honest fix is to name it in
// the expected list, not to delete the test. The point is that a second one
// becomes a decision somebody makes on purpose rather than a copy that
// arrives unnoticed.
func TestRegionOwnershipIsAskedInOneFunction(t *testing.T) {
	readers := functionsWhoseBodyReads(t, func(n ast.Node) bool {
		index, ok := n.(*ast.IndexExpr)
		if !ok {
			return false
		}
		sel, ok := index.X.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "owner"
	})
	require.Equal(t, []string{"compileRegions", "regionOf"}, readers,
		"region ownership must be answered in exactly one place (rpg-toolkit#1108, rpg-project#256): "+
			"a second function reading the owner map is a second answer to which region holds a cell")
}

// TestTheOneConversionIsCalledInOnePlace.
//
// HexCellAt is the ONE conversion from an authored [col,row] pair to the
// absolute axial cell it names (rpg-project#256; the rpg-toolkit#1141/#1150
// lesson). Inside this package exactly one function may call it — the
// field's own cellAt — so every authored coordinate goes through the same
// door and a second reading of the basis cannot grow back.
func TestTheOneConversionIsCalledInOnePlace(t *testing.T) {
	callers := functionsWhoseBodyReads(t, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return false
		}
		ident, ok := call.Fun.(*ast.Ident)
		return ok && ident.Name == "HexCellAt"
	})
	require.Equal(t, []string{"cellAt"}, callers,
		"the authored frame is converted in exactly one place: a second caller of HexCellAt "+
			"is a second place for the basis to be read differently")
}

// TestTheReachedPositionCellIsReadInOneFunction.
//
// A ReachedPosition ending is compiled once, at construction, into the single
// absolute cell an arrival is compared against (compileEndings). Deciding the
// ending means READING that cell — so anything but the one decider that reads
// it is a second implementation of the ending rule, which is exactly what Join
// carried until #1106 deleted it. compileEndings WRITES the field and is not a
// reader; assignment targets are excluded for that reason.
func TestTheReachedPositionCellIsReadInOneFunction(t *testing.T) {
	readers := functionsWhoseBodyReads(t, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "cell"
	})

	require.Equal(t, []string{"firedReachedPosition"}, readers,
		"the compiled ending cell must be read in exactly one place (rpg-toolkit#1108, #1059): "+
			"every arrival — Step, Pump, Join — decides the ending through that one function")
}

// functionsWhoseBodyReads parses this package's non-test source and returns the
// sorted names of every function whose body READS a node the predicate accepts.
// Nodes reached through the left-hand side of an assignment are writes and do
// not count.
//
// Reads the files from disk rather than a go/types view because those files are
// what a future editor changes and what a reviewer reads: a structural test
// whose subject is the source itself explains its own failure.
func functionsWhoseBodyReads(t *testing.T, match func(ast.Node) bool) []string {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)
	require.Contains(t, pkgs, "encounter", "the package must be parseable from its own directory")

	written := map[ast.Node]bool{}
	for _, file := range pkgs["encounter"].Files {
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range assign.Lhs {
				ast.Inspect(lhs, func(inner ast.Node) bool {
					written[inner] = true
					return true
				})
			}
			return true
		})
	}

	names := map[string]bool{}
	for _, file := range pkgs["encounter"].Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if n != nil && !written[n] && match(n) {
					names[fn.Name.Name] = true
				}
				return true
			})
		}
	}

	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
