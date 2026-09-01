// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/stretchr/testify/require"
)

// TestEntriesReturnAnswersNotSheets is the D11 pin: nothing an entry hands back
// carries a ToData().
//
// # Why ToData is the thing being matched
//
// It is the marker of a SHEET. A live character or monster can serialize itself,
// because it is the thing the repository stores; an answer cannot, because it is
// a number somebody derived. So a type with a ToData() crossing this boundary
// means the seam stopped holding records and started holding sheets — and a
// caller given one has a serialization affordance outside the keeper discipline,
// which is exactly the probe that produced this rule ("if we return character
// doesn't that have a ToData() on it?").
//
// # Outputs only, and inputs deliberately not
//
// Records go IN. That is the whole shape: the seam fetches records and only
// records, hands them down, and takes answers back. So the direction is what
// matters, and the direction is what this walks.
//
// # It fails closed on what it cannot see
//
// A static walk over types cannot see through an interface — the dynamic type
// is whatever the code put there at runtime, and a `Payload any` holding a live
// sheet walks clean past a naive check. Both that and a map KEYED by a sheet
// were reproduced as real escapes from the first version of this test.
//
// So an interface-typed field is REFUSED unless it is named in
// [allowedInterfaceFields] with a reason, and maps are walked on both key and
// element. The allow-list is the honest shape: it turns "this walker is blind
// here" from a silent gap into a line somebody had to write down.
func TestEntriesReturnAnswersNotSheets(t *testing.T) {
	outputs := []struct {
		entry string
		typ   reflect.Type
	}{
		{"ProjectCharacter", reflect.TypeOf(ProjectCharacterOutput{})},
		{"Standing", reflect.TypeOf(StandingOutput{})},
		{"Preflight", reflect.TypeOf(PreflightOutput{})},
		{"Resolve", reflect.TypeOf(Output{})},
	}

	for _, out := range outputs {
		offenders := serializableTypesIn(out.typ, map[reflect.Type]bool{}, nil)

		require.Empty(t, offenders,
			"%sOutput reaches something this walk cannot clear. A type with a ToData() is "+
				"what a SHEET has and an ANSWER does not — a caller handed one can serialize "+
				"a live object the keeper is supposed to own. An unlisted interface field is "+
				"refused rather than trusted, because its dynamic type is invisible here: "+
				"send numbers, or name the field in allowedInterfaceFields with the reason "+
				"it is safe. Found: %v",
			out.entry, offenders)
	}
}

// allowedInterfaceFields are the interface-typed fields this walk may not see
// through, and the reason each is safe anyway. Keyed by the field path the walk
// reports.
//
// Every entry is a promise a human made and the compiler cannot keep, so each
// one says what it is resting on.
var allowedInterfaceFields = map[string]string{
	// Preflight's refusal causes are prose — they exist to be read by a person
	// deciding which offer row is dead and why. A sheet smuggled behind an
	// error would have to survive the entry's own construction, and Preflight
	// builds these from attachAll's returned error and nothing else.
	"Unreadable.Reason": "refusal causes are prose, built only from attachAll's error",

	// A monster's saving-throw DC is a SEALED interface in the saves package:
	// isDCSource() is unexported THERE, so neither this package nor any caller
	// can put anything behind it — the set of implementors is fixed by a
	// package that holds no sheets. Reached through DirtyMonsters, which is a
	// RECORD going back to be persisted: records crossing outward is the shape
	// R3 asks for, not what D11 forbids.
	"DirtyMonsters.Actions.Attack.OnHit.Save.DC":   "sealed in saves; reached through a record, not an answer",
	"DirtyCharacters.Actions.Attack.OnHit.Save.DC": "sealed in saves; reached through a record, not an answer",

	// Resolve's outcome is a SEALED interface: isOutcome() is unexported, so
	// only this package can implement it. That does not make it invisible, it
	// makes it enumerable — every implementor is walked by
	// TestEveryOutcomeIsAnswerShaped below, and a new one that forgets to join
	// that table fails it.
	"Outcome": "sealed to this package; every implementor is walked separately",
}

// TestEveryOutcomeIsAnswerShaped walks the concrete types behind Resolve's
// sealed Outcome, which the reflection walk above is allowed to skip.
//
// The allow-list entry for Outcome is only honest if something actually checks
// the types it stands for. This is that something.
func TestEveryOutcomeIsAnswerShaped(t *testing.T) {
	for _, outcome := range outcomeTypes {
		offenders := serializableTypesIn(outcome, map[reflect.Type]bool{}, nil)

		require.Empty(t, offenders,
			"%s reaches a type with a ToData(). An outcome is what a machine produced, "+
				"which is data; a sheet reached through one crosses the seam just as surely "+
				"as a field on the output would: %v",
			outcome.Name(), offenders)
	}
}

// TestTheOutcomeTableIsComplete holds the table above to the source, so the
// allow-list cannot quietly fall behind a new outcome.
//
// The same argument truthEntries makes: the count IS the assertion, so the
// source is the assertion. A new outcome type is a deliberate join, not a
// silent one.
func TestTheOutcomeTableIsComplete(t *testing.T) {
	declared := outcomeImplementorsInSource(t)

	named := make([]string, 0, len(outcomeTypes))
	for _, o := range outcomeTypes {
		named = append(named, o.Name())
	}
	sort.Strings(named)

	require.Equal(t, declared, named,
		"every type declaring isOutcome() must be walked by TestEveryOutcomeIsAnswerShaped. "+
			"A new outcome joins the table or the allow-list entry for Outcome stops being true")
}

// outcomeTypes are the concrete types behind [Outcome].
var outcomeTypes = []reflect.Type{
	reflect.TypeOf(MovementOutcome{}),
	reflect.TypeOf(SaveOutcome{}),
	reflect.TypeOf(ActivationOutcome{}),
	reflect.TypeOf(BoundaryOutcome{}),
	reflect.TypeOf(ContestOutcome{}),
	reflect.TypeOf(StrikeOutcome{}),
}

// outcomeImplementorsInSource returns every type in this package declaring an
// isOutcome method, sorted.
func outcomeImplementorsInSource(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	fset := token.NewFileSet()
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, parseErr)

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "isOutcome" || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
				names = append(names, ident.Name)
			}
		}
	}
	sort.Strings(names)

	return names
}

// The two escapes an earlier version of this walk let through, kept as types
// so the closure is demonstrated rather than asserted.
//
// Both were found by probing rather than by reading, and both walked CLEAN
// past the first version: it returned nil for reflect.Interface, and it
// recursed a map's element while ignoring its key. A fix nobody can see fail
// is a fix nobody can trust, so the shapes stay here and the test below runs
// them.
type escapeBehindAny struct{ Payload any }

type escapeInAMapKey struct{ BySheet map[*character.Character]int }

// TestTheKnownEscapesAreClosed runs the walk over the two shapes that used to
// defeat it.
//
// Named for what they are. An `any` field is the general case — the walk cannot
// see a dynamic type, so it must refuse rather than wave through — and a map
// key is the specific one, where an earlier version looked at half the type.
func TestTheKnownEscapesAreClosed(t *testing.T) {
	behindAny := serializableTypesIn(reflect.TypeOf(escapeBehindAny{}), map[reflect.Type]bool{}, nil)
	require.NotEmpty(t, behindAny,
		"an interface field can hold a live sheet and this walk cannot see it — refusing is the "+
			"only safe answer, and the first version of this test waved it through")
	require.Contains(t, behindAny[0], "Payload")

	inAKey := serializableTypesIn(reflect.TypeOf(escapeInAMapKey{}), map[reflect.Type]bool{}, nil)
	require.NotEmpty(t, inAKey,
		"a map KEYED by a sheet smuggles one as surely as a map holding one; the first version "+
			"walked the element and ignored the key")
	require.Contains(t, inAKey[0], "character.Character")
}

// serializableTypesIn returns the paths within t that this walk cannot clear:
// a type declaring ToData, or an interface it is not allowed to skip.
//
// The cycle guard is the types on the CURRENT PATH, not every type seen
// anywhere. A global set would make the report order-dependent — a type
// reachable by two routes would be attributed to whichever field the walk
// happened to reach first, so the allow-list would be keyed on a path that
// could move when an unrelated field was reordered. Guarding the path only
// costs a repeat visit and buys a deterministic report.
func serializableTypesIn(t reflect.Type, onPath map[reflect.Type]bool, path []string) []string {
	if t == nil || onPath[t] {
		return nil
	}
	onPath[t] = true
	defer delete(onPath, t)

	if hasToData(t) {
		return []string{pathOf(path) + " (" + t.String() + ")"}
	}

	switch t.Kind() {
	case reflect.Interface:
		if _, ok := allowedInterfaceFields[pathOf(path)]; ok {
			return nil
		}

		return []string{pathOf(path) + " (interface " + t.String() + ": dynamic type not visible to this walk)"}
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return serializableTypesIn(t.Elem(), onPath, path)
	case reflect.Map:
		// BOTH halves. A map keyed by a sheet smuggles one just as well as a
		// map holding one, and the first version of this test walked only the
		// element — reproduced as a real escape.
		found := serializableTypesIn(t.Key(), onPath, append(path, "<key>"))

		return append(found, serializableTypesIn(t.Elem(), onPath, path)...)
	case reflect.Struct:
		var found []string
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			found = append(found, serializableTypesIn(field.Type, onPath, append(path, field.Name))...)
		}

		return found
	default:
		return nil
	}
}

// hasToData reports whether t, or a pointer to t, declares ToData.
func hasToData(t reflect.Type) bool {
	if _, ok := t.MethodByName("ToData"); ok {
		return true
	}
	if t.Kind() != reflect.Pointer {
		if _, ok := reflect.PointerTo(t).MethodByName("ToData"); ok {
			return true
		}
	}

	return false
}

func pathOf(path []string) string {
	if len(path) == 0 {
		return "<the output itself>"
	}

	return strings.Join(path, ".")
}
