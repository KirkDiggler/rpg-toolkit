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

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
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

	// A world NPC's content is character.Data's shape, not monster.Data's —
	// the caller builds it field-by-field (npcs.NewMerchant, since no
	// toolkit catalog resolves an NPC from a bare ref the way monsters.ByRef
	// does for monsters) and hands the whole value to PlaceNPCInput.NPC. It
	// DOES appear directly on a verb input, unlike character.Data — that
	// asymmetry is real: a world NPC has no durable host-owned repository to
	// fetch it back from later the way a character does, so the content has
	// to travel with the placement call itself (rpg-toolkit#1404).
	"npc.Data": "contract type: the caller constructs this content field-by-field, same promise as character.Data",

	// Reachable from WorldNPCDescriptor.Capabilities/CombatPolicy
	// (interact.go) — deliberately deferred out of the npc.Data PR (#1414)
	// because nothing used them yet; WorldNPCDescriptor is what makes them
	// real. Same reasoning as npc.Data above: the caller constructs this
	// content, it is never resolved from a ref the way a monster's is.
	"npc.Capability":   "contract type: reachable from WorldNPCDescriptor, same promise as npc.Data",
	"npc.CombatPolicy": "contract type: reachable from WorldNPCDescriptor, same promise as npc.Data",

	// Reachable from WorldNPCDescriptor.Inventory (interact.go, #1447).
	// npcs.StockEntryView is display data npcs already builds and hands
	// back read-only (npcs.VendorInventoryFromNPCData, VendorInventory.View)
	// — this seam never constructs one field-by-field, only reads it — but
	// it is still real domain vocabulary both sides agree on (a vendor's
	// stock row), not an implementation detail we'd refactor without
	// telling the host, so it sits here rather than under persistenceShapes.
	// Its own fields already bottom out in strings and ints (shared.EquipmentType
	// and npcs.StockMode are both string aliases).
	"npcs.StockEntryView": "contract type: reachable from WorldNPCDescriptor, read-only display data npcs resolves and hands back",

	// Reachable from TradeItem.Type (trade.go, rpg-project#369) — the same
	// domain noun npcs.StockEntryView.Type already carries one hop further
	// in, now named directly on a verb input because a caller CONSTRUCTS a
	// TradeItem field-by-field rather than only reading one back. A plain
	// string alias with no behaviour to replace, the same promise
	// spatial.Position makes above.
	"shared.EquipmentType": "contract type: reachable from TradeItem, a caller-constructed field naming what's being traded",

	// Reachable from TradeOffer.Currency (trade.go, rpg-toolkit#1534, Wave
	// 4). A caller CONSTRUCTS this value (it is the payment offered on
	// Give, checked against a server-computed price) rather than only
	// reading one back — same promise as shared.EquipmentType above. A
	// single-field struct wrapping an int, no behaviour to replace.
	"currency.Money": "contract type: reachable from TradeOffer.Currency, a caller-constructed payment amount",
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

	// interrupt.LedgerData was admitted here — SessionData carried a ledger of
	// open windows, and the host round-tripped those bytes without reading
	// them. Nothing in this package opens a window any more (rpg-toolkit#964
	// slice 2), so the field went and the exception went with it.
	//
	// The list SHRINKING is the note worth leaving. An allow-list only ever
	// grows if nobody checks whether an entry is still earning its place, and
	// an exception outliving the thing it excepted is how one turns into
	// decoration.

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

// TestExecutionInputsCarryOnlyTheSelectorEcho pins the three changed host
// boundaries exactly. DeclarationID is the sole new authority-bearing field;
// no compiled definition, spend profile, candidate, or inner type crossed S2.
func TestExecutionInputsCarryOnlyTheSelectorEcho(t *testing.T) {
	require.Equal(t, []string{"Session", "Attacker", "Target", "DeclarationID"},
		structFields(session.AttackInput{}))
	require.Equal(t, []string{"Session", "Member", "DeclarationID", "Path"},
		structFields(session.MoveInput{}))
	require.Equal(t, []string{"Session", "Member", "DeclarationID"},
		structFields(session.EndTurnInput{}))
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

// unexportedInterfaceMethods reports the UNEXPORTED methods an interface
// declares — the ones that seal it, since a type outside the declaring package
// cannot implement them.
//
// Reflection rather than the AST parsing above, and deliberately: for an
// INTERFACE type, reflect's NumMethod includes unexported methods and reports
// each one's package. That is documented behaviour rather than an accident of
// same-module compilation — verified from a separate module entirely, where
// DissolveCause reports NumMethod=2 with isDissolveCause carrying
// PkgPath=".../session" and IsExported()=false. (It is NON-interface types
// whose unexported methods reflect omits, which is the rule this is often
// confused with.)
//
// Pass a typed nil pointer: unexportedInterfaceMethods((*Foo)(nil)).
func unexportedInterfaceMethods(ptr any) []string {
	t := reflect.TypeOf(ptr)
	if t == nil || t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Interface {
		return nil
	}
	iface := t.Elem()
	var sealed []string
	for i := 0; i < iface.NumMethod(); i++ {
		if m := iface.Method(i); !m.IsExported() {
			sealed = append(sealed, m.Name)
		}
	}
	return sealed
}
