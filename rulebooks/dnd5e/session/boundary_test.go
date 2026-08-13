// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// BoundaryTestSuite enforces S2: no inner type crosses this package's exported
// surface.
//
// This is the most important test in the module, and it is not a behaviour
// test. The whole strategy — put the existing implementation behind this SDK,
// migrate the host once, then replace the insides wave by wave while the host
// only ever bumps a version — rests on inner modules being invisible from
// outside. The moment an encounter, combat, clock, intel, record or interrupt
// type appears in an exported signature, replacing that module becomes a
// breaking change for the host, and the promise is silently void. Nothing else
// in CI would notice.
//
// So the guarantee is mechanical rather than aspirational: this test reads the
// package's own source and fails on the violation directly, instead of trusting
// that reviewers will spot a leaked type in a diff two years from now.
type BoundaryTestSuite struct {
	suite.Suite
}

// forbiddenPrefix is the import prefix whose types must not appear in exported
// signatures. Standard library and third-party types are unconstrained: they
// are not modules we intend to replace underneath the host.
const forbiddenPrefix = "github.com/KirkDiggler/rpg-toolkit/"

// The types permitted in exported signatures, written out rather than derived
// from a pattern.
//
// A rule like "any name ending in Data" would be less typing and would quietly
// admit types nobody weighed. Listing them means every future exception is a
// visible line in a diff with a reason attached, which is the only version of
// an exception list that stays honest.
//
// They are split into TWO CATEGORIES because they make two different promises,
// and a flat list hid that. The entities wave forced the distinction: reading
// character.Data as one more grudging exception to replaceability got the
// intent exactly backwards.
//
// The split is not decoration. It tells a future author which question to ask
// about a new entry, and the two questions have different answers.

// contractTypes are shared vocabulary: the host constructs these, names them in
// its own code, and reasons about them.
//
// Their promise is NOT replaceability. It is the opposite — a change to one is
// a breaking change we ANNOUNCE. These are domain nouns both sides agree on,
// and refactoring one without telling the host would be the bug, not the
// safeguard.
//
// The question to ask before adding one: is this a thing the host legitimately
// knows about, such that we would want it to hear about a change?
var contractTypes = map[string]string{
	// A coordinate is a coordinate. spatial.Position is a stable value type
	// with no behaviour to replace, and inventing a twin for it would force
	// hosts to convert on every call for no benefit.
	"spatial.Position": "stable value type; shared vocabulary for placement",

	// A character is a thing, not an implementation detail we would refactor
	// without telling the host. Its surface bottoms out in strings and ints —
	// skills.Skill and classes.Class are literal aliases to string, and
	// features and conditions are already []json.RawMessage — so naming it
	// commits us to a shape that is already wire-like.
	//
	// It appears on CharacterRepository ONLY. Verbs take member IDs, and
	// Member.Kind routes to the repository; character.Data is never a verb
	// input. What crosses is the shape the host already persists, not the
	// object we build from it.
	"character.Data": "contract type: the sheet the host owns, stores, and constructs",
}

// persistenceShapes are bytes the host round-trips and never builds.
//
// Their promise IS replaceability: the module underneath can be reshaped or
// swapped and the host never notices, because it stores these opaquely rather
// than reading them.
//
// The question to ask before adding one: does the host store this without ever
// constructing or inspecting it? If the host would have to build one field by
// field, it is a contract type and belongs above, under a stricter promise.
var persistenceShapes = map[string]string{
	// The documented S3 exception: persistence shapes cross the boundary
	// because they are exactly the bytes the host already stores. Data types
	// are the slowest-moving surface in the toolkit and carry their own
	// compatibility discipline; domain types do not.
	//
	// It appears on EncounterRepository and in StartSession, which hands in
	// authored content. That is not a widening: it is the same bytes the host
	// holds either way, so no host is exposed to anything the repository did
	// not already expose it to. A *domain* type in a verb input would be a different
	// matter, and is what this list exists to keep out.
	"encounter.EncounterData": "persistence shape the host already holds (S3)",

	// The same S3 exception, for the same reason: SessionData is a persistence
	// shape, and the ledger of open windows is part of what a session *is*.
	// The host stores these bytes opaquely and never constructs one — it has no
	// reason to name interrupt.LedgerData in its own code, and every reason to
	// round-trip it untouched.
	//
	// Note what this does NOT admit: interrupt.Window, interrupt.Ledger, or
	// interrupt.Option appearing in a verb signature. Suspension reaches the
	// host through SDK-owned Pending and Option types precisely so the custody
	// module underneath can be replaced. Only the stored shape crosses.
	"interrupt.LedgerData": "persistence shape the host stores opaquely (S3)",

	// A spawned NPC's sheet, inside SessionData.
	//
	// This lands in the persistence category rather than beside character.Data,
	// and the two are worth contrasting because they look alike and are not.
	// The host CONSTRUCTS a character.Data — it owns character storage, and a
	// change to that shape is a change to code it writes, so we announce it.
	// The host never constructs a monster.Data: Spawn builds it from a ref and
	// hands it back already made, and the host's only job is to round-trip the
	// session blob it sits in.
	//
	// So the promise here is replaceability. If monster.Data is reshaped, or
	// the monster package is replaced outright, a host that stored the bytes
	// notices nothing. That is worth having, because monster is the least
	// settled of the packages this module names.
	//
	// The test to apply, from the header above: would the host have to build
	// one field by field? For a character, yes. For a monster, no — it names a
	// ref instead, which is the whole point of the split between Join and
	// Spawn.
	//
	// What is NOT admitted: monster.Monster, the runtime object. Spawn returns
	// an SDK-owned MonsterState, and the loaded monster never crosses.
	"monster.Data": "persistence shape: an NPC sheet the host stores but never builds",
}

// allowed is the union the detector checks against.
var allowed = func() map[string]string {
	all := map[string]string{}
	for k, v := range contractTypes {
		all[k] = v
	}
	for k, v := range persistenceShapes {
		all[k] = v
	}
	return all
}()

// TestNoInnerTypeCrossesTheBoundary parses this package's non-test sources and
// fails on any toolkit type reachable from an exported declaration.
//
// It walks exported functions and methods (parameters and results), exported
// struct fields, exported interface methods, and exported type definitions —
// every route by which a host could come to name an inner type.
func (s *BoundaryTestSuite) TestNoInnerTypeCrossesTheBoundary() {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	s.Require().NoError(err)
	s.Require().Contains(pkgs, "session", "expected to parse package session")

	var violations []string

	for _, file := range pkgs["session"].Files {
		imports := importsOf(file)

		record := func(node ast.Node, where string) {
			for _, use := range toolkitTypesIn(node, imports) {
				if _, ok := allowed[use.qualified]; ok {
					continue
				}
				violations = append(violations, fset.Position(node.Pos()).String()+
					": "+where+" exposes "+use.qualified)
			}
		}

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				// An exported method on an unexported type is unreachable from
				// outside and does not widen the surface.
				if d.Recv != nil && !receiverIsExported(d.Recv) {
					continue
				}
				record(d.Type, "func "+d.Name.Name)

			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || !ts.Name.IsExported() {
						continue
					}
					switch t := ts.Type.(type) {
					case *ast.StructType:
						for _, field := range t.Fields.List {
							if !fieldIsExported(field) {
								continue
							}
							record(field.Type, "field on "+ts.Name.Name)
						}
					case *ast.InterfaceType:
						for _, method := range t.Methods.List {
							record(method.Type, "method on "+ts.Name.Name)
						}
					default:
						record(ts.Type, "type "+ts.Name.Name)
					}
				}
			}
		}
	}

	s.Empty(violations,
		"an inner type reached the exported surface — replacing that module is now "+
			"a breaking change for the host, which is exactly what S2 exists to prevent:\n%s",
		strings.Join(violations, "\n"))
}

// TestBoundaryTestCanActuallyFail is the meta-pin.
//
// A boundary test that cannot fail is worse than no boundary test: it reports
// green forever and everyone stops looking. This exercises the same detection
// machinery against a signature that deliberately names a forbidden type, and
// requires it to be caught.
func (s *BoundaryTestSuite) TestBoundaryTestCanActuallyFail() {
	const leaky = `package session

import "github.com/KirkDiggler/rpg-toolkit/play/record"

// Story leaks an inner type through an exported result.
func (m *Manager) Story() []record.Entry { return nil }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "leaky.go", leaky, 0)
	s.Require().NoError(err)

	imports := importsOf(file)
	fn := file.Decls[1].(*ast.FuncDecl)
	uses := toolkitTypesIn(fn.Type, imports)

	s.Require().Len(uses, 1, "the detector must see the leaked type")
	s.Equal("record.Entry", uses[0].qualified)
	s.NotContains(allowed, "record.Entry", "and must not consider it permitted")
}

// TestAllowListIsJustified requires every exception to carry a reason, so the
// list cannot grow by accretion.
func (s *BoundaryTestSuite) TestAllowListIsJustified() {
	for qualified, reason := range allowed {
		s.NotEmpty(reason, "allowed type %s must record why it is permitted", qualified)
	}
}

// TestTheTwoCategoriesAreDisjoint pins the split itself.
//
// A type listed in both would mean nobody decided which promise it makes, and
// the whole value of the split is that the decision is forced. Merging silently
// into `allowed` would hide that, since the union does not care.
func (s *BoundaryTestSuite) TestTheTwoCategoriesAreDisjoint() {
	for qualified := range contractTypes {
		s.NotContains(persistenceShapes, qualified,
			"%s is in both categories; it must promise one thing or the other", qualified)
	}
	s.Len(allowed, len(contractTypes)+len(persistenceShapes),
		"the union must not silently collapse an entry listed twice")
}

// TestTheRuntimeCharacterIsNotAdmitted is the discriminating half of admitting
// character.Data.
//
// Letting the stored shape cross is a decision about persistence. Letting the
// loaded object cross would be a different decision entirely — it carries
// behaviour, a live bus, and subscriptions, none of which survive a response.
// A rejection table proves the detector refuses things; this names the one
// refusal that admitting its neighbour makes tempting.
func (s *BoundaryTestSuite) TestTheRuntimeCharacterIsNotAdmitted() {
	s.NotContains(allowed, "character.Character",
		"the loaded character must never cross the boundary")
	s.Contains(allowed, "character.Data",
		"while the stored shape it is built from does")
}

type typeUse struct {
	qualified string // e.g. "record.Entry"
}

// importsOf maps each local package identifier in a file to its import path,
// honouring named imports.
func importsOf(file *ast.File) map[string]string {
	out := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		out[name] = path
	}
	return out
}

// toolkitTypesIn walks a type expression and returns every qualified reference
// to a toolkit package, however deeply nested — a []*encounter.Thing inside a
// map value is just as much a leak as a bare parameter.
func toolkitTypesIn(node ast.Node, imports map[string]string) []typeUse {
	var found []typeUse
	ast.Inspect(node, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		path, ok := imports[ident.Name]
		if !ok || !strings.HasPrefix(path, forbiddenPrefix) {
			return true
		}
		found = append(found, typeUse{qualified: ident.Name + "." + sel.Sel.Name})
		return true
	})
	return found
}

func receiverIsExported(recv *ast.FieldList) bool {
	if recv == nil || len(recv.List) == 0 {
		return false
	}
	var name string
	switch t := recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			name = ident.Name
		}
	case *ast.Ident:
		name = t.Name
	}
	return name != "" && ast.IsExported(name)
}

func fieldIsExported(field *ast.Field) bool {
	if len(field.Names) == 0 {
		return true // embedded field: its own name governs
	}
	for _, name := range field.Names {
		if name.IsExported() {
			return true
		}
	}
	return false
}

func TestBoundarySuite(t *testing.T) {
	require.NotEmpty(t, forbiddenPrefix)
	suite.Run(t, new(BoundaryTestSuite))
}

// structFields reports the exported field names of a struct value, so a test
// can assert on a type's shape rather than only on the values it happens to
// carry.
func structFields(v any) []string {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		names = append(names, t.Field(i).Name)
	}
	return names
}
