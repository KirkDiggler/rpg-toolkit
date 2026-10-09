// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// landFile is the one non-test source allowed to land a resolution output
// (design "One landing").
const landFile = "land.go"

// capabilitiesFile is the one non-test source allowed to compose the
// encounter's capabilities or a resolution input (design "One input builder").
const capabilitiesFile = "capabilities.go"

// landingHelpers are the steps only the landing calls: adopting an output's
// world, writing its sheets, and landing or holding its areas.
var landingHelpers = map[string]bool{
	"adopt":         true,
	"saveDirty":     true,
	"landAreas":     true,
	"landToldAreas": true,
	"landHeldAreas": true,
	"holdAreas":     true,
}

// TestOnlyTheLandingLandsAndOnlyTheBuildersCompose holds both laws. Every
// output lands through one function that owns the order, so a verb calling a
// landing step on its own would be a second order nobody reviewed; and every
// capability is composed in one file, so a verb building its own would be a
// second answer to which sheets, brain and die a world on this call reads.
//
// Matched by method name on any receiver, and by composite-literal type name
// for the two composed values, so an alias cannot walk a call past it.
func TestOnlyTheLandingLandsAndOnlyTheBuildersCompose(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var found []string
	calledInLand := map[string]bool{}
	composedInBuilders := map[string]bool{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CallExpr:
				selector, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || !landingHelpers[selector.Sel.Name] {
					return true
				}
				if name == landFile {
					calledInLand[selector.Sel.Name] = true
					return true
				}
				found = append(found, fset.Position(n.Pos()).String()+": calls "+selector.Sel.Name)
			case *ast.CompositeLit:
				selector, ok := n.Type.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				composed := pkg.Name + "." + selector.Sel.Name
				if composed != "resolution.Input" && composed != "encounter.Capabilities" {
					return true
				}
				if name == capabilitiesFile {
					composedInBuilders[composed] = true
					return true
				}
				found = append(found, fset.Position(n.Pos()).String()+": composes "+composed)
			}
			return true
		})
	}
	require.Empty(t, found, "only %s lands an output and only %s composes capabilities", landFile, capabilitiesFile)
	for helper := range landingHelpers {
		require.True(t, calledInLand[helper], "the landing calls %s", helper)
	}
	require.True(t, composedInBuilders["resolution.Input"], "the builders compose the resolution input")
	require.True(t, composedInBuilders["encounter.Capabilities"], "the builders compose the capabilities")
}

// TestAWorldIsLoadedOrSetUpOnlyWhereTheReadersAndTheLandingAre holds the law
// that one world builder serves two readers. An encounter.LoadEncounterInput
// or encounter.SetupInput literal is how a world is stood up, so it appears
// only in read.go (loadGivenWorld and AtlasOf), write.go (adopt, which land.go
// calls) and launch.go (launchWorld); a verb standing up its own would be a second builder for the
// question of which world a call reads.
func TestAWorldIsLoadedOrSetUpOnlyWhereTheReadersAndTheLandingAre(t *testing.T) {
	allowed := map[string]bool{"read.go": true, "write.go": true, "launch.go": true}
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var found []string
	seenIn := map[string]bool{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			selector, ok := lit.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			composed := pkg.Name + "." + selector.Sel.Name
			if composed != "encounter.LoadEncounterInput" && composed != "encounter.SetupInput" {
				return true
			}
			seenIn[name] = true
			if !allowed[name] {
				found = append(found, fset.Position(lit.Pos()).String()+": composes "+composed)
			}
			return true
		})
	}
	require.Empty(t, found, "a world is loaded or set up only in read.go, write.go and launch.go")
	for name := range allowed {
		require.True(t, seenIn[name], "%s stands up a world", name)
	}
}
