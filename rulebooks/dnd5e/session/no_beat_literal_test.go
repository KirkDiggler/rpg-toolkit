// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// encounterModule is the composition whose exported beat kinds this package
// reads.
const encounterModule = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// coincidentalWords are string literals in this package's source that spell a
// beat kind and are not one, each with the reason. Keyed by file, so the same
// word anywhere else is still flagged.
//
// The payload decoder's JSON keys are named here one by one rather than
// excused by shape: a lookup or a membership test against a beat kind is
// exactly an index or a []string element, so shape cannot tell a key from a
// match.
var coincidentalWords = map[string]map[string]string{
	"convert.go":        {"held": "a sighting's status on the wire, held versus current"},
	"declaration_id.go": {"cast": "the cast declaration's selector variant"},
	"events.go": {
		"death_save": "the death save beat's detail object key",
		"warded":     "the warded beat's detail object key",
		"moved":      "a roll component's moved-flag key",
	},
}

// beatKindsOfTheEncounter derives the composition's beat kinds from its own
// source: every exported Beat* constant, and every Outcome* constant of type
// OutcomeKind, with the string each one holds. Derived rather than listed, so
// a kind added upstream is checked here the day this module pins it.
func beatKindsOfTheEncounter(t *testing.T) map[string]string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	require.NoError(t, err)
	dir := strings.TrimSpace(string(out))
	require.NotEmpty(t, dir)

	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)
	kinds := map[string]string{}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				for i, ident := range value.Names {
					beat := strings.HasPrefix(ident.Name, "Beat") && value.Type == nil
					outcome := strings.HasPrefix(ident.Name, "Outcome") && typeNamed(value.Type, "OutcomeKind")
					if !ident.IsExported() || (!beat && !outcome) || i >= len(value.Values) {
						continue
					}
					if kind, ok := stringLiteral(value.Values[i]); ok {
						kinds[kind] = ident.Name
					}
				}
			}
		}
	}
	require.NotEmpty(t, kinds, "the encounter exports beat kinds")
	return kinds
}

func typeNamed(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

// TestNoBeatKindIsMatchedByALiteral holds rpg-project#539's line: the beat
// kinds a client reads are constants the composition exports, and this
// package's projection maps constants, never string literals. A literal keeps
// compiling after the composition renames a beat and silently answers
// "unknown"; a constant fails to compile.
//
// EVERY string literal in non-test source is checked against the derived
// kinds, wherever it sits. What may spell a beat kind:
//   - the value of a constant this package declares with its own named type
//     (EventKind, Verb, DissolveKind): this package's wire vocabulary, which
//     the projection maps TO, and which happens to share many words;
//   - a struct tag;
//   - a word in coincidentalWords, by file, with its reason — the payload
//     decoder's JSON keys among them.
func TestNoBeatKindIsMatchedByALiteral(t *testing.T) {
	kinds := beatKindsOfTheEncounter(t)

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var found []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)

		allowed := map[*ast.BasicLit]bool{}
		allow := func(expr ast.Expr) {
			if lit, ok := expr.(*ast.BasicLit); ok {
				allowed[lit] = true
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ValueSpec:
				if v.Type != nil {
					for _, value := range v.Values {
						allow(value)
					}
				}
			case *ast.Field:
				if v.Tag != nil {
					allow(v.Tag)
				}
			}
			return true
		})

		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || allowed[lit] {
				return true
			}
			value, ok := stringLiteral(lit)
			if !ok {
				return true
			}
			constant, isKind := kinds[value]
			if !isKind {
				return true
			}
			if _, coincidental := coincidentalWords[name][value]; coincidental {
				return true
			}
			found = append(found, fset.Position(lit.Pos()).String()+" spells "+strconv.Quote(value)+
				"; use encounter."+constant)
			return true
		})
	}
	require.Empty(t, found, "a beat kind written as a string literal")
}
